package init

import (
	"fmt"
	"io"
	"os"

	"github.com/gramLabs/redsky/pkg/api"
	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchutil "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type InitOptions struct {
	Bootstrap bool
	DryRun    bool

	installNamespace string
	installName      string
	command          string

	ClientSet *kubernetes.Clientset

	cmdutil.IOStreams
}

func NewInitOptions(ioStreams cmdutil.IOStreams) *InitOptions {
	return &InitOptions{
		installNamespace: "redsky-system",
		installName:      "redsky-bootstrap",
		command:          "install",
		IOStreams:        ioStreams,
	}
}

func NewInitCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewInitOptions(ioStreams)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Red Sky in a cluster",
		Long:  "The initialize command will install (or optionally generate) the required Red Sky manifests.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.Bootstrap, "bootstrap", false, "stop after creating the bootstrap configuration")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "generate the manifests instead of applying them")

	return cmd
}

func (o *InitOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {

	// If this is really a reset then change the setup command to uninstall
	if cmd.Name() == "reset" {
		o.command = "uninstall"
	}

	var err error
	o.ClientSet, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	return nil
}

func (o *InitOptions) Run() error {
	// TODO Should the util.Factory expose the configuration for bootstraping? Or should be build this some other way?
	clientConfig, err := api.DefaultConfig()
	if err != nil {
		return err
	}

	bootstrapConfig, err := NewBootstrapConfig(o.installNamespace, o.installName, o.command, clientConfig)
	if err != nil {
		return err
	}

	// A bootstrap dry run just means serialize the bootstrap config
	if o.Bootstrap && o.DryRun {
		return bootstrapConfig.Marshal(o.Out)
	}

	// Create, but do not execute the job
	if o.Bootstrap {
		bootstrapConfig.Job.Spec.Parallelism = new(int32)
	}

	// Request generation of manifests only
	if o.DryRun {
		bootstrapConfig.Job.Spec.Template.Spec.Containers[0].Args = append(bootstrapConfig.Job.Spec.Template.Spec.Containers[0].Args, "--dry-run")
	}

	// Get all the Kubernetes clients we need
	namespacesClient := o.ClientSet.CoreV1().Namespaces()
	clusterRolesClient := o.ClientSet.RbacV1().ClusterRoles()
	clusterRoleBindingsClient := o.ClientSet.RbacV1().ClusterRoleBindings()
	rolesClient := o.ClientSet.RbacV1().Roles(o.installNamespace)
	roleBindingsClient := o.ClientSet.RbacV1().RoleBindings(o.installNamespace)
	secretsClient := o.ClientSet.CoreV1().Secrets(o.installNamespace)
	jobsClient := o.ClientSet.BatchV1().Jobs(o.installNamespace)
	podsClient := o.ClientSet.CoreV1().Pods(o.installNamespace)

	// TODO It would be nice if we could set ownership to the job to cascade the clean up for us

	// The namespace isn't a bootstrap object, it needs to persist and we can't fail if already exists
	if _, err = namespacesClient.Create(&bootstrapConfig.Namespace); err != nil && os.IsExist(err) {
		return err
	}

	if _, err = clusterRolesClient.Create(&bootstrapConfig.ClusterRole); err != nil {
		return err
	}
	defer func() {
		_ = clusterRolesClient.Delete(o.installName, nil)
	}()

	if _, err = clusterRoleBindingsClient.Create(&bootstrapConfig.ClusterRoleBinding); err != nil {
		return err
	}
	defer func() {
		_ = clusterRoleBindingsClient.Delete(o.installName, nil)
	}()

	if _, err = rolesClient.Create(&bootstrapConfig.Role); err != nil {
		return err
	}
	defer func() {
		_ = rolesClient.Delete(o.installName, nil)
	}()

	if _, err = roleBindingsClient.Create(&bootstrapConfig.RoleBinding); err != nil {
		return err
	}
	defer func() {
		_ = roleBindingsClient.Delete(o.installName, nil)
	}()

	if _, err = secretsClient.Create(&bootstrapConfig.Secret); err != nil {
		return err
	}
	defer func() {
		_ = secretsClient.Delete(o.installName, nil)
	}()

	job, err := jobsClient.Create(&bootstrapConfig.Job)
	if err != nil {
		return err
	}
	defer func() {
		if job.Spec.TTLSecondsAfterFinished == nil {
			pp := metav1.DeletePropagationForeground
			_ = jobsClient.Delete(o.installName, &metav1.DeleteOptions{
				PropagationPolicy: &pp,
			})
		}
	}()

	// Wait for the job to finish (unless we are bootstraping the install or uninstalling)
	if !o.Bootstrap {
		watch, err := podsClient.Watch(metav1.ListOptions{LabelSelector: "job-name = " + o.installName})
		if err != nil {
			return err
		}
		defer watch.Stop()
		for event := range watch.ResultChan() {
			if p, ok := event.Object.(*corev1.Pod); ok {
				if p.Status.Phase == corev1.PodSucceeded {
					// TODO Go routine to pump pod logs to stdout? Should we do that no matter what?
					if o.command != "uninstall" {
						if err := dumpLog(podsClient, p.Name, o.Out); err != nil {
							return err
						}
					}
					watch.Stop()
				} else if p.Status.Phase == corev1.PodPending || p.Status.Phase == corev1.PodFailed {
					for _, c := range p.Status.ContainerStatuses {
						if c.State.Waiting != nil && c.State.Waiting.Reason == "ImagePullBackOff" {
							return fmt.Errorf("unable to pull image '%s'", c.Image)
						} else if c.State.Terminated != nil && c.State.Terminated.Reason == "Error" {
							// TODO For now just copy logs over?
							if o.command != "uninstall" {
								if err := dumpLog(podsClient, p.Name, o.ErrOut); err != nil {
									return err
								}
							}
							return fmt.Errorf("encountered an error")
						}
					}
				} else if event.Type == watchutil.Deleted {
					return fmt.Errorf("initialization pod was deleted before it could finish")
				}
			}
		}
	}

	return nil
}

func dumpLog(podsClient clientcorev1.PodInterface, name string, w io.Writer) error {
	r, err := podsClient.GetLogs(name, &corev1.PodLogOptions{}).Stream()
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(w, r)
	if err != nil {
		return err
	}
	return nil
}

package cordeliactl

import (
	"fmt"
	"io"
	"os"

	"github.com/gramLabs/cordelia/pkg/api"
	cmdutil "github.com/gramLabs/cordelia/pkg/cordeliactl/util"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchutil "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
)

type initOptions struct {
	kubeconfig       string
	installNamespace string
	installName      string
	bootstrap        bool
	dryRun           bool
	uninstall        bool
}

func newInitOptions() *initOptions {
	return &initOptions{
		installNamespace: "cordelia-system",
		installName:      "cordelia-bootstrap",
	}
}

func newInitCommand() *cobra.Command {
	o := newInitOptions()

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Cordelia in a cluster",
		Long:  "The initialize command will install (or optionally generate) the required Cordelia manifests.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.run())
		},
	}

	// TODO Should this be a persistent flag on the root command?
	cmdutil.KubeConfig(cmd, &o.kubeconfig)

	cmd.Flags().BoolVar(&o.bootstrap, "bootstrap", false, "stop after creating the bootstrap configuration")
	cmd.Flags().BoolVar(&o.dryRun, "dry-run", false, "generate the manifests instead of applying them")

	// TODO How do we get the server address?
	// TODO How do we collect client_id/secret? Only from a file?

	return cmd
}

func (o *initOptions) run() error {
	// TODO How do we generate the client configuration? Registration?
	clientConfig, err := api.DefaultConfig()
	if err != nil {
		return err
	}

	bootstrapConfig, err := newBootstrapConfig(o.installNamespace, o.installName, clientConfig)
	if err != nil {
		return err
	}

	// If this is a request to uninstall, change the arguments
	if o.uninstall {
		bootstrapConfig.Job.Spec.Template.Spec.Containers[0].Args = []string{"uninstall"}
	}

	// A bootstrap dry run just means serialize the bootstrap config
	if o.bootstrap && o.dryRun {
		bootstrapConfig.Marshal(os.Stdout)
		return nil
	}

	// Create, but do not execute the job
	if o.bootstrap {
		bootstrapConfig.Job.Spec.Parallelism = new(int32)
	}

	// Request generation of manifests only
	if o.dryRun {
		bootstrapConfig.Job.Spec.Template.Spec.Containers[0].Args = append(bootstrapConfig.Job.Spec.Template.Spec.Containers[0].Args, "--dry-run")
	}

	// Get all the Kubernetes clients we need
	config, err := clientcmd.BuildConfigFromFlags("", o.kubeconfig)
	if err != nil {
		return err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}
	namespacesClient := clientset.CoreV1().Namespaces()
	clusterRolesClient := clientset.RbacV1().ClusterRoles()
	clusterRoleBindingsClient := clientset.RbacV1().ClusterRoleBindings()
	rolesClient := clientset.RbacV1().Roles(o.installNamespace)
	roleBindingsClient := clientset.RbacV1().RoleBindings(o.installNamespace)
	secretsClient := clientset.CoreV1().Secrets(o.installNamespace)
	jobsClient := clientset.BatchV1().Jobs(o.installNamespace)
	podsClient := clientset.CoreV1().Pods(o.installNamespace)

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

	// Wait for the job to finish (unless we are just bootstraping the install)
	if !o.bootstrap {
		watch, err := podsClient.Watch(metav1.ListOptions{LabelSelector: "job-name = " + o.installName})
		if err != nil {
			return err
		}
		defer watch.Stop()
		for event := range watch.ResultChan() {
			if p, ok := event.Object.(*corev1.Pod); ok {
				if p.Status.Phase == corev1.PodSucceeded {
					// TODO Go routine to pump pod logs to stdout? Should we do that no matter what?
					if err := dumpLog(podsClient, p.Name, os.Stdout); err != nil {
						return err
					}
					watch.Stop()
				} else if p.Status.Phase == corev1.PodPending || p.Status.Phase == corev1.PodFailed {
					for _, c := range p.Status.ContainerStatuses {
						if c.State.Waiting != nil && c.State.Waiting.Reason == "ImagePullBackOff" {
							return fmt.Errorf("unable to pull image '%s' needed for installation", c.Image)
						} else if c.State.Terminated != nil && c.State.Terminated.Reason == "Error" {
							// TODO For now just copy logs over?
							if err := dumpLog(podsClient, p.Name, os.Stderr); err != nil {
								return err
							}

							return fmt.Errorf("installation encountered an error")
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
	r, err := podsClient.GetLogs(name, nil).Stream()
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

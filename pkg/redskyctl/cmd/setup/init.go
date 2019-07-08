package setup

import (
	"os"
	"path/filepath"

	"github.com/gramLabs/redsky/pkg/api"
	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
)

func NewInitCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewSetupOptions(ioStreams)

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Red Sky in a cluster",
		Long:  "The initialize command will install (or optionally generate) the required Red Sky manifests.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	o.AddFlags(cmd)

	return cmd
}

func (o *SetupOptions) initCluster() error {
	// TODO Should the util.Factory expose the configuration for bootstraping? Or should be build this some other way?
	clientConfig, err := api.DefaultConfig()
	if err != nil {
		return err
	}

	bootstrapConfig, err := NewBootstrapInitConfig(o, clientConfig)
	if err != nil {
		return err
	}

	// A bootstrap dry run just means serialize the bootstrap config
	if o.Bootstrap && o.DryRun {
		return bootstrapConfig.Marshal(o.Out)
	}

	// If there are any left over bootstrap objects, remove them before initializing
	deleteFromCluster(bootstrapConfig)

	// We need the namespace to exist so we can bind roles to the service account
	if _, err = bootstrapConfig.namespacesClient.Create(&bootstrapConfig.Namespace); err != nil && os.IsExist(err) {
		return err
	}

	// Create the bootstrap config to initiate the install job
	defer deleteFromCluster(bootstrapConfig)
	if err = createInCluster(bootstrapConfig); err != nil {
		return err
	}

	// If we are bootstraping the job will never start so we are done
	if o.Bootstrap {
		return nil
	}

	// Wait for the job to finish
	if err = waitForJob(o.ClientSet.CoreV1().Pods(o.namespace), bootstrapConfig.Job.Name, o.Out, o.ErrOut); err != nil {
		return err
	}

	return nil
}

// The current implementation of Kustomize exec plugins use an executable whose name matches the plugin
// kind and accepts a single argument (the config input file). To support that we create a symlink to the
// `redskyctl` executable from the location Kustomize will invoke it.
func (o *SetupOptions) initKustomize() error {
	e, err := os.Executable()
	if err != nil {
		return err
	}

	p := filepath.Join(kustomizePluginDir()...)
	s := filepath.Join(p, KustomizePluginKind)

	if err = os.MkdirAll(p, 0700); err != nil {
		return err
	}

	if _, err = os.Lstat(s); err == nil {
		if err = os.Remove(s); err != nil {
			return err
		}
	}

	if err = os.Symlink(e, s); err != nil {
		return err
	}

	return nil
}

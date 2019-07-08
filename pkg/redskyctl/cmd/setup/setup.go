package setup

import (
	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

const (
	KustomizePluginKind = "ExperimentGenerator"
)

type SetupOptions struct {
	Bootstrap bool
	DryRun    bool
	Kustomize bool

	namespace string
	name      string

	ClientSet *kubernetes.Clientset

	Run func() error
	cmdutil.IOStreams
}

func NewSetupOptions(ioStreams cmdutil.IOStreams) *SetupOptions {
	return &SetupOptions{
		namespace: "redsky-system",
		name:      "redsky-bootstrap",
		IOStreams: ioStreams,
	}
}

func (o *SetupOptions) AddFlags(cmd *cobra.Command) {
	// TODO Adjust usage strings based on `cmd.Name()`
	cmd.Flags().BoolVar(&o.Bootstrap, "bootstrap", false, "stop after creating the bootstrap configuration")
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "generate the manifests instead of applying them")
	cmd.Flags().BoolVar(&o.Kustomize, "kustomize", false, "install/update the Kustomize plugin and exit")
}

func (o *SetupOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	var err error
	o.ClientSet, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	switch cmd.Name() {
	case "init":
		if o.Kustomize {
			o.Run = o.initKustomize
		} else {
			o.Run = o.initCluster
		}
	case "reset":
		if o.Kustomize {
			o.Run = o.resetKustomize
		} else {
			o.Run = o.resetCluster
		}
	default:
		o.Run = func() error { panic("invalid command for setup: " + cmd.Name()) }
	}

	return nil
}

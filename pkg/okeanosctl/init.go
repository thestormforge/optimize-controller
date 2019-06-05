package okeanosctl

import (
	"fmt"
	"path/filepath"

	cmdutil "github.com/gramLabs/okeanos/pkg/okeanosctl/util"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type initOptions struct {
	kubeconfig string
}

func newInitOptions() *initOptions {
	o := &initOptions{}

	if homeDir := cmdutil.HomeDir(); homeDir != "" {
		o.kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	return o
}

func newInitCommand() *cobra.Command {
	o := newInitOptions()

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Okeanos in a cluster",
		Long:  "The initialize command will install (or optionally generate) the required Okeanos manifests.",
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.run())
		},
	}

	// TODO This should probably be on the parent command or in a util for common client commands
	cmd.Flags().StringVar(&o.kubeconfig, "kubeconfig", o.kubeconfig, "absolute path to the kubeconfig file")
	if o.kubeconfig == "" {
		cmd.MarkFlagRequired("kubeconfig")
	}

	return cmd
}

func (o *initOptions) run() error {
	config, err := clientcmd.BuildConfigFromFlags("", o.kubeconfig)
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	// TODO How do we use clientset to apply the manifests?
	if clientset == nil {
		return fmt.Errorf("Missing client")
	}

	// We have the generated code in /config that based on the annotations...
	// We either need to use those manifests directly by embedding them as static assets in the binary
	// Or we need to deserialize those manifests into client objects
	// (does the generator do that already and just serializes it? can we get it to dump Go code?)

	return nil
}

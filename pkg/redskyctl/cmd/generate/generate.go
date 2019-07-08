package generate

import (
	"io"
	"io/ioutil"

	redskyv1alpha1 "github.com/gramLabs/redsky/pkg/apis/redsky/v1alpha1"
	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type ExperimentGenerator struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

type GenerateOptions struct {
	Config *ExperimentGenerator
	cmdutil.IOStreams
}

func NewGenerateOptions(ioStreams cmdutil.IOStreams) *GenerateOptions {
	return &GenerateOptions{
		IOStreams: ioStreams,
	}
}

func NewGenerateCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewGenerateOptions(ioStreams)

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate experiments",
		Long:  "Generate an experiment manifest from a configuration file.",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *GenerateOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if b, err := ioutil.ReadFile(args[0]); err != nil {
		return err
	} else if err := yaml.Unmarshal(b, &o.Config); err != nil {
		return err
	}
	return nil
}

func (o *GenerateOptions) Run() error {
	e := redskyv1alpha1.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: o.Config.Namespace,
			Name:      o.Config.Name,
		},
	}

	// TODO Populate this thing

	if err := serialize(&e, o.Out); err != nil {
		return err
	}
	return nil
}

func serialize(e *redskyv1alpha1.Experiment, w io.Writer) error {
	scheme := runtime.NewScheme()
	_ = redskyv1alpha1.AddToScheme(scheme)
	u := &unstructured.Unstructured{}
	if err := scheme.Convert(e, u, runtime.InternalGroupVersioner); err != nil {
		return err
	}
	if b, err := yaml.Marshal(u); err != nil {
		return err
	} else if _, err := w.Write(b); err != nil {
		return err
	}
	return nil
}

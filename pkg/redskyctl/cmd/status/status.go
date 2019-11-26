package status

import (
	"fmt"
	"strings"

	"github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	redskykube "github.com/redskyops/k8s-experiment/pkg/kubernetes"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/k8s-experiment/pkg/util"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TODO Have --wait
// TODO Filters?

const (
	statusLong    = `Check in cluster experiment or trial status`
	statusExample = ``
)

type StatusOptions struct {
	Namespace string
	Name      string

	includeNamespace bool

	Printer         cmdutil.ResourcePrinter
	RedSkyClientSet *redskykube.Clientset

	cmdutil.IOStreams
}

func NewStatusOptions(ioStreams cmdutil.IOStreams) *StatusOptions {
	return &StatusOptions{
		IOStreams: ioStreams,
	}
}

func NewStatusCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewStatusOptions(ioStreams)

	printFlags := cmdutil.NewPrintFlags(cmdutil.NewTableMeta(o))
	printFlags.TablePrintFlags.ShowLabels = nil

	cmd := &cobra.Command{
		Use:     "status [NAME]",
		Short:   "Check in cluster experiment status",
		Long:    statusLong,
		Example: statusExample,
		Args:    cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, printFlags, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	printFlags.AddFlags(cmd)

	return cmd
}

func (o *StatusOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, printFlags *cmdutil.PrintFlags, args []string) error {
	if cs, err := f.RedSkyClientSet(); err != nil {
		return err
	} else {
		o.RedSkyClientSet = cs
	}

	// Get the individual name
	if len(args) > 0 {
		o.Name = args[0]
	}

	// Get the namespace to use
	var err error
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	// Construct a printer
	o.Printer, err = printFlags.ToPrinter()
	if err != nil {
		return err
	}

	return nil
}

func (o *StatusOptions) Run() error {
	var status interface{}
	if o.Name != "" {
		exp, err := o.RedSkyClientSet.RedskyopsV1alpha1().Experiments(o.Namespace).Get(o.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		list, err := o.listTrials(exp)
		if err != nil {
			return err
		}

		o.includeNamespace = !hasUniqueNamespace(list)
		status = list
	} else {
		list, err := o.RedSkyClientSet.RedskyopsV1alpha1().Experiments("").List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		o.includeNamespace = !hasUniqueNamespace(list)
		status = list
	}

	return o.Printer.PrintObj(status, o.Out)
}

func (o *StatusOptions) listTrials(exp *v1alpha1.Experiment) (*v1alpha1.TrialList, error) {
	// Get the trials for a specific experiment
	opts := metav1.ListOptions{}
	if sel, err := util.MatchingSelector(exp.GetTrialSelector()); err != nil {
		return nil, err
	} else {
		sel.ApplyToListOptions(&opts)
	}

	list, err := o.RedSkyClientSet.RedskyopsV1alpha1().Trials("").List(opts)
	if err != nil {
		return nil, err
	}

	return list, nil
}

func hasUniqueNamespace(obj runtime.Object) bool {
	if !meta.IsListType(obj) {
		return true
	}

	var ns string
	err := meta.EachListItem(obj, func(o runtime.Object) error {
		if acc, err := meta.Accessor(o); err != nil {
			return err
		} else if ns == "" {
			ns = acc.GetNamespace()
			return nil
		} else if ns != acc.GetNamespace() {
			// Use the error to stop iteration early
			return fmt.Errorf("found multiple namesapces")
		}
		return nil
	})
	return err == nil
}

func (o *StatusOptions) ExtractValue(obj runtime.Object, column string) (string, error) {
	switch column {
	case "status":
		switch v := obj.(type) {
		case *v1alpha1.Trial:
			return v.Status.Summary, nil
		case *v1alpha1.Experiment:
			return v.Status.Summary, nil
		}
	}
	return "", nil
}

func (o *StatusOptions) Columns(outputFormat string) []string {
	var columns []string
	if o.includeNamespace {
		columns = append(columns, "namespace")
	}
	columns = append(columns, "name", "status")
	return columns
}

func (o *StatusOptions) Allow(outputFormat string) bool {
	return outputFormat == ""
}

func (*StatusOptions) Header(outputFormat string, column string) string {
	return strings.ToUpper(column)
}

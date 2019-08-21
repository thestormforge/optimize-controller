package status

import (
	"fmt"
	"io"

	"github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	redskykube "github.com/redskyops/k8s-experiment/pkg/kubernetes"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/k8s-experiment/pkg/util"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TODO Have --wait
// TODO Filters?

const (
	statusLong    = `Check in cluster status for each trial in an experiment`
	statusExample = ``
)

type TrialStatusPrinter interface {
	PrintTrialListStatus(*v1alpha1.TrialList, io.Writer) error
}

type StatusOptions struct {
	Name         string
	Namespace    string
	OutputFormat string

	Printer         TrialStatusPrinter
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

	cmd := &cobra.Command{
		Use:     "status NAME",
		Short:   "Check in cluster experiment status",
		Long:    statusLong,
		Example: statusExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Experiment namespace in the Kubernetes cluster.")
	cmd.Flags().StringVarP(&o.OutputFormat, "output", "o", "", "Output format. One of: json|yaml|name.")

	return cmd
}

func (o *StatusOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	if cs, err := f.RedSkyClientSet(); err != nil {
		return err
	} else {
		o.RedSkyClientSet = cs
	}

	o.Name = args[0]
	if o.Namespace == "" {
		o.Namespace = "default"
	}

	switch o.OutputFormat {
	case "":
		o.Printer = &tablePrinter{}
	case "json":
		o.Printer = &jsonYamlPrinter{}
	case "yaml":
		o.Printer = &jsonYamlPrinter{yaml: true}
	case "name":
		o.Printer = &namePrinter{}
	default:
		return fmt.Errorf("unknown output format: %s", o.OutputFormat)
	}

	return nil
}

func (o *StatusOptions) Run() error {
	exp, err := o.RedSkyClientSet.RedskyopsV1alpha1().Experiments(o.Namespace).Get(o.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{}
	if sel, err := util.MatchingSelector(exp.GetTrialSelector()); err != nil {
		return err
	} else {
		sel.ApplyToListOptions(&opts)
	}

	list, err := o.RedSkyClientSet.RedskyopsV1alpha1().Trials("").List(opts)
	if err != nil {
		return err
	}

	if err := o.Printer.PrintTrialListStatus(list, o.Out); err != nil {
		return err
	}

	return nil
}

// Returns a string to summarize the trial status
func summarize(status *v1alpha1.TrialStatus) string {
	s := "Created"
	for i := range status.Conditions {
		c := status.Conditions[i]
		switch c.Type {
		case v1alpha1.TrialSetupCreated:
			switch c.Status {
			case corev1.ConditionTrue:
				s = "Setup Created"
			case corev1.ConditionFalse:
				s = "Setting up"
			case corev1.ConditionUnknown:
				s = "Setting up"
			}
		case v1alpha1.TrialSetupDeleted:
			switch c.Status {
			case corev1.ConditionTrue:
				s = "Setup Deleted"
			case corev1.ConditionFalse:
				s = "Tearing Down"
			}
		case v1alpha1.TrialPatched:
			switch c.Status {
			case corev1.ConditionTrue:
				s = "Patched"
			case corev1.ConditionFalse:
				s = "Patching"
			case corev1.ConditionUnknown:
				s = "Patching"
			}
		case v1alpha1.TrialStable:
			switch c.Status {
			case corev1.ConditionTrue:
				if status.StartTime != nil {
					s = "Running"
				} else {
					s = "Stabilized"
				}
			case corev1.ConditionFalse:
				s = "Waiting"
			case corev1.ConditionUnknown:
				s = "Waiting"
			}
		case v1alpha1.TrialObserved:
			switch c.Status {
			case corev1.ConditionTrue:
				s = "Captured"
			case corev1.ConditionFalse:
				s = "Capturing"
			case corev1.ConditionUnknown:
				s = "Capturing"
			}
		case v1alpha1.TrialComplete:
			switch c.Status {
			case corev1.ConditionTrue:
				return "Completed"
			}
		case v1alpha1.TrialFailed:
			switch c.Status {
			case corev1.ConditionTrue:
				return "Failed"
			}
		}
	}
	return s
}

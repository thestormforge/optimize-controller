package suggest

import (
	"context"
	"encoding/json"
	"strconv"

	redsky "github.com/gramLabs/redsky/pkg/api/redsky/v1alpha1"
	"github.com/gramLabs/redsky/pkg/apis/redsky/v1alpha1"
	"github.com/gramLabs/redsky/pkg/controller/experiment"
	redskykube "github.com/gramLabs/redsky/pkg/kubernetes"
	cmdutil "github.com/gramLabs/redsky/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	suggestLong    = `Manually suggest assignments for a trial.`
	suggestExample = ``
)

// SuggestionSource provides suggested parameter assignments
type SuggestionSource interface {
	AssignInt(name string, min, max int64) (int64, error)
	AssignDouble(name string, min, max float64) (float64, error)
}

type SuggestOptions struct {
	Remote    bool
	Namespace string
	Name      string

	Suggestions     SuggestionSource
	RedSkyAPI       *redsky.API
	RedSkyClientSet *redskykube.Clientset

	cmdutil.IOStreams
}

func NewSuggestOptions(ioStreams cmdutil.IOStreams) *SuggestOptions {
	return &SuggestOptions{
		IOStreams: ioStreams,
	}
}

func NewSuggestCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewSuggestOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "suggest",
		Short:   "Suggest assignments",
		Long:    suggestLong,
		Example: suggestExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.Remote, "remote", false, "Create the suggestion on the Red Sky server")
	cmd.Flags().StringVar(&o.Namespace, "namespace", "", "Experiment namespace in the Kubernetes cluster")
	cmd.Flags().StringVar(&o.Name, "name", "", "Experiment name to suggest assignments for")
	_ = cmd.MarkFlagRequired("name")

	sourceFlags := NewSuggestionSourceFlags(ioStreams)
	sourceFlags.AddFlags(cmd)
	o.Suggestions = sourceFlags

	return cmd
}

func (o *SuggestOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	if o.Remote {
		// Send it to the remote Red Sky API
		if api, err := f.RedSkyAPI(); err != nil {
			return err
		} else {
			o.RedSkyAPI = &api
		}
	} else {
		// Send it to the Kube cluster
		if cs, err := f.RedSkyClientSet(); err != nil {
			return err
		} else {
			o.RedSkyClientSet = cs
		}

		// Provide a default value for the namespace
		if o.Namespace == "" {
			o.Namespace = "default"
		}
	}
	return nil
}

func (o *SuggestOptions) Run() error {
	// If we have a clientset then create the suggestion in the cluster
	if o.RedSkyClientSet != nil {
		if err := createInClusterSuggestion(o.Namespace, o.Name, o.Suggestions, o.RedSkyClientSet); err != nil {
			return err
		}
	}

	// If we have an API then create the suggestion on the remote server
	if o.RedSkyAPI != nil {
		if err := createRemoteSuggestion(o.Name, o.Suggestions, *o.RedSkyAPI); err != nil {
			return err
		}
	}

	return nil
}

func createInClusterSuggestion(namespace, name string, suggestions SuggestionSource, clientset *redskykube.Clientset) error {
	exp, err := clientset.RedskyV1alpha1().Experiments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	trial := &v1alpha1.Trial{}
	experiment.PopulateTrialFromTemplate(exp, trial, namespace)
	trial.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(exp, exp.GetSelfReference().GroupVersionKind())})
	trial.Finalizers = nil

	for _, p := range exp.Spec.Parameters {
		v, err := suggestions.AssignInt(p.Name, p.Min, p.Max)
		if err != nil {
			return nil
		}
		trial.Spec.Assignments = append(trial.Spec.Assignments, v1alpha1.Assignment{
			Name:  p.Name,
			Value: v,
		})
	}

	_, err = clientset.RedskyV1alpha1().Trials(namespace).Create(trial)
	return err
}

func createRemoteSuggestion(name string, suggestions SuggestionSource, api redsky.API) error {
	exp, err := api.GetExperimentByName(context.TODO(), redsky.NewExperimentName(name))
	if err != nil {
		return err
	}

	ta := redsky.TrialAssignments{}
	for _, p := range exp.Parameters {
		switch p.Type {
		case redsky.ParameterTypeInteger:
			min, err := p.Bounds.Min.Int64()
			if err != nil {
				return err
			}
			max, err := p.Bounds.Max.Int64()
			if err != nil {
				return err
			}
			a, err := suggestions.AssignInt(p.Name, min, max)
			if err != nil {
				return err
			}
			ta.Assignments = append(ta.Assignments, redsky.Assignment{
				ParameterName: p.Name,
				Value:         json.Number(strconv.FormatInt(a, 10)),
			})
		case redsky.ParameterTypeDouble:
			min, err := p.Bounds.Min.Float64()
			if err != nil {
				return err
			}
			max, err := p.Bounds.Max.Float64()
			if err != nil {
				return err
			}
			a, err := suggestions.AssignDouble(p.Name, min, max)
			if err != nil {
				return err
			}
			ta.Assignments = append(ta.Assignments, redsky.Assignment{
				ParameterName: p.Name,
				Value:         json.Number(strconv.FormatFloat(a, 'f', -1, 64)),
			})
		}
	}

	_, err = api.CreateTrial(context.TODO(), exp.Trials, ta)
	return err
}

/*
Copyright 2019 GramLabs, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package suggest

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	redsky "github.com/redskyops/k8s-experiment/pkg/api/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	"github.com/redskyops/k8s-experiment/pkg/controller/experiment"
	redskykube "github.com/redskyops/k8s-experiment/pkg/kubernetes"
	"github.com/redskyops/k8s-experiment/pkg/kubernetes/scheme"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	suggestLong    = `Manually suggest assignments for a trial.`
	suggestExample = ``
)

// SuggestionSource provides suggested parameter assignments
type SuggestionSource interface {
	AssignInt(name string, min, max int64, def *int64) (int64, error)
	AssignDouble(name string, min, max float64, def *float64) (float64, error)
}

type SuggestOptions struct {
	Remote    bool
	Namespace string
	Name      string

	Suggestions      SuggestionSource
	RedSkyAPI        *redsky.API
	RedSkyClientSet  *redskykube.Clientset
	ControllerReader client.Reader

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
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Experiment namespace in the Kubernetes cluster")

	// TODO Should this be an argument instead of an option?
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

		// This is a brutal hack to allow us to re-use the controller code
		// TODO Can we make a lightweight version of this that leverages the clientset we already have? It needs to work with namespaces also...
		if rc, err := f.ToRESTConfig(); err != nil {
			return err
		} else {
			s := runtime.NewScheme()
			if err := scheme.AddToScheme(s); err != nil {
				return err
			}
			if err := corev1.AddToScheme(s); err != nil {
				return err
			}
			if cc, err := client.New(rc, client.Options{Scheme: s}); err != nil {
				return err
			} else {
				o.ControllerReader = cc
			}
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
		if err := createInClusterSuggestion(o.Namespace, o.Name, o.Suggestions, o.RedSkyClientSet, o.ControllerReader); err != nil {
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

func createInClusterSuggestion(namespace, name string, suggestions SuggestionSource, clientset *redskykube.Clientset, controllerClient client.Reader) error {
	exp, err := clientset.RedskyV1alpha1().Experiments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	trialList, err := experiment.FindTrials(controllerClient, exp)
	if err != nil {
		return err
	}
	trialNamespace, err := experiment.FindAvailableNamespace(controllerClient, exp, trialList.Items)
	if err != nil {
		return err
	}
	if trialNamespace == "" {
		return fmt.Errorf("no available namespace to create trial")
	}

	trial := &v1alpha1.Trial{}
	experiment.PopulateTrialFromTemplate(exp, trial, trialNamespace)
	trial.Finalizers = nil
	if err := controllerutil.SetControllerReference(exp, trial, scheme.Scheme); err != nil {
		return err
	}

	for _, p := range exp.Spec.Parameters {
		v, err := suggestions.AssignInt(p.Name, p.Min, p.Max, nil)
		if err != nil {
			return err
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
			var def *int64
			a, err := suggestions.AssignInt(p.Name, min, max, def)
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
			var def *float64
			a, err := suggestions.AssignDouble(p.Name, min, max, def)
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

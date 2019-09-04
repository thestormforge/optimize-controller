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
	"github.com/redskyops/k8s-experiment/pkg/util"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	suggestLong    = `Suggest assignments for a new trial run`
	suggestExample = ``
)

// TODO Accept suggestion inputs from standard input, what formats?

// SuggestionSource provides suggested parameter assignments
type SuggestionSource interface {
	AssignInt(name string, min, max int64, def *int64) (int64, error)
	AssignDouble(name string, min, max float64, def *float64) (float64, error)
}

type SuggestOptions struct {
	Namespace       string
	Name            string
	ForceRedSkyAPI  bool
	ForceKubernetes bool

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
		Use:     "suggest NAME",
		Short:   "Suggest assignments",
		Long:    suggestLong,
		Example: suggestExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	sourceFlags := NewSuggestionSourceFlags(ioStreams)
	sourceFlags.AddFlags(cmd)
	o.Suggestions = sourceFlags

	return cmd
}

func (o *SuggestOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.Name = args[0]

	if !o.ForceKubernetes {
		if api, err := f.RedSkyAPI(); err == nil {
			// Send it to the remote Red Sky API
			o.RedSkyAPI = &api
		} else if o.ForceRedSkyAPI {
			// Failure to explicitly use the Red Sky API
			return err
		}
	}

	if o.RedSkyAPI == nil {
		if cs, err := f.RedSkyClientSet(); err == nil {
			// Send it to the Kube cluster
			o.RedSkyClientSet = cs

			// Get the namespace to use
			o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
			if err != nil {
				return err
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
		} else if o.ForceKubernetes {
			// Failure to explicitly use the Kubernetes cluster
			return err
		}
	}

	if o.RedSkyAPI == nil && o.RedSkyClientSet == nil {
		return fmt.Errorf("unable to connect")
	}

	return nil
}

func (o *SuggestOptions) Run() error {
	// If we have an API then create the suggestion using the Red Sky API
	if o.RedSkyAPI != nil {
		if err := createRedSkyAPISuggestion(o.Name, o.Suggestions, *o.RedSkyAPI); err != nil {
			return err
		}
	}

	// If we have a clientset then create the suggestion in the Kubernetes cluster
	if o.RedSkyClientSet != nil {
		if err := createKubernetesSuggestion(o.Namespace, o.Name, o.Suggestions, o.RedSkyClientSet, o.ControllerReader); err != nil {
			return err
		}
	}

	return nil
}

func createRedSkyAPISuggestion(name string, suggestions SuggestionSource, api redsky.API) error {
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

func createKubernetesSuggestion(namespace, name string, suggestions SuggestionSource, clientset *redskykube.Clientset, controllerClient client.Reader) error {
	exp, err := clientset.RedskyopsV1alpha1().Experiments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{}
	if sel, err := util.MatchingSelector(exp.GetTrialSelector()); err != nil {
		return err
	} else {
		sel.ApplyToListOptions(&opts)
	}

	trialList, err := clientset.RedskyopsV1alpha1().Trials("").List(opts)
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

	_, err = clientset.RedskyopsV1alpha1().Trials(trialNamespace).Create(trial)
	return err
}

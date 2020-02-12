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

	"github.com/redskyops/redskyops-controller/internal/experiment"
	"github.com/redskyops/redskyops-controller/internal/meta"
	"github.com/redskyops/redskyops-controller/pkg/apis/redsky/v1alpha1"
	redskykube "github.com/redskyops/redskyops-controller/pkg/kubernetes"
	"github.com/redskyops/redskyops-controller/pkg/kubernetes/scheme"
	cmdutil "github.com/redskyops/redskyops-controller/pkg/redskyctl/util"
	redskyapi "github.com/redskyops/redskyops-controller/redskyapi/experiments/v1alpha1"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// SuggestOptions is the configuration for suggesting assignments
type SuggestOptions struct {
	Namespace       string
	Name            string
	ForceRedSkyAPI  bool
	ForceKubernetes bool

	Suggestions      SuggestionSource
	RedSkyAPI        *redskyapi.API
	RedSkyClientSet  *redskykube.Clientset
	ControllerClient client.Client

	cmdutil.IOStreams
}

// NewSuggestOptions returns a new suggestion options struct
func NewSuggestOptions(ioStreams cmdutil.IOStreams) *SuggestOptions {
	return &SuggestOptions{
		IOStreams: ioStreams,
	}
}

// NewSuggestCommand returns a new suggestion command
func NewSuggestCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewSuggestOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "suggest NAME",
		Short:   "Suggest assignments",
		Long:    suggestLong,
		Example: suggestExample,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(cmd, o.Complete(f, cmd, args))
			cmdutil.CheckErr(cmd, o.Run())
		},
	}

	sourceFlags := NewSuggestionSourceFlags(ioStreams)
	sourceFlags.AddFlags(cmd)
	o.Suggestions = sourceFlags

	return cmd
}

// Complete the suggestion options
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
				if err := rbacv1.AddToScheme(s); err != nil {
					return err
				}
				if cc, err := client.New(rc, client.Options{Scheme: s}); err != nil {
					return err
				} else {
					o.ControllerClient = cc
				}
			}
		} else if o.ForceKubernetes {
			// Failure to explicitly use the Kubernetes cluster
			return err
		}
	}

	if o.RedSkyAPI == nil && o.RedSkyClientSet == nil {
		return fmt.Errorf("unable to connect, make sure either your Red Sky API or Kube configuration is valid")
	}

	return nil
}

// Run the suggestion options
func (o *SuggestOptions) Run() error {
	// If we have an API then create the suggestion using the Red Sky API
	if o.RedSkyAPI != nil {
		if err := createRedSkyAPISuggestion(o.Name, o.Suggestions, *o.RedSkyAPI); err != nil {
			return err
		}
	}

	// If we have a clientset then create the suggestion in the Kubernetes cluster
	if o.RedSkyClientSet != nil {
		if err := createKubernetesSuggestion(o.Namespace, o.Name, o.Suggestions, o.RedSkyClientSet, o.ControllerClient); err != nil {
			return err
		}
	}

	return nil
}

func createRedSkyAPISuggestion(name string, suggestions SuggestionSource, api redskyapi.API) error {
	exp, err := api.GetExperimentByName(context.TODO(), redskyapi.NewExperimentName(name))
	if err != nil {
		return err
	}

	ta := redskyapi.TrialAssignments{}
	for _, p := range exp.Parameters {
		switch p.Type {
		case redskyapi.ParameterTypeInteger:
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
			ta.Assignments = append(ta.Assignments, redskyapi.Assignment{
				ParameterName: p.Name,
				Value:         json.Number(strconv.FormatInt(a, 10)),
			})
		case redskyapi.ParameterTypeDouble:
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
			ta.Assignments = append(ta.Assignments, redskyapi.Assignment{
				ParameterName: p.Name,
				Value:         json.Number(strconv.FormatFloat(a, 'f', -1, 64)),
			})
		}
	}

	_, err = api.CreateTrial(context.TODO(), exp.Trials, ta)
	return err
}

func createKubernetesSuggestion(namespace, name string, suggestions SuggestionSource, clientset *redskykube.Clientset, controllerClient client.Client) error {
	exp, err := clientset.RedskyopsV1alpha1().Experiments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	opts := metav1.ListOptions{}
	if sel, err := meta.MatchingSelector(exp.TrialSelector()); err != nil {
		return err
	} else {
		sel.ApplyToListOptions(&opts)
	}

	trialList, err := clientset.RedskyopsV1alpha1().Trials("").List(opts)
	if err != nil {
		return err
	}

	trialNamespace, err := experiment.NextTrialNamespace(controllerClient, context.Background(), exp, trialList)
	if err != nil {
		return err
	}
	if trialNamespace == "" {
		return fmt.Errorf("experiment is already at scale")
	}

	trial := &v1alpha1.Trial{}
	experiment.PopulateTrialFromTemplate(exp, trial)
	trial.Namespace = trialNamespace

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

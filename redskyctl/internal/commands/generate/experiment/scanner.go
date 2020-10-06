/*
Copyright 2020 GramLabs, Inc.

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

package experiment

import (
	"fmt"
	"strings"

	"github.com/redskyops/redskyops-controller/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/k8sdeps/kunstruct"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/loader"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
)

type Scanner struct {
	FileSystem filesys.FileSystem
}

func (s *Scanner) ScanInto(references []string, exp *v1beta1.Experiment) error {
	// The collection of all the resources lists (limits/requests) we have found
	var rl []resourceLists

	// Iterate over the references and scan each resource we find
	for _, r := range references {
		rm, err := s.load(r)
		if err != nil {
			return err
		}

		// Iterate of the resources in the Kustomize resource map and scan each one
		for _, rr := range rm.Resources() {
			e, err := s.scan(rr)
			if err != nil {
				return err
			}

			if e != nil {
				rl = append(rl, *e)
			}
		}
	}

	// Apply the accumulated results to the experiment
	if err := s.apply(rl, exp); err != nil {
		return err
	}

	return nil
}

func (s *Scanner) load(ref string) (resmap.ResMap, error) {
	// This is a hack so you can use process substitution to get resources in for scanning
	if strings.HasPrefix(ref, "/dev/fd/") {
		kf := kunstruct.NewKunstructuredFactoryImpl()
		rf := resource.NewFactory(kf)
		rmf := resmap.NewFactory(rf, nil)
		ldr := loader.NewFileLoaderAtRoot(s.FileSystem)
		return rmf.FromFile(ldr, ref)
	}

	return krusty.MakeKustomizer(s.FileSystem, krusty.MakeDefaultOptions()).Run(ref)
}

func (s *Scanner) scan(r *resource.Resource) (*resourceLists, error) {
	// Inspect the resource for resource lists (i.e. collections of requests/limits)
	paths, err := s.findPaths(r)
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	rl := &resourceLists{resourcesPaths: paths}

	// Update the target reference
	gvk := r.GetGvk()
	rl.targetRef.Name = r.GetName()
	rl.targetRef.Namespace = r.GetNamespace()
	rl.targetRef.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    gvk.Kind,
	})

	return rl, nil
}

func (s *Scanner) findPaths(r *resource.Resource) ([]fieldPath, error) {
	// This is going to take the naive approach. We are going to look for `apps/v1` _only_
	gvk := r.GetGvk()
	if gvk.Group != "apps" || gvk.Version != "v1" {
		return nil, nil
	}

	// Expect to find a container list
	containers, ok := fieldPath([]string{"spec", "template", "spec", "containers"}).read(r.Map()).([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected sequence")
	}

	// For each container, add a path
	p := make([]fieldPath, 0, len(containers))
	for i := range containers {
		c, ok := containers[i].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("expected mapping")
		}
		p = append(p, []string{
			"spec", "template", "spec",
			fmt.Sprintf("containers{name=%s}", c["name"]),
			"resources"})
	}

	return p, nil
}

func (s *Scanner) apply(rl []resourceLists, exp *v1beta1.Experiment) error {
	// TODO We can probably be smarter determining if a prefix is necessary
	needsPrefix := len(rl) > 1

	for i := range rl {
		parameters, patch, err := rl[i].generate(needsPrefix)
		if err != nil {
			return err
		}

		// Set arbitrary bounds on each parameter
		for j := range parameters {
			parameters[j].Min = 100
			parameters[j].Max = 4000
		}

		exp.Spec.Parameters = append(exp.Spec.Parameters, parameters...)
		exp.Spec.Patches = append(exp.Spec.Patches, *patch)
	}

	return nil
}

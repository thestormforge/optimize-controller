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
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment/locust"
	"github.com/redskyops/redskyops-controller/redskyctl/internal/commands/generate/experiment/stormforger"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/api/resmap"
)

func (g *Generator) addScenario(arm resmap.ResMap, list *corev1.List) error {
	for i := range g.Application.Scenarios {
		switch {

		case g.Application.Scenarios[i].StormForger != nil:
			return stormforger.AddTrialJob(&g.Application.Scenarios[i], &g.Application, g.fs, arm, list)

		case g.Application.Scenarios[i].Locust != nil:
			return locust.AddTrialJob(&g.Application.Scenarios[i], &g.Application, g.fs, arm, list)

		}
	}

	return nil
}

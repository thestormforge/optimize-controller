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
	"bytes"
	"log"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

// This doesnt necessarily need to live here, but seemed to make sense
func Run(appCh chan *redskyappsv1alpha1.Application) {
	for app := range appCh {
		// TODO
		//
		// How do we want to scope the app.yaml here?
		//
		// Should we assume that since we're in control of the endpoint that is generating
		// the app.yaml, we should only ever have 1 scenario/objective?
		// Or should we support fetching app.yaml from any endpoint
		// at which point we need to select an appropriate scenario/objective and handle
		// all the same nuances from the redskyctl generate experiment -
		// name/namespace/objective/scenario/resources location
		//
		// Do annotations here make sense to provide that hint around which
		// objective/scenario to pick?

		if app.Namespace == "" || app.Name == "" {
			log.Println("bad app.yaml")
			continue
		}

		g := &Generator{
			Application: *app,
		}

		// TODO
		// (larger note above) how do we want to handle scenario/objective filtering?
		g.SetDefaultSelectors()

		var inputsBuf bytes.Buffer

		// TODO
		// Since we're using konjure for this, we need to bundle all of the bins konjure supports
		// inside the controller container
		// Or should we swap to client libraries?
		if err := g.Execute(kio.ByteWriter{Writer: &inputsBuf}); err != nil {
			log.Println(err)
			continue
		}

		// Do we want to look for annotations to trigger the start of the experiment?
		// ex, `stormforge.dev/application-verified: true` translates to `exp.spec.replicas = 1`

		// TODO
		// What next with the generated experiment?
		//
		// handle creation here?
		// // just experiment.yaml, no rbac?
		// maybe send up to some api for preview/confirmation?
		// maybe both?
		// maybe neither?
	}
}

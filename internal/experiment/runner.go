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
	"context"
	"log"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// This doesnt necessarily need to live here, but seemed to make sense
func Run(ctx context.Context, kclient client.Client, appCh chan *redskyappsv1alpha1.Application) {
	// api applicationsv1alpha1.API
	// Just a placeholder chan to illustrate what we'll be doing
	// eventually this will be replaced with something from the api
	// ex, for app := range <- api.Watch() {

	//appCh := make(chan *redskyappsv1alpha1.Application)

	for {
		select {
		case <-ctx.Done():
			return
		case app := <-appCh:
			if app.Namespace == "" || app.Name == "" {
				// api.UpdateStatus("failed")
				log.Println("bad app.yaml")
				continue
			}

			g := &Generator{
				Application: *app,
			}

			var inputsBuf bytes.Buffer

			// TODO
			// Since we're using konjure for this, we need to bundle all of the bins konjure supports
			// inside the controller container
			// Or should we swap to client libraries?
			if err := g.Execute(kio.ByteWriter{Writer: &inputsBuf}); err != nil {
				log.Println(err)
				continue
			}

			exp := &redskyv1beta1.Experiment{}
			if err := yaml.Unmarshal(inputsBuf.Bytes(), exp); err != nil {
				// api.UpdateStatus("failed")
				log.Println(err)
				continue
			}

			if _, ok := app.Annotations[redskyappsv1alpha1.AnnotationUserConfirmed]; !ok {
				var replicas int32 = 0
				exp.Spec.Replicas = &replicas
			}

			// TODO
			// How should we handle the rejection of an application ( user wanted to make
			// changes, so we need to delete the old experiment )

			if err := kclient.Create(ctx, exp); err != nil {
				// api.UpdateStatus("failed")
				log.Println("bad experiment")
				continue
			}
		}
	}
}

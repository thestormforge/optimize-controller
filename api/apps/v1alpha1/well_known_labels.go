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

package v1alpha1

// Application labels and annotations

const (
	// LabelApplication is the name of the application associated with an object.
	LabelApplication = "stormforge.io/application"

	// LabelScenario is the application scenario associated with an object.
	LabelScenario = "stormforge.io/scenario"

	// LabelObjective is the application objective associated with an object.
	LabelObjective = "stormforge.io/objective"

	// AnnotationLastScanned is the timestamp of the last application scan.
	AnnotationLastScanned = "apps.stormforge.io/last-scanned"
)

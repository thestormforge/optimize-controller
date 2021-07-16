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

package v1beta2

// Experiment labels and annotations

const (
	// AnnotationExperimentURL is the URL of the experiment on the remote server
	AnnotationExperimentURL = "stormforge.io/experiment-url"
	// AnnotationNextTrialURL is the URL used to obtain the next trial suggestion
	AnnotationNextTrialURL = "stormforge.io/next-trial-url"
	// AnnotationReportTrialURL is the URL used to report trial observations
	AnnotationReportTrialURL = "stormforge.io/report-trial-url"
	// AnnotationServerSync controls additional behavior around synchronizing the experiment remotely
	AnnotationServerSync = "stormforge.io/server-sync"

	// LabelExperiment is the name of the experiment associated with an object
	LabelExperiment = "stormforge.io/experiment"
)

// Trial labels and annotations

const (
	// AnnotationInitializer is a comma-delimited list of initializing processes. Similar to a "finalizer", the trial
	// will not start executing until the initializer is empty.
	AnnotationInitializer = "stormforge.io/initializer"

	// LabelTrial contains the name of the trial associated with an object
	LabelTrial = "stormforge.io/trial"
	// LabelTrialRole contains the role in trial execution
	LabelTrialRole = "stormforge.io/trial-role"
)

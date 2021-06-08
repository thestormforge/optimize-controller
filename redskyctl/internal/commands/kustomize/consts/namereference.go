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

package consts

const nameReferenceFieldSpecs = `
nameReference:
- kind: Experiment
  group: optimize.stormforge.io
  fieldSpecs:
  - path: spec/template/spec/experimentRef/name
    group: optimize.stormforge.io
    kind: Experiment

- kind: ConfigMap
  version: v1
  fieldSpecs:
  - path: spec/template/spec/setupVolumes/configMap/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/setupTasks/helmValuesFrom/configMap/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/volumes/configMap/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/env/valueFrom/configMapKeyRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/env/valueFrom/configMapKeyRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/envFrom/configMapRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/envFrom/configMapRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/volumes/projected/sources/configMap/name
    group: optimize.stormforge.io
    kind: Experiment

- kind: Secret
  version: v1
  fieldSpecs:
  - path: spec/template/spec/template/spec/template/spec/volumes/secret/secretName
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/env/valueFrom/secretKeyRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/env/valueFrom/secretKeyRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/envFrom/secretRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/envFrom/secretRef/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/imagePullSecrets/name
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/volumes/projected/sources/secret/name
    group: optimize.stormforge.io
    kind: Experiment

- kind: ServiceAccount
  version: v1
  fieldSpecs:
  - path: spec/template/spec/setupServiceAccountName
    group: optimize.stormforge.io
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/serviceAccountName
    group: optimize.stormforge.io
    kind: Experiment

- kind: PersistentVolumeClaim
  version: v1
  fieldSpecs:
  - path: spec/template/spec/template/spec/template/spec/volumes/persistentVolumeClaim/claimName
    group: optimize.stormforge.io
    kind: Experiment

`

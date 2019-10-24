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

package consts

const nameReferenceFieldSpecs = `
nameReference:
- kind: Experiment
  group: redskyops.dev
  fieldSpecs:
  - path: spec/template/spec/experimentRef/name
    group: redskyops.dev
    kind: Experiment

- kind: ConfigMap
  version: v1
  fieldSpecs:
  - path: spec/template/spec/setupVolumes/configMap/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/setupTasks/helmValuesFrom/configMap/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/volumes/configMap/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/env/valueFrom/configMapKeyRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/env/valueFrom/configMapKeyRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/envFrom/configMapRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/envFrom/configMapRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/volumes/projected/sources/configMap/name
    group: redskyops.dev
    kind: Experiment

- kind: Secret
  version: v1
  fieldSpecs:
  - path: spec/template/spec/template/spec/template/spec/volumes/secret/secretName
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/env/valueFrom/secretKeyRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/env/valueFrom/secretKeyRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/containers/envFrom/secretRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/initContainers/envFrom/secretRef/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/imagePullSecrets/name
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/volumes/projected/sources/secret/name
    group: redskyops.dev
    kind: Experiment

- kind: ServiceAccount
  version: v1
  fieldSpecs:
  - path: spec/template/spec/setupServiceAccountName
    group: redskyops.dev
    kind: Experiment
  - path: spec/template/spec/template/spec/template/spec/serviceAccountName
    group: redskyops.dev
    kind: Experiment

- kind: PersistentVolumeClaim
  version: v1
  fieldSpecs:
  - path: spec/template/spec/template/spec/template/spec/volumes/persistentVolumeClaim/claimName
    group: redskyops.dev
    kind: Experiment

`

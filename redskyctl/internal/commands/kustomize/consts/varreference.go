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

const varReferenceFieldSpecs = `
varReference:
- path: spec/template/spec/template/spec/template/spec/containers/args
  group: redskyops.dev
  kind: Experiment

- path: spec/template/spec/template/spec/template/spec/containers/command
  group: redskyops.dev
  kind: Experiment

- path: spec/template/spec/template/spec/template/spec/containers/env/value
  group: redskyops.dev
  kind: Experiment

- path: spec/template/spec/template/spec/template/spec/containers/volumeMounts/mountPath
  group: redskyops.dev
  kind: Experiment

- path: spec/template/spec/template/spec/template/spec/initContainers/args
  group: redskyops.dev
  kind: Experiment

- path: spec/template/spec/template/spec/template/spec/initContainers/command
  group: redskyops.dev
  kind: Experiment

- path: spec/template/spec/template/spec/template/spec/initContainers/env/value
  group: redskyops.dev
  kind: Experiment

- path: spec/template/spec/template/spec/template/spec/initContainers/volumeMounts/mountPath
  group: redskyops.dev
  kind: Experiment

`

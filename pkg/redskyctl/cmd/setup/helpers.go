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
package setup

import (
	"io"
	"os"
	"path/filepath"
	"strings"

	redskyv1alpha1 "github.com/redskyops/k8s-experiment/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func waitForJob(podsClient clientcorev1.PodInterface, podWatch watch.Interface, out, errOut io.Writer) error {
	for event := range podWatch.ResultChan() {
		if p, ok := event.Object.(*corev1.Pod); ok {
			if p.Status.Phase == corev1.PodSucceeded {
				// TODO Go routine to pump pod logs to stdout? Should we do that no matter what?
				if err := dumpLog(podsClient, p.Name, out); err != nil {
					return err
				}
				podWatch.Stop()
			} else if p.Status.Phase == corev1.PodPending || p.Status.Phase == corev1.PodFailed {
				for _, c := range p.Status.ContainerStatuses {
					if c.State.Waiting != nil && c.State.Waiting.Reason == "ImagePullBackOff" {
						return &SetupError{ImagePullBackOff: c.Image}
					} else if c.State.Terminated != nil && c.State.Terminated.Reason == "Error" {
						// TODO For now just copy logs over?
						if err := dumpLog(podsClient, p.Name, errOut); err != nil {
							return err
						}
						return &SetupError{}
					}
				}
			} else if event.Type == watch.Deleted {
				return &SetupError{PodDeleted: true}
			}
		}
	}
	return nil
}

func dumpLog(podsClient clientcorev1.PodInterface, name string, w io.Writer) error {
	if w == nil {
		return nil
	}
	r, err := podsClient.GetLogs(name, &corev1.PodLogOptions{}).Stream()
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(w, r)
	if err != nil {
		return err
	}
	return nil
}

func kustomizePluginDir() []string {
	// This is not a full XDG Base Directory implementation, just enough for Kustomize
	// https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = os.Getenv("USERPROFILE")
		}
		// NOTE: This can produce just ".config" if the environment variables aren't set
		configDir = filepath.Join(homeDir, ".config")
	}
	return []string{configDir, "kustomize", "plugin", redskyv1alpha1.SchemeGroupVersion.Group, redskyv1alpha1.SchemeGroupVersion.Version, strings.ToLower(KustomizePluginKind)}
}

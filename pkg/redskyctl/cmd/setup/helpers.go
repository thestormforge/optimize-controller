package setup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	redskyv1alpha1 "github.com/gramLabs/redsky/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	watchutil "k8s.io/apimachinery/pkg/watch"
	clientcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func waitForJob(podsClient clientcorev1.PodInterface, name string, out, errOut io.Writer) error {
	watch, err := podsClient.Watch(metav1.ListOptions{LabelSelector: "job-name = " + name})
	if err != nil {
		return err
	}
	defer watch.Stop()
	for event := range watch.ResultChan() {
		if p, ok := event.Object.(*corev1.Pod); ok {
			if p.Status.Phase == corev1.PodSucceeded {
				// TODO Go routine to pump pod logs to stdout? Should we do that no matter what?
				if err := dumpLog(podsClient, p.Name, out); err != nil {
					return err
				}
				watch.Stop()
			} else if p.Status.Phase == corev1.PodPending || p.Status.Phase == corev1.PodFailed {
				for _, c := range p.Status.ContainerStatuses {
					if c.State.Waiting != nil && c.State.Waiting.Reason == "ImagePullBackOff" {
						return fmt.Errorf("unable to pull image '%s'", c.Image)
					} else if c.State.Terminated != nil && c.State.Terminated.Reason == "Error" {
						// TODO For now just copy logs over?
						if err := dumpLog(podsClient, p.Name, errOut); err != nil {
							return err
						}
						return fmt.Errorf("encountered an error")
					}
				}
			} else if event.Type == watchutil.Deleted {
				return fmt.Errorf("initialization pod was deleted before it could finish")
			}
		}
	}
	return nil
}

func dumpLog(podsClient clientcorev1.PodInterface, name string, w io.Writer) error {
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

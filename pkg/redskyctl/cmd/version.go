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
package cmd

import (
	"bytes"
	"fmt"
	"time"

	"github.com/redskyops/k8s-experiment/pkg/controller/trial"
	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/redskyops/k8s-experiment/pkg/version"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// TODO Add support for getting Red Sky server version
// TODO Add support for getting manager version in cluster
// TODO Add a "--notes" option to print the release notes?

const (
	versionLong    = `Show the version information for Red Sky Control.`
	versionExample = ``
)

type VersionOptions struct {
	Namespace       string
	SetupToolsImage bool

	root       *cobra.Command
	restConfig *rest.Config
	clientSet  *kubernetes.Clientset

	cmdutil.IOStreams
}

func NewVersionOptions(ioStreams cmdutil.IOStreams) *VersionOptions {
	return &VersionOptions{
		Namespace: "redsky-system",
		IOStreams: ioStreams,
	}
}

func NewVersionCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewVersionOptions(ioStreams)

	cmd := &cobra.Command{
		Use:     "version",
		Short:   "Print version information",
		Long:    versionLong,
		Example: versionExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd))
			cmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVar(&o.SetupToolsImage, "setuptools", false, "print only the name of the setuptools image")

	return cmd
}

func (o *VersionOptions) Complete(f cmdutil.Factory, cmd *cobra.Command) error {
	if c, err := f.ToRESTConfig(); err == nil {
		c.Timeout = 1 * time.Second // Don't try too hard
		if cs, err := kubernetes.NewForConfig(c); err == nil {
			o.restConfig = c
			o.clientSet = cs
		}
	}

	o.root = cmd.Root()

	return nil
}

func (o *VersionOptions) Run() error {
	if o.SetupToolsImage {
		// TODO We should have an option to print this as JSON with the pull policy, e.g. `{"image":"...", "imagePullPolicy":"..."}`...
		_, err := fmt.Fprintf(o.Out, "%s\n", trial.DefaultImage)
		return err
	}

	if err := o.redskyctlVersion(); err != nil {
		return err
	}

	if err := o.managerVersion(); err != nil {
		return err
	}

	return nil
}

func (o *VersionOptions) redskyctlVersion() error {
	_, err := fmt.Fprintf(o.Out, "%s version: %s\n", o.root.Name(), version.GetVersion())
	return err
}

func (o *VersionOptions) managerVersion() error {
	if o.restConfig == nil && o.clientSet == nil {
		return nil
	}

	var pod *corev1.Pod
	podList, err := o.clientSet.CoreV1().Pods(o.Namespace).List(metav1.ListOptions{LabelSelector: "control-plane=controller-manager"})
	if err != nil {
		// Silently ignore
		return nil
	}
	for i := range podList.Items {
		pod = &podList.Items[i]
	}
	if pod == nil {
		// TODO Should we print out a warning about not being able to find it?
		return nil
	}

	req := o.clientSet.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec")
	req.VersionedParams(&corev1.PodExecOptions{
		Container: "manager",
		Command:   []string{"/manager", "version"},
		Stdout:    true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(o.restConfig, "POST", req.URL())
	if err != nil {
		return err
	}

	var execOut bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{Stdout: &execOut, Tty: false})
	if err != nil {
		return err
	}

	_, err = o.Out.Write(execOut.Bytes())
	return err
}

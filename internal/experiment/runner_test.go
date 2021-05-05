package experiment

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	konjurev1beta2 "github.com/thestormforge/konjure/pkg/api/core/v1beta2"
	"github.com/thestormforge/konjure/pkg/konjure"
	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	//lint:ignore SA1019 backed out
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRunner(t *testing.T) {
	testCases := []struct {
		desc    string
		app     *redskyappsv1alpha1.Application
		err     error
		expName string
	}{
		{
			desc: "no parameters",
			app:  invalidExperimentNoParams,
			err:  errors.New("failed to generate experiment: invalid experiment, no parameters found"),
		},
		{
			desc: "no objectives",
			app:  invalidExperimentNoObjectives,
			err:  errors.New("failed to generate experiment: invalid experiment, no metrics found"),
		},
		{
			desc:    "success",
			app:     success,
			expName: "sampleapplication-a2987cd6",
		},
		{
			desc:    "confirmed",
			app:     confirmed,
			expName: "sampleapplication-a2987cd6",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {

			scheme := runtime.NewScheme()
			redskyv1beta1.AddToScheme(scheme)
			redskyappsv1alpha1.AddToScheme(scheme)

			client := fake.NewFakeClientWithScheme(scheme, &redskyv1beta1.Experiment{}, &redskyappsv1alpha1.Application{})

			appCh := make(chan *redskyappsv1alpha1.Application)
			runner, errCh := New(client, appCh)
			runner.kubectlExecFn = fakeKubectlExec

			ctx, cancel := context.WithCancel(context.Background())

			go func() { appCh <- tc.app }()
			go func() {
				runner.Run(ctx)
				cancel()
			}()

			for {
				select {
				case <-ctx.Done():
					if tc.err != nil {
						return
					}

					exp := &redskyv1beta1.Experiment{}
					err := client.Get(ctx, types.NamespacedName{Namespace: "default", Name: tc.expName}, exp)
					assert.NoError(t, err)

					// TODO
					// Need to figure out the right approach here
					// it's valid to have spec.replicas == 1 or spec.replicas == nil
					// how do we make our generation pipeline work for this
					assert.NotNil(t, exp)
					assert.NotNil(t, exp.Spec)
					assert.NotNil(t, exp.Spec.Replicas)

					if _, ok := tc.app.Annotations[redskyappsv1alpha1.AnnotationUserConfirmed]; ok {
						assert.Equal(t, int32(1), *exp.Spec.Replicas)
					} else {
						assert.Equal(t, int32(0), *exp.Spec.Replicas)
					}

					// fmt.Println(exp)
					return
				case err := <-errCh:
					if tc.err != nil {
						assert.Error(t, err)
						assert.Equal(t, tc.err.Error(), err.Error())
						cancel()

						continue
					}

					assert.NoError(t, err)
					cancel()
				}
			}

		})
	}
}

func fakeKubectlExec(cmd *exec.Cmd) ([]byte, error) {
	return []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  labels:
    app: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: 1
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
        resources:
          limits:
            memory: 25Mi
            cpu: 50m
          requests:
            memory: 25Mi
            cpu: 50m`), nil
}

var invalidExperimentNoParams = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
	},
}

var invalidExperimentNoObjectives = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
	},
	Resources: konjure.Resources{
		{
			Kubernetes: &konjurev1beta2.Kubernetes{
				Namespaces: []string{"default"},
				Selector:   "app=nginx",
			},
		},
	},
}

var success = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
	},
	Resources: konjure.Resources{
		{
			Kubernetes: &konjurev1beta2.Kubernetes{
				Namespaces: []string{"default"},
				Selector:   "app=nginx",
			},
		},
	},
	Objectives: []redskyappsv1alpha1.Objective{
		{
			Goals: []redskyappsv1alpha1.Goal{
				{
					Duration: &redskyappsv1alpha1.DurationGoal{
						DurationType: "trial",
					},
				},
			},
		},
	},
	Scenarios: []redskyappsv1alpha1.Scenario{
		{
			Name: "how-do-you-make-a-tissue-dance",
			StormForger: &redskyappsv1alpha1.StormForgerScenario{
				TestCase: "put-a-little-boogie-in-it",
			},
		},
	},
	StormForger: &redskyappsv1alpha1.StormForger{
		Organization: "sf",
		AccessToken: &redskyappsv1alpha1.StormForgerAccessToken{
			Literal: "Why couldn't the bicycle stand up by itself? It was two tired!",
		},
	},
}

var confirmed = &redskyappsv1alpha1.Application{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "sampleApplication",
		Namespace: "default",
		Annotations: map[string]string{
			redskyappsv1alpha1.AnnotationUserConfirmed: "true",
		},
	},
	Resources: konjure.Resources{
		{
			Kubernetes: &konjurev1beta2.Kubernetes{
				Namespaces: []string{"default"},
				Selector:   "app=nginx",
			},
		},
	},
	Objectives: []redskyappsv1alpha1.Objective{
		{
			Goals: []redskyappsv1alpha1.Goal{
				{
					Duration: &redskyappsv1alpha1.DurationGoal{
						DurationType: "trial",
					},
				},
			},
		},
	},
	Scenarios: []redskyappsv1alpha1.Scenario{
		{
			Name: "how-do-you-make-a-tissue-dance",
			StormForger: &redskyappsv1alpha1.StormForgerScenario{
				TestCase: "put-a-little-boogie-in-it",
			},
		},
	},
	StormForger: &redskyappsv1alpha1.StormForger{
		Organization: "sf",
		AccessToken: &redskyappsv1alpha1.StormForgerAccessToken{
			Literal: "Why couldn't the bicycle stand up by itself? It was two tired!",
		},
	},
}

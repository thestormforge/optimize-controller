package experiment

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	//lint:ignore SA1019 backed out
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	redskyappsv1alpha1 "github.com/thestormforge/optimize-controller/api/apps/v1alpha1"
	redskyv1beta1 "github.com/thestormforge/optimize-controller/api/v1beta1"
)

func Test(t *testing.T) {
	testCases := []struct {
		desc string
		app  *redskyappsv1alpha1.Application
	}{
		{
			desc: "default",
			app: &redskyappsv1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "sampleApplication",
					Namespace: "default",
				},
				/*
					Resources: konjure.Resource{
						Kubernetes: &konjurev1beta2.Kubernetes{},
					},
				*/
			},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {

			scheme := runtime.NewScheme()
			scheme.AddKnownTypes(redskyv1beta1.GroupVersion, &redskyv1beta1.Experiment{})
			scheme.AddKnownTypes(redskyappsv1alpha1.GroupVersion, &redskyappsv1alpha1.Application{})
			// customListKinds := map[schema.GroupVersionResource]string{
			// 	redskyv1beta1.GroupVersion.WithResource("experiments"): "ExperimentList",
			// }
			// client := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, customListKinds)
			// client := fake.NewSimpleDynamicClient(scheme, &redskyv1beta1.Experiment{}, &redskyappsv1alpha1.Application{})
			client := fake.NewFakeClientWithScheme(scheme, &redskyv1beta1.Experiment{}, &redskyappsv1alpha1.Application{})

			ctx, _ := context.WithCancel(context.Background())
			appCh := make(chan *redskyappsv1alpha1.Application)

			go Run(ctx, client, appCh)

			appCh <- tc.app

			var err error
			assert.NoError(t, err)
		})
	}
}

/*
  ObjectMeta: metav1.ObjectMeta{
    Name:      "sampleApplication",
    Namespace: "default",
  },
  Resources: konjure.Resources{konjure.NewResource(filename)},
  Parameters: &app.Parameters{
    ContainerResources: &app.ContainerResources{
      LabelSelector: "component=postgres",
    },
  },
  Scenarios: []app.Scenario{
    {
      Name: "how-do-you-make-a-tissue-dance",
      StormForger: &app.StormForgerScenario{
        TestCase: "tissue-box",
      },
    },
    {
      Name: "put-a-little-boogie-in-it",
      StormForger: &app.StormForgerScenario{
        TestCase: "boogie",
      },
    },
  },
  Objectives: []app.Objective{
    {
      Goals: []app.Goal{
        {
          Name: "cost",
          Max:  resource.NewQuantity(100, resource.DecimalExponent),
          Requests: &app.RequestsGoal{
            MetricSelector: "everybody=yes",
            Weights: corev1.ResourceList{
              corev1.ResourceCPU:    resource.MustParse("100m"),
              corev1.ResourceMemory: resource.MustParse("100M"),
            },
          },
        },
      },
    },
*/

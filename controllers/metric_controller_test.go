package controllers

import (
	"context"
	"path/filepath"
	"testing"

	redskyv1alpha1 "github.com/redskyops/redskyops-controller/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestMetricController(t *testing.T) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	defer testEnv.Stop()

	rtscheme := scheme.Scheme
	err = scheme.AddToScheme(rtscheme)
	require.NoError(t, err)
	err = redskyv1alpha1.AddToScheme(rtscheme)
	require.NoError(t, err)

	var k8sClient client.Client
	k8sClient, err = client.New(cfg, client.Options{Scheme: rtscheme})
	require.NoError(t, err)
	require.NotNil(t, k8sClient)

	// Verify CRD was added
	for _, obj := range []runtime.Object{&redskyv1alpha1.ExperimentList{}, &redskyv1alpha1.TrialList{}} {
		err = k8sClient.List(context.Background(), obj)
		require.NoError(t, err)
		require.NotNil(t, obj)
	}

	tmr := &TestMetricReconciler{
		MetricReconciler: MetricReconciler{
			Client: k8sClient,
			Scheme: rtscheme,
			Log:    ctrl.Log,
		},
	}

	testCases := map[string]func(t *testing.T){"collectMetrics": tmr.tester}

	for name, testCase := range testCases {
		t.Run(name, testCase)
	}

}

type TestMetricReconciler struct {
	MetricReconciler
}

func (r *TestMetricReconciler) tester(t *testing.T) {

	cases := []struct {
		desc          string
		metrics       []redskyv1alpha1.Metric
		values        []redskyv1alpha1.Value
		condition     map[redskyv1alpha1.TrialConditionType]corev1.ConditionStatus
		expectedError bool
	}{
		{
			desc: "single local metric no attempts left",
			metrics: []redskyv1alpha1.Metric{
				{
					Name:  "testMetric",
					Query: "{{duration .StartTime .CompletionTime}}",
					Type:  redskyv1alpha1.MetricLocal,
				},
			},
			values: []redskyv1alpha1.Value{
				{
					Name:              "testMetric",
					Value:             "123",
					AttemptsRemaining: 0,
				},
			},
			condition:     map[redskyv1alpha1.TrialConditionType]corev1.ConditionStatus{redskyv1alpha1.TrialObserved: corev1.ConditionTrue},
			expectedError: false,
		},
		{
			desc: "single local metric more attempts",
			metrics: []redskyv1alpha1.Metric{
				{
					Name:  "testMetric",
					Query: "{{duration .StartTime .CompletionTime}}",
					Type:  redskyv1alpha1.MetricLocal,
				},
			},
			values: []redskyv1alpha1.Value{
				{
					Name:              "testMetric",
					Value:             "123",
					AttemptsRemaining: 1,
				},
			},
			condition:     map[redskyv1alpha1.TrialConditionType]corev1.ConditionStatus{redskyv1alpha1.TrialObserved: corev1.ConditionFalse},
			expectedError: false,
		},
		{
			desc: "single pods metric more attempts no selector",
			metrics: []redskyv1alpha1.Metric{
				{
					Name:  "testMetric",
					Query: "{{duration .StartTime .CompletionTime}}",
					Type:  redskyv1alpha1.MetricPods,
				},
			},
			values: []redskyv1alpha1.Value{
				{
					Name:              "testMetric",
					Value:             "123",
					AttemptsRemaining: 1,
				},
			},
			condition:     map[redskyv1alpha1.TrialConditionType]corev1.ConditionStatus{redskyv1alpha1.TrialObserved: corev1.ConditionFalse},
			expectedError: false,
		},
		{
			desc: "single pods metric more attempts with selector",
			metrics: []redskyv1alpha1.Metric{
				{
					Name:  "testMetric",
					Query: "{{duration .StartTime .CompletionTime}}",
					Type:  redskyv1alpha1.MetricPods,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"fake": "fake"},
					},
				},
			},
			values: []redskyv1alpha1.Value{
				{
					Name:              "testMetric",
					Value:             "123",
					AttemptsRemaining: 1,
				},
			},
			condition:     map[redskyv1alpha1.TrialConditionType]corev1.ConditionStatus{redskyv1alpha1.TrialObserved: corev1.ConditionFalse},
			expectedError: false,
		},
		{
			desc: "no query",
			metrics: []redskyv1alpha1.Metric{
				{
					Name: "testMetric",
					Type: redskyv1alpha1.MetricPods,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"fake": "fake"},
					},
				},
			},
			values: []redskyv1alpha1.Value{
				{
					Name:              "testMetric",
					Value:             "123",
					AttemptsRemaining: 1,
				},
			},
			condition:     map[redskyv1alpha1.TrialConditionType]corev1.ConditionStatus{redskyv1alpha1.TrialFailed: corev1.ConditionTrue},
			expectedError: false,
		},
	}

	var err error
	for _, testCase := range cases {
		t.Run(testCase.desc, func(t *testing.T) {
			exp, tri, pod := r.createExperimentAndTrialAndPod(testCase.metrics, testCase.values)

			resources := []runtime.Object{exp, tri, pod}

			for _, obj := range resources {
				err = r.Create(context.Background(), obj)
				assert.NoError(t, err)
			}

			now := metav1.Now()
			_, err = r.collectMetrics(context.Background(), tri, &now)
			if testCase.expectedError {
				assert.Error(t, err)
				r.cleanup(t, exp, tri)
			} else {
				assert.NoError(t, err)
			}

			for expectedCondition, expectedConditionStatus := range testCase.condition {
				for _, condition := range tri.Status.Conditions {
					if condition.Type != expectedCondition {
						continue
					}
					assert.Equal(t, expectedConditionStatus, condition.Status)
				}
			}

			r.cleanup(t, resources...)
		})
	}
}

func (r *TestMetricReconciler) createExperimentAndTrialAndPod(metrics []redskyv1alpha1.Metric, values []redskyv1alpha1.Value) (*redskyv1alpha1.Experiment, *redskyv1alpha1.Trial, *corev1.Pod) {
	// Create Experiment
	exp := &redskyv1alpha1.Experiment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "myexperiment",
		},
		Spec: redskyv1alpha1.ExperimentSpec{
			Metrics: metrics,
		},
	}

	// Create Trial
	tri := &redskyv1alpha1.Trial{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "myexperiment",
		},
		Spec: redskyv1alpha1.TrialSpec{
			Values: values,
		},
	}

	labels := make(map[string]string)
	for _, metric := range metrics {
		if metric.Selector == nil {
			continue
		}

		for k, v := range metric.Selector.MatchLabels {
			labels[k] = v
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels:    labels,
			Namespace: "default",
			Name:      "mypod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "web",
					Image: "nginx:1.12",
				},
			},
		},
	}

	return exp, tri, pod
}

func (r *TestMetricReconciler) cleanup(t *testing.T, objs ...runtime.Object) {
	for _, obj := range objs {
		err := r.Delete(context.Background(), obj)
		assert.NoError(t, err)
	}
}

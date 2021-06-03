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

package controller

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// ReconcileConflictErrors is a Prometheus counter metric which holds the total
	// number of conflict errors from the Reconciler
	ReconcileConflictErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "controller_runtime_reconcile_conflict_errors_total",
		Help: "Total number of reconciliation conflict errors per controller",
	}, []string{"controller"})

	// TODO Experiment is an unbounded label, that might be problematic

	// ExperimentTrials is a Prometheus gauge metric which holds the total number
	// of trials for an experiment (trial counts can go down when they are cleaned up)
	ExperimentTrials = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "optimize_experiment_trials_total",
		Help: "Total number of trials present for an experiment",
	}, []string{"experiment"})

	// ExperimentActiveTrials is a Prometheus gauge metric which holds the total number
	// of active trials for an experiment
	ExperimentActiveTrials = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "optimize_experiment_active_trials_total",
		Help: "Total number of active trials present for an experiment",
	}, []string{"experiment"})
)

func init() {
	metrics.Registry.MustRegister(
		ReconcileConflictErrors,
		ExperimentTrials,
		ExperimentActiveTrials,
	)
}

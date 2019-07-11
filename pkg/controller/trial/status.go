package trial

import (
	redskyv1alpha1 "github.com/gramLabs/redsky/pkg/apis/redsky/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Checks to see if the specified trial is finished
func IsTrialFinished(trial *redskyv1alpha1.Trial) bool {
	for _, c := range trial.Status.Conditions {
		if (c.Type == redskyv1alpha1.TrialComplete || c.Type == redskyv1alpha1.TrialFailed) && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// Updates a the status of an existing condition or adds it if it does not exist
func applyCondition(status *redskyv1alpha1.TrialStatus, conditionType redskyv1alpha1.TrialConditionType, conditionStatus corev1.ConditionStatus, reason, message string, time *metav1.Time) {
	if time == nil {
		now := metav1.Now()
		time = &now
	}

	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			if status.Conditions[i].Status != conditionStatus {
				status.Conditions[i].Status = conditionStatus
				status.Conditions[i].Reason = reason
				status.Conditions[i].Message = message
				status.Conditions[i].LastTransitionTime = *time
			} else {
				status.Conditions[i].LastProbeTime = *time
			}
			return
		}
	}

	status.Conditions = append(status.Conditions, redskyv1alpha1.TrialCondition{
		Type:               conditionType,
		Status:             conditionStatus,
		Reason:             reason,
		Message:            message,
		LastProbeTime:      *time,
		LastTransitionTime: *time,
	})
}

// Checks to see if a condition has a specific status and if it exists
func checkCondition(status *redskyv1alpha1.TrialStatus, conditionType redskyv1alpha1.TrialConditionType, conditionStatus corev1.ConditionStatus) (bool, bool) {
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return status.Conditions[i].Status == conditionStatus, true
		}
	}
	return false, false
}

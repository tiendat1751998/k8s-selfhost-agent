package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RevisionAnnotation is the standard Kubernetes annotation storing deployment/replicaset revision.
const RevisionAnnotation = "deployment.kubernetes.io/revision"

// RollbackDeployment rolls back a Kubernetes Deployment to its previous ReplicaSet revision (N-1).
// It copies the PodTemplateSpec from the previous revision ReplicaSet into the Deployment.
func RollbackDeployment(ctx context.Context, client kubernetes.Interface, namespace, deploymentName string) (string, error) {
	if client == nil {
		return "", fmt.Errorf("kubernetes client is required for rollback")
	}
	if namespace == "" {
		namespace = "default"
	}

	deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("getting deployment %s for rollback: %w", deploymentName, err)
	}

	rsList, err := client.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("listing replicasets for deployment %s: %w", deploymentName, err)
	}

	// Filter ReplicaSets that belong to this Deployment
	var matchingRS []appsv1.ReplicaSet
	for _, rs := range rsList.Items {
		owned := false
		for _, ref := range rs.OwnerReferences {
			if ref.Kind == "Deployment" && ref.Name == deploy.Name {
				owned = true
				break
			}
		}
		if !owned && deploy.Spec.Selector != nil && len(deploy.Spec.Selector.MatchLabels) > 0 {
			match := true
			for k, v := range deploy.Spec.Selector.MatchLabels {
				if rs.Labels[k] != v {
					match = false
					break
				}
			}
			if match {
				owned = true
			}
		}
		if !owned && strings.HasPrefix(rs.Name, deploy.Name+"-") {
			owned = true
		}
		if owned {
			matchingRS = append(matchingRS, rs)
		}
	}

	if len(matchingRS) == 0 {
		return "", fmt.Errorf("Cannot rollback deployment '%s': No previous revision found in ReplicaSet history", deploymentName)
	}

	// Discover current revision from deployment annotation or max RS revision
	var currentRev int64
	if revStr, ok := deploy.Annotations[RevisionAnnotation]; ok {
		currentRev, _ = strconv.ParseInt(revStr, 10, 64)
	}

	if currentRev == 0 {
		for _, rs := range matchingRS {
			if revStr, ok := rs.Annotations[RevisionAnnotation]; ok {
				if rev, err := strconv.ParseInt(revStr, 10, 64); err == nil && rev > currentRev {
					currentRev = rev
				}
			}
		}
	}

	// Find previous revision (highest revision strictly less than currentRev)
	var prevRS *appsv1.ReplicaSet
	var prevRev int64 = -1

	for i := range matchingRS {
		rs := &matchingRS[i]
		if revStr, ok := rs.Annotations[RevisionAnnotation]; ok {
			rev, err := strconv.ParseInt(revStr, 10, 64)
			if err == nil && rev < currentRev && rev > prevRev {
				prevRev = rev
				prevRS = rs
			}
		}
	}

	if prevRS == nil || prevRev < 0 {
		return "", fmt.Errorf("Cannot rollback deployment '%s': No previous revision found in ReplicaSet history", deploymentName)
	}

	// Rollback deployment pod template spec to previous revision
	deploy.Spec.Template = prevRS.Spec.Template
	if deploy.Annotations == nil {
		deploy.Annotations = make(map[string]string)
	}
	deploy.Annotations[RevisionAnnotation] = strconv.FormatInt(prevRev, 10)

	_, err = client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("updating deployment %s to revision %d: %w", deploymentName, prevRev, err)
	}

	return strconv.FormatInt(prevRev, 10), nil
}

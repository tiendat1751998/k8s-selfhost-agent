package kubernetes

import (
	"context"
	cryptorand "crypto/rand"
	"fmt"
	"math/big"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ScaleDeployment scales a Deployment to the specified number of replicas.
func (r *ResourceRepo) ScaleDeployment(ctx context.Context, clusterID, namespace, name string, replicas int32) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	if namespace == "" {
		namespace = r.discoverNamespace(ctx, client, "deployments", name)
	}

	deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s: %w", name, err)
	}

	deploy.Spec.Replicas = &replicas
	if _, err := client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating scale for deployment %s: %w", name, err)
	}
	return nil
}

// RestartDeployment triggers a rolling restart of a Deployment by updating its restartedAt annotation.
func (r *ResourceRepo) RestartDeployment(ctx context.Context, clusterID, namespace, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	if namespace == "" {
		namespace = r.discoverNamespace(ctx, client, "deployments", name)
	}

	deploy, err := client.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting deployment %s: %w", name, err)
	}

	if deploy.Spec.Template.Annotations == nil {
		deploy.Spec.Template.Annotations = make(map[string]string)
	}
	deploy.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)

	if _, err := client.AppsV1().Deployments(namespace).Update(ctx, deploy, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("restarting deployment %s: %w", name, err)
	}
	return nil
}

// ScaleStatefulSet scales a StatefulSet to the specified number of replicas.
func (r *ResourceRepo) ScaleStatefulSet(ctx context.Context, clusterID, namespace, name string, replicas int32) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	if namespace == "" {
		namespace = r.discoverNamespace(ctx, client, "statefulsets", name)
	}

	sts, err := client.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting statefulset %s: %w", name, err)
	}

	sts.Spec.Replicas = &replicas
	if _, err := client.AppsV1().StatefulSets(namespace).Update(ctx, sts, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating scale for statefulset %s: %w", name, err)
	}
	return nil
}

// RestartDaemonSet triggers a rolling restart of a DaemonSet by updating its restartedAt annotation.
func (r *ResourceRepo) RestartDaemonSet(ctx context.Context, clusterID, namespace, name string) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	if namespace == "" {
		namespace = r.discoverNamespace(ctx, client, "daemonsets", name)
	}

	ds, err := client.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting daemonset %s: %w", name, err)
	}

	if ds.Spec.Template.Annotations == nil {
		ds.Spec.Template.Annotations = make(map[string]string)
	}
	ds.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().UTC().Format(time.RFC3339)

	if _, err := client.AppsV1().DaemonSets(namespace).Update(ctx, ds, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("restarting ddemonset %s: %w", name, err)
	}
	return nil
}

func randomSuffix(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		num, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return fmt.Sprintf("%d", time.Now().UnixNano()%100000)
		}
		b[i] = charset[num.Int64()]
	}
	return string(b)
}

// TriggerCronJob triggers a manual run of a CronJob by creating a Job from its JobTemplate.
func (r *ResourceRepo) TriggerCronJob(ctx context.Context, clusterID, namespace, name string) (*batchv1.Job, error) {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace = r.discoverNamespace(ctx, client, "cronjobs", name)
	}

	cronJob, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting cronjob %s: %w", name, err)
	}

	randSuffix := randomSuffix(5)
	jobName := fmt.Sprintf("%s-manual-%s", name, randSuffix)
	if len(jobName) > 63 {
		maxBase := 63 - len(fmt.Sprintf("-manual-%s", randSuffix))
		if maxBase > 0 && len(name) > maxBase {
			jobName = fmt.Sprintf("%s-manual-%s", name[:maxBase], randSuffix)
		}
	}

	jobMeta := metav1.ObjectMeta{
		Name:        jobName,
		Namespace:   namespace,
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
	}
	for k, v := range cronJob.Spec.JobTemplate.ObjectMeta.Labels {
		jobMeta.Labels[k] = v
	}
	for k, v := range cronJob.Spec.JobTemplate.ObjectMeta.Annotations {
		jobMeta.Annotations[k] = v
	}
	jobMeta.Annotations["cronjob.kubernetes.io/instantiate"] = "manual"

	job := &batchv1.Job{
		ObjectMeta: jobMeta,
		Spec:       cronJob.Spec.JobTemplate.Spec,
	}

	createdJob, err := client.BatchV1().Jobs(namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("creating manual job for cronjob %s: %w", name, err)
	}
	return createdJob, nil
}

// SuspendCronJob suspends or resumes a CronJob.
func (r *ResourceRepo) SuspendCronJob(ctx context.Context, clusterID, namespace, name string, suspend bool) error {
	client, err := r.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	if namespace == "" {
		namespace = r.discoverNamespace(ctx, client, "cronjobs", name)
	}

	cronJob, err := client.BatchV1().CronJobs(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting cronjob %s: %w", name, err)
	}

	cronJob.Spec.Suspend = &suspend
	if _, err := client.BatchV1().CronJobs(namespace).Update(ctx, cronJob, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating suspend for cronjob %s: %w", name, err)
	}
	return nil
}

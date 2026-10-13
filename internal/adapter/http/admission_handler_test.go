package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	usecasesec "github.com/datdt/k8sselfhost/internal/usecase/security"
)

func createTestAdmissionReview(namespace, name string, pod *corev1.Pod) *admissionv1.AdmissionReview {
	raw, _ := json.Marshal(pod)
	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-webhook-uid-9999",
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Name:      name,
			Namespace: namespace,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

func createCompliantTestPod() *corev1.Pod {
	readOnly := true
	privileged := false
	runAsUser := int64(1000)
	runAsNonRoot := true

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "compliant-workload",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &runAsUser,
			},
			Containers: []corev1.Container{
				{
					Name:  "web",
					Image: "registry.internal/apps/web:v1.0.0@sha256:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: &readOnly,
						Privileged:             &privileged,
					},
				},
			},
		},
	}
}

func TestAdmissionHandler_DockerSockMount_Rejected(t *testing.T) {
	enforcer := usecasesec.NewAdmissionEnforcer()
	handler := NewAdmissionHandler(enforcer)

	pod := createCompliantTestPod()
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "docker-socket",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/run/docker.sock",
			},
		},
	})
	review := createTestAdmissionReview("default", "dind-pod", pod)

	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/admission/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ValidateAdmission(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp admissionv1.AdmissionReview
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Response)

	assert.False(t, resp.Response.Allowed)
	assert.Equal(t, int32(403), resp.Response.Result.Code)
	assert.Contains(t, resp.Response.Result.Message, "docker.sock")
}

func TestAdmissionHandler_LatestTag_Rejected(t *testing.T) {
	enforcer := usecasesec.NewAdmissionEnforcer()
	handler := NewAdmissionHandler(enforcer)

	pod := createCompliantTestPod()
	pod.Spec.Containers[0].Image = "nginx:latest"
	review := createTestAdmissionReview("default", "latest-tag-pod", pod)

	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/admission/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ValidateAdmission(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp admissionv1.AdmissionReview
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Response)

	assert.False(t, resp.Response.Allowed)
	assert.Equal(t, int32(422), resp.Response.Result.Code)
	assert.NotEmpty(t, resp.Response.Warnings)
}

func TestAdmissionHandler_Privileged_Rejected(t *testing.T) {
	enforcer := usecasesec.NewAdmissionEnforcer()
	handler := NewAdmissionHandler(enforcer)

	pod := createCompliantTestPod()
	isPrivileged := true
	pod.Spec.Containers[0].SecurityContext.Privileged = &isPrivileged
	review := createTestAdmissionReview("default", "privileged-pod", pod)

	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/admission/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ValidateAdmission(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp admissionv1.AdmissionReview
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Response)

	assert.False(t, resp.Response.Allowed)
	assert.Equal(t, int32(422), resp.Response.Result.Code)
}

func TestAdmissionHandler_CompliantPod_Allowed(t *testing.T) {
	enforcer := usecasesec.NewAdmissionEnforcer()
	handler := NewAdmissionHandler(enforcer)

	pod := createCompliantTestPod()
	review := createTestAdmissionReview("default", "compliant-pod", pod)

	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/admission/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ValidateAdmission(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp admissionv1.AdmissionReview
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Response)

	assert.True(t, resp.Response.Allowed)
	assert.Equal(t, int32(200), resp.Response.Result.Code)
}

func TestAdmissionHandler_KubeSystem_Exempt(t *testing.T) {
	enforcer := usecasesec.NewAdmissionEnforcer()
	handler := NewAdmissionHandler(enforcer)

	pod := createCompliantTestPod()
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "docker-sock",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/run/docker.sock",
			},
		},
	})
	review := createTestAdmissionReview("kube-system", "kube-system-pod", pod)

	body, err := json.Marshal(review)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/admission/validate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ValidateAdmission(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp admissionv1.AdmissionReview
	err = json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.NotNil(t, resp.Response)

	assert.True(t, resp.Response.Allowed)
	assert.Equal(t, int32(200), resp.Response.Result.Code)
}

func TestAdmissionHandler_MalformedJSON_RFC7807(t *testing.T) {
	enforcer := usecasesec.NewAdmissionEnforcer()
	handler := NewAdmissionHandler(enforcer)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/security/admission/validate", bytes.NewReader([]byte("{invalid-json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ValidateAdmission(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/problem+json", w.Header().Get("Content-Type"))

	var problem ProblemDetails
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, 400, problem.Status)
	assert.Equal(t, "VALIDATION_ERROR", problem.Code)
}

package http

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
)

// K8sLogsHandler provides HTTP handlers for retrieving and streaming Kubernetes pod logs.
type K8sLogsHandler struct {
	defaultClient kubernetes.Interface
	clientManager *cluster.ClientManager
}

// NewK8sLogsHandler creates a new K8sLogsHandler.
func NewK8sLogsHandler(defaultClient kubernetes.Interface, cm *cluster.ClientManager) *K8sLogsHandler {
	return &K8sLogsHandler{
		defaultClient: defaultClient,
		clientManager: cm,
	}
}

func (h *K8sLogsHandler) getK8sClient(ctx context.Context, clusterID string) (kubernetes.Interface, error) {
	var client kubernetes.Interface
	if h.clientManager != nil && clusterID != "" && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		cli, err := h.clientManager.GetK8sClient(ctx, clusterID)
		if err == nil && cli != nil {
			client = cli
		}
	}

	if client == nil {
		client = h.defaultClient
	}

	if client == nil {
		return nil, infraK8s.ErrK8sUnavailable
	}
	return client, nil
}

// HandlePodLogs handles retrieving pod logs (JSON when follow=false, SSE when follow=true).
func (h *K8sLogsHandler) HandlePodLogs(w http.ResponseWriter, r *http.Request) {
	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		cluster = r.URL.Query().Get("cluster")
	}

	pod := chi.URLParam(r, "pod")
	if pod == "" {
		pod = r.URL.Query().Get("pod")
	}
	if strings.TrimSpace(pod) == "" {
		writeError(w, http.StatusBadRequest, "pod is required", nil)
		return
	}

	namespace := getNamespaceQuery(r)
	if namespace == "" {
		namespace = "default"
	}

	container := strings.TrimSpace(r.URL.Query().Get("container"))
	follow := r.URL.Query().Get("follow") == "true" || r.URL.Query().Get("follow") == "1"
	previous := r.URL.Query().Get("previous") == "true" || r.URL.Query().Get("previous") == "1"

	var tailLines int64 = 100
	tailParam := r.URL.Query().Get("tailLines")
	if tailParam == "" {
		tailParam = r.URL.Query().Get("tail")
	}
	if tailParam != "" {
		if parsed, err := strconv.ParseInt(tailParam, 10, 64); err == nil && parsed > 0 {
			tailLines = parsed
		}
	}

	client, err := h.getK8sClient(r.Context(), cluster)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get kubernetes client", err)
		return
	}

	opts := &corev1.PodLogOptions{
		Follow:    follow,
		Previous:  previous,
		TailLines: &tailLines,
	}
	if container != "" {
		opts.Container = container
	}

	req := client.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(r.Context())
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to stream logs", err)
		return
	}
	defer stream.Close()

	if !follow {
		logsBytes, err := io.ReadAll(stream)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read logs", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": map[string]string{
				"logs": string(logsBytes),
			},
		})
		return
	}

	// Follow=true -> Server-Sent Events (SSE) stream
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	scanner := bufio.NewScanner(stream)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		line := scanner.Text()
		if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
}

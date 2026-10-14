package http

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HandleWorkloadLogs retrieves and aggregates logs for a Kubernetes workload (Deployment, StatefulSet, DaemonSet).
func (h *K8sLogsHandler) HandleWorkloadLogs(w http.ResponseWriter, r *http.Request) {
	cluster := chi.URLParam(r, "cluster")
	if cluster == "" {
		cluster = r.URL.Query().Get("cluster")
	}

	kind := chi.URLParam(r, "kind")
	if kind == "" {
		kind = r.URL.Query().Get("kind")
	}
	if strings.TrimSpace(kind) == "" {
		kind = "deployments"
	}
	k := strings.ToLower(strings.TrimSpace(kind))

	name := chi.URLParam(r, "name")
	if name == "" {
		name = r.URL.Query().Get("name")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name is required", nil)
		return
	}

	namespace := getNamespaceQuery(r)
	if strings.TrimSpace(namespace) == "" {
		writeError(w, http.StatusBadRequest, "namespace is required", nil)
		return
	}

	if err := checkNamespaceAccess(r, namespace); err != nil {
		writeError(w, http.StatusForbidden, err.Error(), nil)
		return
	}

	pod := strings.TrimSpace(r.URL.Query().Get("pod"))
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
	if tailLines > 1000 {
		tailLines = 1000
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

	// If pod is explicitly specified, stream/read logs for that pod only
	if pod != "" {
		opts := &corev1.PodLogOptions{
			Follow:     follow,
			Previous:   previous,
			TailLines:  &tailLines,
			Timestamps: true,
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
			if k8serrors.IsNotFound(err) {
				writeError(w, http.StatusNotFound, "pod not found", err)
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
			logsStr := h.redactLogString(string(logsBytes))
			totalLines := 0
			if len(logsStr) > 0 {
				totalLines = len(strings.Split(strings.TrimRight(logsStr, "\r\n"), "\n"))
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"workload":    name,
				"namespace":   namespace,
				"pods":        []string{pod},
				"logs":        logsStr,
				"total_lines": totalLines,
				"data": map[string]interface{}{
					"workload":    name,
					"namespace":   namespace,
					"pods":        []string{pod},
					"logs":        logsStr,
					"total_lines": totalLines,
				},
			})
			return
		}

		h.streamSinglePodLogs(w, r, stream)
		return
	}

	// Resolve workload selector
	var labelSelector *metav1.LabelSelector
	switch k {
	case "deployments", "deployment", "deploy":
		dep, err := client.AppsV1().Deployments(namespace).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			if isK8sUnavailable(err) {
				writeK8sUnavailable(w)
				return
			}
			if k8serrors.IsNotFound(err) {
				writeError(w, http.StatusNotFound, "deployment not found", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to get deployment", err)
			return
		}
		labelSelector = dep.Spec.Selector
	case "statefulsets", "statefulset", "sts":
		sts, err := client.AppsV1().StatefulSets(namespace).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			if isK8sUnavailable(err) {
				writeK8sUnavailable(w)
				return
			}
			if k8serrors.IsNotFound(err) {
				writeError(w, http.StatusNotFound, "statefulset not found", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to get statefulset", err)
			return
		}
		labelSelector = sts.Spec.Selector
	case "daemonsets", "daemonset", "ds":
		ds, err := client.AppsV1().DaemonSets(namespace).Get(r.Context(), name, metav1.GetOptions{})
		if err != nil {
			if isK8sUnavailable(err) {
				writeK8sUnavailable(w)
				return
			}
			if k8serrors.IsNotFound(err) {
				writeError(w, http.StatusNotFound, "daemonset not found", err)
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to get daemonset", err)
			return
		}
		labelSelector = ds.Spec.Selector
	default:
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported workload kind: %s", kind), nil)
		return
	}

	var selectorStr string
	if labelSelector != nil {
		if sel, err := metav1.LabelSelectorAsSelector(labelSelector); err == nil {
			selectorStr = sel.String()
		}
	}

	podList, err := client.CoreV1().Pods(namespace).List(r.Context(), metav1.ListOptions{LabelSelector: selectorStr})
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list pods for workload", err)
		return
	}

	activePods := make([]corev1.Pod, 0, len(podList.Items))
	podNames := make([]string, 0, len(podList.Items))
	for _, p := range podList.Items {
		if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		activePods = append(activePods, p)
		podNames = append(podNames, p.Name)
	}

	if len(activePods) == 0 {
		if !follow {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"workload":    name,
				"namespace":   namespace,
				"pods":        []string{},
				"logs":        "",
				"total_lines": 0,
				"data": map[string]interface{}{
					"workload":    name,
					"namespace":   namespace,
					"pods":        []string{},
					"logs":        "",
					"total_lines": 0,
				},
			})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return
	}

	if !follow {
		opts := &corev1.PodLogOptions{
			Follow:     false,
			Previous:   previous,
			TailLines:  &tailLines,
			Timestamps: true,
		}
		if container != "" {
			opts.Container = container
		}

		var aggregatedLines []string
		bufPtr := logScanBufPool.Get().(*[]byte)
		defer logScanBufPool.Put(bufPtr)
		for _, p := range activePods {
			req := client.CoreV1().Pods(namespace).GetLogs(p.Name, opts)
			stream, err := req.Stream(r.Context())
			if err != nil {
				continue
			}
			sc := bufio.NewScanner(stream)
			sc.Buffer(*bufPtr, 1024*1024)
			for sc.Scan() {
				text := sc.Text()
				if strings.TrimSpace(text) == "" {
					continue
				}
				text = h.redactLogString(text)
				aggregatedLines = append(aggregatedLines, fmt.Sprintf("[%s] %s", p.Name, text))
			}
			_ = stream.Close()
		}

		logsStr := strings.Join(aggregatedLines, "\n")
		totalLines := len(aggregatedLines)
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"workload":    name,
			"namespace":   namespace,
			"pods":        podNames,
			"logs":        logsStr,
			"total_lines": totalLines,
			"data": map[string]interface{}{
				"workload":    name,
				"namespace":   namespace,
				"pods":        podNames,
				"logs":        logsStr,
				"total_lines": totalLines,
			},
		})
		return
	}

	// SSE Follow stream aggregated across multiple pods
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, _ := w.(http.Flusher)
	if flusher != nil {
		flusher.Flush()
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	type podLine struct {
		pod  string
		text string
	}
	linesChan := make(chan podLine, 100)
	var wg sync.WaitGroup

	followOpts := &corev1.PodLogOptions{
		Follow:     true,
		Previous:   previous,
		TailLines:  &tailLines,
		Timestamps: true,
	}
	if container != "" {
		followOpts.Container = container
	}

	for _, p := range activePods {
		wg.Add(1)
		go func(podName string) {
			defer wg.Done()
			req := client.CoreV1().Pods(namespace).GetLogs(podName, followOpts)
			stream, err := req.Stream(ctx)
			if err != nil {
				return
			}
			defer stream.Close()

			streamDone := make(chan struct{})
			defer close(streamDone)
			go func() {
				select {
				case <-ctx.Done():
					_ = stream.Close()
				case <-streamDone:
				}
			}()

			scanner := bufio.NewScanner(stream)
			bufPtr := logScanBufPool.Get().(*[]byte)
			defer logScanBufPool.Put(bufPtr)
			scanner.Buffer(*bufPtr, 1024*1024)

			for scanner.Scan() {
				select {
				case <-ctx.Done():
					return
				case linesChan <- podLine{pod: podName, text: h.redactLogString(scanner.Text())}:
				}
			}
		}(p.Name)
	}

	go func() {
		wg.Wait()
		close(linesChan)
	}()

	for {
		select {
		case <-r.Context().Done():
			return
		case item, ok := <-linesChan:
			if !ok {
				return
			}
			line := fmt.Sprintf("[%s] %s", item.pod, item.text)
			if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}

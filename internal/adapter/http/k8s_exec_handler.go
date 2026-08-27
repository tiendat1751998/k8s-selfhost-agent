package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// K8sExecHandler provides WebSocket HTTP handlers for interactive terminal exec into Kubernetes pods.
type K8sExecHandler struct {
	defaultClient kubernetes.Interface
	clientManager *cluster.ClientManager
}

// NewK8sExecHandler creates a new K8sExecHandler.
func NewK8sExecHandler(defaultClient kubernetes.Interface, cm *cluster.ClientManager) *K8sExecHandler {
	return &K8sExecHandler{
		defaultClient: defaultClient,
		clientManager: cm,
	}
}

func (h *K8sExecHandler) getK8sClientAndConfig(ctx context.Context, clusterID string) (kubernetes.Interface, *rest.Config, error) {
	var client kubernetes.Interface
	var cfg *rest.Config

	if h.clientManager != nil && clusterID != "" && clusterID != "local" && clusterID != "default" && clusterID != "in-cluster" {
		if c, err := h.clientManager.GetK8sClient(ctx, clusterID); err == nil && c != nil {
			client = c
		}
		if cCfg, err := h.clientManager.GetK8sRestConfig(ctx, clusterID); err == nil && cCfg != nil {
			cfg = cCfg
		}
	}

	if client == nil {
		client = h.defaultClient
	}
	if cfg == nil {
		cfg = infraK8s.GetLastConfig()
	}

	if client == nil || cfg == nil {
		return nil, nil, infraK8s.ErrK8sUnavailable
	}

	return client, cfg, nil
}

type terminalSizeQueue struct {
	resizeChan chan remotecommand.TerminalSize
}

func newTerminalSizeQueue() *terminalSizeQueue {
	return &terminalSizeQueue{
		resizeChan: make(chan remotecommand.TerminalSize, 16),
	}
}

func (q *terminalSizeQueue) Next() *remotecommand.TerminalSize {
	size, ok := <-q.resizeChan
	if !ok {
		return nil
	}
	return &size
}

type wsConnWriter struct {
	conn *websocket.Conn
	mu   *sync.Mutex
}

func (w *wsConnWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.WriteMessage(websocket.TextMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

type wsExecMessage struct {
	Type   string `json:"type"`
	Data   string `json:"data"`
	Cols   uint16 `json:"cols"`
	Rows   uint16 `json:"rows"`
	Width  uint16 `json:"width"`
	Height uint16 `json:"height"`
}

// HandleExec upgrades an HTTP request to a WebSocket and bridges interactive terminal exec to a Pod.
func (h *K8sExecHandler) HandleExec(w http.ResponseWriter, r *http.Request) {
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

	var cmdArgs []string
	if cmds := r.URL.Query()["command"]; len(cmds) > 0 {
		cmdArgs = cmds
	} else if cmd := r.URL.Query().Get("command"); cmd != "" {
		cmdArgs = []string{cmd}
	} else if cmd := r.URL.Query().Get("cmd"); cmd != "" {
		cmdArgs = strings.Fields(cmd)
	}
	if len(cmdArgs) == 0 {
		cmdArgs = []string{"/bin/sh"}
	}

	client, cfg, err := h.getK8sClientAndConfig(r.Context(), cluster)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get kubernetes client or config", err)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.Get().Error("failed to upgrade websocket for k8s exec", zap.Error(err))
		return
	}
	defer conn.Close()

	execOpts := &corev1.PodExecOptions{
		Command: cmdArgs,
		Stdin:   true,
		Stdout:  true,
		Stderr:  true,
		TTY:     true,
	}
	if container != "" {
		execOpts.Container = container
	}

	req := client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(execOpts, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		logger.Get().Error("failed to create SPDY executor", zap.Error(err))
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nFailed to initialize executor: %v\r\n", err)))
		return
	}

	stdinReader, stdinWriter := io.Pipe()
	defer stdinWriter.Close()

	sizeQueue := newTerminalSizeQueue()
	var writeMu sync.Mutex
	outWriter := &wsConnWriter{conn: conn, mu: &writeMu}

	execCtx, cancelExec := context.WithCancel(r.Context())
	defer cancelExec()

	// Read pump for WebSocket messages
	go func() {
		defer func() {
			_ = stdinWriter.Close()
			cancelExec()
		}()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if len(data) > 0 && data[0] == '{' {
				var ctrlMsg wsExecMessage
				if err := json.Unmarshal(data, &ctrlMsg); err == nil {
					if ctrlMsg.Type == "resize" {
						cols := ctrlMsg.Cols
						if cols == 0 {
							cols = ctrlMsg.Width
						}
						rows := ctrlMsg.Rows
						if rows == 0 {
							rows = ctrlMsg.Height
						}
						if cols > 0 && rows > 0 {
							select {
							case sizeQueue.resizeChan <- remotecommand.TerminalSize{Width: cols, Height: rows}:
							default:
							}
						}
						continue
					} else if ctrlMsg.Type == "stdin" {
						if _, err := stdinWriter.Write([]byte(ctrlMsg.Data)); err != nil {
							return
						}
						continue
					} else if ctrlMsg.Type == "ping" {
						writeMu.Lock()
						_ = conn.WriteMessage(websocket.PongMessage, nil)
						writeMu.Unlock()
						continue
					}
				}
			}
			if _, err := stdinWriter.Write(data); err != nil {
				return
			}
		}
	}()

	err = executor.StreamWithContext(execCtx, remotecommand.StreamOptions{
		Stdin:             stdinReader,
		Stdout:            outWriter,
		Stderr:            outWriter,
		Tty:               true,
		TerminalSizeQueue: sizeQueue,
	})

	close(sizeQueue.resizeChan)
	if err != nil {
		logger.Get().Debug("k8s exec session closed", zap.Error(err))
		writeMu.Lock()
		_ = conn.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf("\r\nSession closed: %v\r\n", err)))
		writeMu.Unlock()
	}
}

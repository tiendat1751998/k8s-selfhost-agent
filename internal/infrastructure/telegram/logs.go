package telegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// StripANSI strips ANSI color and terminal escape sequences.
func StripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

// FormatPodLogs formats raw pod logs into a Telegram pre-formatted code block,
// keeping at most 50 lines and capping at 3900 characters (well under Telegram's 4096 limit).
func FormatPodLogs(rawLogs string) string {
	cleaned := StripANSI(rawLogs)
	cleaned = strings.TrimRight(cleaned, "\r\n")
	if cleaned == "" {
		return "<pre>\n(no logs recorded)\n</pre>"
	}

	lines := strings.Split(cleaned, "\n")
	if len(lines) > 50 {
		lines = lines[len(lines)-50:]
	}

	content := strings.Join(lines, "\n")

	// Max safe length inside <pre>...</pre> tag to guarantee total message < 4000 characters.
	const maxContentLen = 3900
	if len(content) > maxContentLen {
		truncatedNotice := "...[TRUNCATED TO LAST LINES]...\n"
		keepLen := maxContentLen - len(truncatedNotice)
		if keepLen > 0 && keepLen < len(content) {
			content = truncatedNotice + content[len(content)-keepLen:]
		}
	}

	return fmt.Sprintf("<pre>\n%s\n</pre>", content)
}

// PodLogFetcher abstracts pod log extraction.
type PodLogFetcher interface {
	GetPodLogs(ctx context.Context, namespace, pod string, tailLines int64) (string, error)
}

// K8sPodLogFetcher fetches pod logs from real Kubernetes CoreV1 Pods API.
type K8sPodLogFetcher struct {
	client kubernetes.Interface
}

// NewK8sPodLogFetcher creates a log fetcher backed by a Kubernetes client.
func NewK8sPodLogFetcher(client kubernetes.Interface) *K8sPodLogFetcher {
	return &K8sPodLogFetcher{client: client}
}

// GetPodLogs retrieves tail logs for a pod.
func (f *K8sPodLogFetcher) GetPodLogs(ctx context.Context, namespace, pod string, tailLines int64) (string, error) {
	if f.client == nil {
		return "", fmt.Errorf("kubernetes client is nil")
	}
	if namespace == "" {
		namespace = "default"
	}

	opts := &corev1.PodLogOptions{
		TailLines: &tailLines,
	}

	req := f.client.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("streaming pod logs for %s/%s: %w", namespace, pod, err)
	}
	defer stream.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, stream); err != nil {
		return "", fmt.Errorf("reading pod logs: %w", err)
	}
	return buf.String(), nil
}

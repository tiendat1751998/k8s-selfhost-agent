package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
)

// HandleTailLogs handles GET /logs/tail?service=<name>&tail=<n>
func (s *LogServer) HandleTailLogs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	cli := s.dockerCli
	eng := s.engineSrc
	sources := s.sources
	s.mu.RUnlock()

	q := r.URL.Query()
	service := q.Get("service")
	if service == "" {
		service = q.Get("app")
	}
	if service == "" {
		service = q.Get("container")
	}

	tailStr := q.Get("tail")
	tail := 100
	if tailStr != "" && tailStr != "0" {
		if t, err := strconv.Atoi(tailStr); err == nil && t > 0 {
			tail = t
		}
	}

	var targetContainerID string
	if cli != nil && service != "" {
		containers, err := cli.ContainerList(r.Context(), container.ListOptions{All: true})
		if err == nil {
			for _, c := range containers {
				cID := c.ID
				cName := cID
				if len(c.Names) > 0 {
					cName = strings.TrimPrefix(c.Names[0], "/")
				}
				swarmSvc := c.Labels["com.docker.swarm.service.name"]
				composeSvc := c.Labels["com.docker.compose.service"]

				if service == cID || (len(cID) > 12 && service == cID[:12]) ||
					service == cName || service == swarmSvc || service == composeSvc {
					targetContainerID = cID
					break
				}
			}
		}
	}

	if targetContainerID == "" {
		if eng != nil || (cli == nil && len(sources) > 0) || service == "" {
			s.streamFromEngine(w, r, service, tail)
			return
		}
		if cli == nil {
			http.Error(w, "docker client unavailable", http.StatusServiceUnavailable)
			return
		}
		http.Error(w, fmt.Sprintf("service %q not found", service), http.StatusNotFound)
		return
	}

	reader, err := cli.ContainerLogs(r.Context(), targetContainerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Timestamps: true,
		Tail:       tailStr,
	})
	if err != nil {
		http.Error(w, "failed to stream logs: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	done := make(chan struct{})
	go func() {
		select {
		case <-r.Context().Done():
			_ = reader.Close()
		case <-done:
		}
	}()
	defer close(done)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	flusher, hasFlusher := w.(http.Flusher)

	bufReader := bufio.NewReader(reader)
	header, peekErr := bufReader.Peek(8)
	isStdCopy := peekErr == nil && len(header) == 8 && (header[0] == 1 || header[0] == 2) && header[1] == 0 && header[2] == 0 && header[3] == 0

	var scanner *bufio.Scanner
	if isStdCopy {
		pr, pw := io.Pipe()
		defer pr.Close()
		go func() {
			_, _ = stdcopy.StdCopy(pw, pw, bufReader)
			_ = pw.Close()
		}()
		scanner = bufio.NewScanner(pr)
	} else {
		scanner = bufio.NewScanner(bufReader)
	}

	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		if r.Context().Err() != nil {
			return
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", line); err != nil {
			return
		}
		if hasFlusher {
			flusher.Flush()
		}
	}

	if err := scanner.Err(); err != nil && r.Context().Err() == nil {
		slog.Error("scanner error reading log stream", slog.Any("error", err))
		_, _ = fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		if hasFlusher {
			flusher.Flush()
		}
	}
}

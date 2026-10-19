package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

const (
	maxPartialBufferSize = 1024 * 1024 // 1MB dynamic line buffer
	defaultPodLogDir     = "/var/log/pods"
)

// CRILogLine represents a parsed Kubernetes CRI log line.
type CRILogLine struct {
	Timestamp time.Time
	Stream    string // "stdout" or "stderr"
	Flag      string // "P" (partial) or "F" (final)
	Content   string
}

// PodMeta holds Kubernetes metadata extracted from the CRI log path.
type PodMeta struct {
	Namespace     string
	PodName       string
	PodUID        string
	ContainerName string
	ServiceName   string
}

type podFileState struct {
	offset        int64
	lastSize      int64
	partialBuf    strings.Builder
	partialTime   time.Time
	partialStream string
}

// PodTailer tails Kubernetes CRI pod logs from /var/log/pods directly.
type PodTailer struct {
	baseDir        string
	checkpointPath string
	writer         *logengine.Writer
	logger         *slog.Logger
	pollInterval   time.Duration
	entryWriter    func(logengine.Entry) error // test hook

	mu      sync.Mutex
	files   map[string]*podFileState
	wg      sync.WaitGroup
	stopped bool
}

// NewPodTailer initializes a PodTailer.
func NewPodTailer(baseDir string, writer *logengine.Writer, logger *slog.Logger, checkpointPath ...string) *PodTailer {
	if baseDir == "" {
		baseDir = defaultPodLogDir
	}
	if logger == nil {
		logger = slog.Default()
	}
	var cp string
	if len(checkpointPath) > 0 {
		cp = checkpointPath[0]
	}
	tailer := &PodTailer{
		baseDir:        baseDir,
		checkpointPath: cp,
		writer:         writer,
		logger:         logger,
		pollInterval:   3 * time.Second,
		files:          make(map[string]*podFileState),
	}
	tailer.loadCheckpoint()
	return tailer
}

// Start initiates periodic discovery and tailing until ctx is canceled.
func (p *PodTailer) Start(ctx context.Context) {
	p.logger.Info("Starting direct Kubernetes CRI pod log tailer", slog.String("base_dir", p.baseDir))
	p.pollOnce(ctx)

	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.Stop()
			return
		case <-ticker.C:
			p.pollOnce(ctx)
		}
	}
}

// Stop cleanly terminates the PodTailer.
func (p *PodTailer) Stop() {
	p.mu.Lock()
	p.stopped = true
	p.mu.Unlock()
	p.wg.Wait()
	p.saveCheckpoint()
	p.logger.Info("Pod log tailer stopped")
}

func (p *PodTailer) pollOnce(ctx context.Context) {
	p.mu.Lock()
	if p.stopped {
		p.mu.Unlock()
		return
	}
	p.wg.Add(1)
	defer p.wg.Done()
	p.mu.Unlock()

	pattern := filepath.Join(p.baseDir, "*", "*", "*.log")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		p.logger.Debug("Glob error scanning pod logs", slog.String("error", err.Error()))
		return
	}

	var dirty bool
	seen := make(map[string]bool, len(matches))
	for _, path := range matches {
		if ctx.Err() != nil {
			return
		}
		// Skip rotated files (.log.1, .log.gz)
		base := filepath.Base(path)
		if rotatedLogRegex.MatchString(base) {
			continue
		}

		seen[path] = true
		if p.tailFile(ctx, path) {
			dirty = true
		}
	}

	// Prune dead files no longer existing on disk
	p.mu.Lock()
	for path := range p.files {
		if !seen[path] {
			delete(p.files, path)
			dirty = true
		}
	}
	p.mu.Unlock()

	if dirty {
		p.saveCheckpoint()
	}
}

func (p *PodTailer) tailFile(ctx context.Context, path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}

	p.mu.Lock()
	state, exists := p.files[path]
	if !exists {
		state = &podFileState{}
		p.files[path] = state
	}
	p.mu.Unlock()

	var modified bool
	// Detect log rotation: file truncated or replaced
	if fi.Size() < state.offset {
		// Check if rotated file .1 exists to read any remaining unread bytes
		rotatedPath := path + ".1"
		if rfi, rerr := os.Stat(rotatedPath); rerr == nil && rfi.Size() > state.offset {
			if p.readChunk(ctx, rotatedPath, state, rfi.Size()) {
				modified = true
			}
		}
		state.offset = 0
		state.partialBuf.Reset()
		modified = true
	}

	if fi.Size() > state.offset {
		if p.readChunk(ctx, path, state, fi.Size()) {
			modified = true
		}
	}
	return modified
}

func (p *PodTailer) readChunk(ctx context.Context, path string, state *podFileState, currentSize int64) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	if state.offset > 0 {
		if _, err := f.Seek(state.offset, io.SeekStart); err != nil {
			return false
		}
	}

	meta := extractPodMetaFromPath(path)
	reader := bufio.NewReader(f)

	var bytesRead int64
	for {
		if ctx.Err() != nil {
			break
		}
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			bytesRead += int64(len(line))
			p.processLine(line, meta, state)
		}
		if readErr != nil {
			break
		}
	}

	state.offset += bytesRead
	state.lastSize = currentSize
	return bytesRead > 0
}

func (p *PodTailer) processLine(rawLine string, meta PodMeta, state *podFileState) {
	cleanLine := strings.TrimRight(rawLine, "\r\n")
	if cleanLine == "" {
		return
	}

	parsed, ok := parseCRILine(cleanLine)
	if !ok {
		// Fallback for non-CRI formatted plain lines
		p.emitEntry(logengine.Entry{
			Timestamp: time.Now().UTC(),
			Service:   meta.ServiceName,
			Level:     "info",
			Message:   cleanLine,
			Raw:       rawLine,
		})
		return
	}

	if parsed.Flag == "P" {
		if state.partialBuf.Len() == 0 {
			state.partialTime = parsed.Timestamp
			state.partialStream = parsed.Stream
		}
		state.partialBuf.WriteString(parsed.Content)

		// Dynamic 1MB line buffer guard: flush if exceeding capacity
		if state.partialBuf.Len() >= maxPartialBufferSize {
			p.emitEntry(logengine.Entry{
				Timestamp: state.partialTime,
				Service:   meta.ServiceName,
				Level:     streamToLevel(state.partialStream),
				Message:   state.partialBuf.String(),
				Raw:       rawLine,
			})
			state.partialBuf.Reset()
		}
		return
	}

	// Final chunk (Flag == "F")
	var finalMessage string
	var finalTime time.Time
	var stream string

	if state.partialBuf.Len() > 0 {
		state.partialBuf.WriteString(parsed.Content)
		finalMessage = state.partialBuf.String()
		finalTime = state.partialTime
		stream = state.partialStream
		state.partialBuf.Reset()
	} else {
		finalMessage = parsed.Content
		finalTime = parsed.Timestamp
		stream = parsed.Stream
	}

	p.emitEntry(logengine.Entry{
		Timestamp: finalTime,
		Service:   meta.ServiceName,
		Level:     streamToLevel(stream),
		Message:   finalMessage,
		Raw:       rawLine,
	})
}

func (p *PodTailer) emitEntry(entry logengine.Entry) {
	if p.entryWriter != nil {
		_ = p.entryWriter(entry)
		return
	}
	if p.writer != nil {
		_ = p.writer.Write(entry)
	}
}

func streamToLevel(stream string) string {
	if stream == "stderr" {
		return "error"
	}
	return "info"
}

// parseCRILine parses `<RFC3339Nano-timestamp> <stream:stdout|stderr> <flag:P|F> <content>`.
func parseCRILine(line string) (CRILogLine, bool) {
	s1 := strings.IndexByte(line, ' ')
	if s1 <= 0 {
		return CRILogLine{}, false
	}
	rest1 := line[s1+1:]
	s2 := strings.IndexByte(rest1, ' ')
	if s2 <= 0 {
		return CRILogLine{}, false
	}
	rest2 := rest1[s2+1:]
	s3 := strings.IndexByte(rest2, ' ')
	if s3 <= 0 {
		return CRILogLine{}, false
	}

	tsStr := line[:s1]
	stream := rest1[:s2]
	flag := rest2[:s3]
	content := rest2[s3+1:]

	ts, err := time.Parse(time.RFC3339Nano, tsStr)
	if err != nil {
		return CRILogLine{}, false
	}

	return CRILogLine{
		Timestamp: ts,
		Stream:    stream,
		Flag:      flag,
		Content:   content,
	}, true
}

// extractPodMetaFromPath parses Kubernetes metadata from path:
// `/var/log/pods/<namespace>_<pod-name>_<pod-uid>/<container-name>/<restart-count>.log`
func extractPodMetaFromPath(path string) PodMeta {
	clean := filepath.ToSlash(path)
	parts := strings.Split(clean, "/")
	if len(parts) < 3 {
		base := strings.TrimSuffix(filepath.Base(path), ".log")
		return PodMeta{ServiceName: base}
	}

	containerName := parts[len(parts)-2]
	podDir := parts[len(parts)-3]

	podDirParts := strings.Split(podDir, "_")
	var ns, podName, uid string
	if len(podDirParts) >= 3 {
		ns = podDirParts[0]
		uid = podDirParts[len(podDirParts)-1]
		podName = strings.Join(podDirParts[1:len(podDirParts)-1], "_")
	} else if len(podDirParts) == 2 {
		ns = podDirParts[0]
		podName = podDirParts[1]
	} else {
		podName = podDir
	}

	svcName := podName
	if svcName == "" {
		svcName = containerName
	}

	return PodMeta{
		Namespace:     ns,
		PodName:       podName,
		PodUID:        uid,
		ContainerName: containerName,
		ServiceName:   svcName,
	}
}

func (p *PodTailer) loadCheckpoint() {
	if p.checkpointPath == "" {
		return
	}
	data, err := os.ReadFile(p.checkpointPath)
	if err != nil {
		return
	}
	var offsets map[string]int64
	if err := json.Unmarshal(data, &offsets); err != nil {
		p.logger.Warn("Failed to unmarshal tail offsets checkpoint", slog.String("error", err.Error()))
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	for path, offset := range offsets {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		if fi.Size() >= offset {
			p.files[path] = &podFileState{offset: offset, lastSize: fi.Size()}
		} else {
			p.files[path] = &podFileState{offset: 0, lastSize: fi.Size()}
		}
	}
}

func (p *PodTailer) saveCheckpoint() {
	if p.checkpointPath == "" {
		return
	}
	p.mu.Lock()
	offsets := make(map[string]int64, len(p.files))
	for path, state := range p.files {
		offsets[path] = state.offset
	}
	p.mu.Unlock()

	data, err := json.MarshalIndent(offsets, "", "  ")
	if err != nil {
		return
	}

	dir := filepath.Dir(p.checkpointPath)
	_ = os.MkdirAll(dir, 0755)

	tmpFile := p.checkpointPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		p.logger.Debug("Failed to write checkpoint temp file", slog.String("error", err.Error()))
		return
	}
	if err := os.Rename(tmpFile, p.checkpointPath); err != nil {
		_ = os.Remove(tmpFile)
		_ = os.WriteFile(p.checkpointPath, data, 0644)
	}
}


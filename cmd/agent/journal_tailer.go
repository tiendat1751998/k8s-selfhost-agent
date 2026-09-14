package main

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/logengine"
)

// JournalTailer streams journalctl logs into the embedded columnar logengine.Writer.
type JournalTailer struct {
	writer         *logengine.Writer
	logger         *slog.Logger
	availableCheck func() bool
	streamReader   func(ctx context.Context) (ioCloserReader, error)
}

// NewJournalTailer initializes a JournalTailer.
func NewJournalTailer(writer *logengine.Writer, logger *slog.Logger) *JournalTailer {
	if logger == nil {
		logger = slog.Default()
	}
	return &JournalTailer{
		writer: writer,
		logger: logger,
		availableCheck: func() bool {
			_, err := exec.LookPath("journalctl")
			return err == nil
		},
	}
}

// Start checks availability and begins continuous log streaming until ctx is cancelled.
func (j *JournalTailer) Start(ctx context.Context) {
	if j.writer == nil {
		return
	}

	if !j.availableCheck() {
		j.logger.Info("journalctl not available on this platform/environment, skipping journal log ingestion")
		return
	}

	j.logger.Info("Starting continuous journalctl log tailer")
	j.streamLoop(ctx)
}

type journalJSONEntry struct {
	Message           string `json:"MESSAGE"`
	RealtimeTimestamp string `json:"__REALTIME_TIMESTAMP"`
	SystemdUnit       string `json:"_SYSTEMD_UNIT"`
	SyslogIdentifier  string `json:"SYSLOG_IDENTIFIER"`
	Priority          string `json:"PRIORITY"`
}

func (j *JournalTailer) streamLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		r, err := j.openStream(ctx)
		if err != nil {
			j.logger.Warn("Failed to start journalctl stream, retrying in 5s", slog.String("error", err.Error()))
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}

		j.readStream(ctx, r)
		_ = r.Close()

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
		}
	}
}

func (j *JournalTailer) openStream(ctx context.Context) (ioCloserReader, error) {
	if j.streamReader != nil {
		return j.streamReader(ctx)
	}

	cmd := exec.CommandContext(ctx, "journalctl", "-f", "-o", "json", "-n", "100", "--no-pager")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdout.Close()
		return nil, err
	}

	return &cmdStreamCloser{ReadCloser: stdout, cmd: cmd}, nil
}

type cmdStreamCloser struct {
	io.ReadCloser
	cmd *exec.Cmd
}

func (c *cmdStreamCloser) Close() error {
	_ = c.ReadCloser.Close()
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_ = c.cmd.Wait()
	}
	return nil
}

func (j *JournalTailer) readStream(ctx context.Context, r io.Reader) {
	scanner := bufio.NewScanner(r)
	// Strictly bound line buffer < 64KB
	scanner.Buffer(make([]byte, 4096), 64*1024)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		entry := j.parseJournalLine(line)
		if j.writer != nil {
			_ = j.writer.Write(logengine.Entry{
				Timestamp: entry.Timestamp,
				Service:   entry.Service,
				Level:     entry.Level,
				Message:   entry.Message,
				Raw:       entry.Raw,
			})
		}
	}
}

func (j *JournalTailer) parseJournalLine(line string) LogEntry {
	var je journalJSONEntry
	if err := json.Unmarshal([]byte(line), &je); err == nil && je.Message != "" {
		ts := time.Now().UTC()
		if je.RealtimeTimestamp != "" {
			if micro, err := strconv.ParseInt(je.RealtimeTimestamp, 10, 64); err == nil {
				ts = time.Unix(0, micro*1000).UTC()
			}
		}

		svc := strings.TrimSpace(je.SystemdUnit)
		if svc == "" {
			svc = strings.TrimSpace(je.SyslogIdentifier)
		}
		if svc == "" {
			svc = "journald"
		}

		lvl := priorityToLevel(je.Priority)

		return LogEntry{
			Timestamp: ts,
			Service:   svc,
			Level:     lvl,
			Message:   je.Message,
			Raw:       line,
		}
	}

	return parseLogLine(line, "journald")
}

func priorityToLevel(p string) string {
	switch strings.TrimSpace(p) {
	case "0", "1", "2", "3":
		return "error"
	case "4":
		return "warn"
	case "5", "6":
		return "info"
	case "7":
		return "debug"
	default:
		return "info"
	}
}

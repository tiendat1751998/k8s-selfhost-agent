package logengine

import "time"

// Entry represents a single log entry passed into the log engine.
type Entry struct {
	Timestamp time.Time
	Service   string
	Level     string
	Message   string
	Raw       string
}

// WriterOption configures the Writer.
type WriterOption func(*Writer)

// WithMaxRowsPerBlock configures the threshold rows before flushing a block.
func WithMaxRowsPerBlock(n int) WriterOption {
	return func(w *Writer) {
		if n > 0 {
			w.maxRows = n
		}
	}
}

// WithFlushInterval configures the maximum idle time before flushing a block.
func WithFlushInterval(d time.Duration) WriterOption {
	return func(w *Writer) {
		if d > 0 {
			w.flushInterval = d
		}
	}
}

// WithRedactor injects a custom in-stream redactor for masking secrets.
func WithRedactor(r *InStreamRedactor) WriterOption {
	return func(w *Writer) {
		w.redactor = r
	}
}

// WithRingBufferCapacity configures the ingest ring buffer capacity.
func WithRingBufferCapacity(cap int) WriterOption {
	return func(w *Writer) {
		if cap > 0 {
			w.ringCap = cap
		}
	}
}

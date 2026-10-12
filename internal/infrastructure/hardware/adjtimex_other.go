//go:build !linux

package hardware

import "time"

type fallbackClockReader struct{}

// NewClockReader returns a portable fallback ClockReader for non-Linux platforms.
func NewClockReader() ClockReader {
	return &fallbackClockReader{}
}

func (r *fallbackClockReader) ReadTimeOffset() (time.Duration, bool, error) {
	// Safe portable fallback for non-Linux operating systems
	return 0, true, nil
}

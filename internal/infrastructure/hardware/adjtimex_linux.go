//go:build linux

package hardware

import (
	"fmt"
	"time"

	"golang.org/x/sys/unix"
)

type linuxClockReader struct{}

// NewClockReader returns an OS-level ClockReader for Linux via adjtimex syscall.
func NewClockReader() ClockReader {
	return &linuxClockReader{}
}

func (r *linuxClockReader) ReadTimeOffset() (time.Duration, bool, error) {
	var tx unix.Timex
	state, err := unix.Adjtimex(&tx)
	if err != nil {
		return 0, false, fmt.Errorf("adjtimex syscall failed: %w", err)
	}

	// Status flags defined in linux/timex.h
	const staNano = 0x2000   // STA_NANO: offset in nanoseconds
	const staUnsync = 0x0040 // STA_UNSYNC: clock unsynchronized

	var offset time.Duration
	if tx.Status&staNano != 0 {
		offset = time.Duration(tx.Offset) * time.Nanosecond
	} else {
		offset = time.Duration(tx.Offset) * time.Microsecond
	}

	isSynchronized := (state != unix.TIME_ERROR) && (tx.Status&staUnsync == 0)
	return offset, isSynchronized, nil
}

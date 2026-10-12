package hardware

import "time"

// ClockReader abstracts reading the host system clock offset and NTP sync status.
type ClockReader interface {
	ReadTimeOffset() (offset time.Duration, isSynchronized bool, err error)
}

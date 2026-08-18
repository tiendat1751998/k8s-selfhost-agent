package telegram

import (
	"fmt"
	"sync"
	"time"
)

type AlertDebouncer struct {
	mu          sync.Mutex
	window      time.Duration
	pending     map[string]*AlertPayload
	timers      map[string]*time.Timer
	dispatchFn  func(alert *AlertPayload)
}

func NewAlertDebouncer(window time.Duration, dispatchFn func(alert *AlertPayload)) *AlertDebouncer {
	if window <= 0 {
		window = 10 * time.Second
	}
	return &AlertDebouncer{
		window:     window,
		pending:    make(map[string]*AlertPayload),
		timers:     make(map[string]*time.Timer),
		dispatchFn: dispatchFn,
	}
}

func (d *AlertDebouncer) fingerprint(a *AlertPayload) string {
	if a.Fingerprint != "" {
		return a.Fingerprint
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s", a.Cluster, a.Namespace, a.Service, a.Severity, a.Message)
}

func (d *AlertDebouncer) Push(alert *AlertPayload) {
	d.mu.Lock()
	defer d.mu.Unlock()

	fp := d.fingerprint(alert)
	existing, ok := d.pending[fp]
	if ok {
		// Aggregate cascading storms
		existing.Count++
		if alert.Pod != "" && existing.Pod != alert.Pod {
			existing.Pod = fmt.Sprintf("%s, %s", existing.Pod, alert.Pod)
		}
		if alert.RCAAnalysis != "" {
			existing.RCAAnalysis = alert.RCAAnalysis
		}
		return
	}

	// First occurrence
	alert.Count = 1
	alert.Fingerprint = fp
	if alert.Timestamp.IsZero() {
		alert.Timestamp = time.Now().UTC()
	}
	d.pending[fp] = alert

	// Schedule flush timer
	t := time.AfterFunc(d.window, func() {
		d.flushKey(fp)
	})
	d.timers[fp] = t
}

func (d *AlertDebouncer) flushKey(key string) {
	d.mu.Lock()
	alert, ok := d.pending[key]
	if !ok {
		d.mu.Unlock()
		return
	}
	delete(d.pending, key)
	if timer, hasTimer := d.timers[key]; hasTimer {
		timer.Stop()
		delete(d.timers, key)
	}
	d.mu.Unlock()

	if d.dispatchFn != nil {
		d.dispatchFn(alert)
	}
}

func (d *AlertDebouncer) FlushAll() {
	d.mu.Lock()
	keys := make([]string, 0, len(d.pending))
	for k := range d.pending {
		keys = append(keys, k)
	}
	d.mu.Unlock()

	for _, k := range keys {
		d.flushKey(k)
	}
}

package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectLogLevel(t *testing.T) {
	tests := []struct {
		msg        string
		defaultLvl string
		expected   string
	}{
		{"database error: connection lost", "INFO", "ERROR"},
		{"fatal: cannot bind socket", "INFO", "ERROR"},
		{"panic: nil pointer dereference", "INFO", "ERROR"},
		{"warning: high memory usage detected", "INFO", "WARN"},
		{"debug: cache hit ratio 0.94", "INFO", "DEBUG"},
		{"trace: entering function foo", "INFO", "DEBUG"},
		{"info: worker initialized", "WARN", "INFO"},
		{"generic log output without level keyword", "NOTICE", "NOTICE"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			actual := detectLogLevel(tt.msg, tt.defaultLvl)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestDockerTailerManager_AttachAndCancel(t *testing.T) {
	mgr := &dockerTailerManager{
		cancels: make(map[string]context.CancelFunc),
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initial cancellation tracking
	cID := "container-12345"
	subCtx, subCancel := context.WithCancel(ctx)
	mgr.mu.Lock()
	mgr.cancels[cID] = subCancel
	mgr.mu.Unlock()

	mgr.mu.Lock()
	_, exists := mgr.cancels[cID]
	mgr.mu.Unlock()
	assert.True(t, exists)

	// Cancel simulates restart event
	mgr.cancel(cID)

	assert.Equal(t, context.Canceled, subCtx.Err())

	mgr.mu.Lock()
	_, existsAfter := mgr.cancels[cID]
	mgr.mu.Unlock()
	assert.False(t, existsAfter)
}

package agent

import (
	"testing"
	"time"
)

func TestHostBackoffManager_CooldownAndRecovery(t *testing.T) {
	bm := NewHostBackoffManager(100 * time.Millisecond)

	endpoint := "10.10.10.60:9100"

	// Initially reachable
	if bm.IsUnreachable(endpoint) {
		t.Fatalf("expected endpoint to be reachable initially")
	}

	// Mark failed -> should be unreachable
	bm.MarkFailed(endpoint)
	if !bm.IsUnreachable(endpoint) {
		t.Fatalf("expected endpoint to be unreachable after MarkFailed")
	}

	// Normalizes http prefix
	if !bm.IsUnreachable("http://10.10.10.60:9100") {
		t.Fatalf("expected normalized endpoint to match unreachable status")
	}

	// Successful recovery clears unreachable status immediately
	bm.MarkSuccess(endpoint)
	if bm.IsUnreachable(endpoint) {
		t.Fatalf("expected endpoint to be reachable immediately after MarkSuccess")
	}

	// Mark failed again and wait for cooldown expiry
	bm.MarkFailed(endpoint)
	if !bm.IsUnreachable(endpoint) {
		t.Fatalf("expected endpoint to be unreachable after MarkFailed")
	}

	time.Sleep(120 * time.Millisecond)
	if bm.IsUnreachable(endpoint) {
		t.Fatalf("expected endpoint to be reachable after cooldown elapsed")
	}
}

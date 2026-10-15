package vfswatchdog

import (
	"context"
	"testing"
)

func TestDefaultFSProvider_StatFS_NoHardcodedValues(t *testing.T) {
	p := &defaultFSProvider{}
	total, free, err := p.StatFS(context.Background(), "/")
	if total == 10000000 || free == 7500000 {
		t.Fatalf("StatFS must not return hardcoded stub values, got total=%d free=%d", total, free)
	}
	if err == nil {
		t.Fatal("expected error from defaultFSProvider.StatFS without an OS-specific statfs provider")
	}
}

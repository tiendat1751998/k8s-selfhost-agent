package logger

import (
	"context"
	"sync"
	"testing"

	"go.uber.org/zap"
)

func resetGlobal() {
	global = nil
	once = sync.Once{}
}

func TestInit_DefaultLevel(t *testing.T) {
	resetGlobal()
	Init("info")
	l := Get()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestGet_LazyInit(t *testing.T) {
	resetGlobal()
	l := Get()
	if l == nil {
		t.Fatal("expected non-nil logger from lazy init")
	}
}

func TestWithContext_NoLogger(t *testing.T) {
	resetGlobal()
	ctx := context.Background()
	l := WithContext(ctx)
	if l == nil {
		t.Fatal("expected non-nil logger from context")
	}
}

func TestToContext_RoundTrip(t *testing.T) {
	original := zap.NewNop()
	ctx := ToContext(context.Background(), original)
	retrieved := WithContext(ctx)

	if retrieved != original {
		t.Error("expected same logger instance from context round-trip")
	}
}

func TestWith(t *testing.T) {
	resetGlobal()
	Init("debug")
	l := With(zap.String("component", "test"))
	if l == nil {
		t.Fatal("expected non-nil child logger")
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
		{"unknown", "info"},
	}

	for _, tc := range tests {
		level := parseLevel(tc.input)
		if level.String() != tc.want {
			t.Errorf("parseLevel(%q) = %s, want %s", tc.input, level.String(), tc.want)
		}
	}
}

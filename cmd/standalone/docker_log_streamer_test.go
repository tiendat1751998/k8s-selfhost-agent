package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTraceID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "logfmt with trace_id=",
			input:    "2026-09-16 08:00:00 [ERROR] trace_id=c0ffee1234 service error",
			expected: "c0ffee1234",
		},
		{
			name:     "logfmt with traceId quotes",
			input:    `level=info traceId="9876-abcd" msg="hello"`,
			expected: "9876-abcd",
		},
		{
			name:     "json trace_id",
			input:    `{"timestamp":"2026-09-16T08:00:00Z","trace_id":"xyz-123","message":"user login"}`,
			expected: "xyz-123",
		},
		{
			name:     "json traceId",
			input:    `{"traceId":"abc-789","level":"warn"}`,
			expected: "abc-789",
		},
		{
			name:     "hyphenated trace-id=",
			input:    "trace-id=feedbeef99 container started",
			expected: "feedbeef99",
		},
		{
			name:     "colon formatted trace_id",
			input:    `trace_id: "token-456", detail: "ok"`,
			expected: "token-456",
		},
		{
			name:     "no trace present",
			input:    "standard nginx access log without tracing 200 OK",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := extractTraceID(tc.input)
			require.Equal(t, tc.expected, actual)
		})
	}
}

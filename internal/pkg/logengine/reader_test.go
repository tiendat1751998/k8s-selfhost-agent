package logengine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchFilter_ExactAndBoundary(t *testing.T) {
	tests := []struct {
		name     string
		actual   string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match same case",
			actual:   "postgres",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "exact match different case",
			actual:   "Postgres_DB",
			pattern:  "postgres_db",
			expected: true,
		},
		{
			name:     "short pattern <= 3 exact match allowed",
			actual:   "db",
			pattern:  "db",
			expected: true,
		},
		{
			name:     "short pattern <= 3 substring in dbus.service MUST NOT MATCH",
			actual:   "dbus.service",
			pattern:  "db",
			expected: false,
		},
		{
			name:     "short pattern <= 3 substring in postgres_db MUST NOT MATCH",
			actual:   "postgres_db",
			pattern:  "db",
			expected: false,
		},
		{
			name:     "short pattern <= 3 substring in systemd-udevd MUST NOT MATCH",
			actual:   "systemd-udevd",
			pattern:  "dev",
			expected: false,
		},
		{
			name:     "prefix boundary with underscore",
			actual:   "postgres_db",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "prefix boundary with hyphen",
			actual:   "postgres-main",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "prefix boundary with dot",
			actual:   "dbus.service",
			pattern:  "dbus",
			expected: true,
		},
		{
			name:     "suffix boundary with hyphen",
			actual:   "prod-postgres",
			pattern:  "postgres",
			expected: true,
		},
		{
			name:     "arbitrary substring inside service name without boundary MUST NOT MATCH",
			actual:   "postgresql",
			pattern:  "postgres",
			expected: false,
		},
		{
			name:     "arbitrary substring in middle MUST NOT MATCH",
			actual:   "unauthorized-worker",
			pattern:  "auth",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := matchFilter(tc.actual, tc.pattern)
			assert.Equal(t, tc.expected, got, "matchFilter(%q, %q)", tc.actual, tc.pattern)
		})
	}
}

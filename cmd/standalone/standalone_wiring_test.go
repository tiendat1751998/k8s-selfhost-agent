package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseDockerLogLine(t *testing.T) {
	t.Run("RFC3339Nano timestamp prefix", func(t *testing.T) {
		input := "2026-09-16T05:14:12.123456789Z database system is ready"
		parsedTime, msg := parseDockerLogLine(input)

		expectedTime := time.Date(2026, 9, 16, 5, 14, 12, 123456789, time.UTC)
		assert.Equal(t, expectedTime, parsedTime)
		assert.Equal(t, "database system is ready", msg)
	})

	t.Run("RFC3339 standard timestamp prefix", func(t *testing.T) {
		input := "2026-09-16T05:14:12Z worker active"
		parsedTime, msg := parseDockerLogLine(input)

		expectedTime := time.Date(2026, 9, 16, 5, 14, 12, 0, time.UTC)
		assert.Equal(t, expectedTime, parsedTime)
		assert.Equal(t, "worker active", msg)
	})

	t.Run("RFC3339Nano with offset", func(t *testing.T) {
		input := "2026-09-16T07:14:12.123456789+02:00 server started"
		parsedTime, msg := parseDockerLogLine(input)

		expectedTime := time.Date(2026, 9, 16, 5, 14, 12, 123456789, time.UTC)
		assert.Equal(t, expectedTime, parsedTime)
		assert.Equal(t, "server started", msg)
	})

	t.Run("no timestamp prefix fallback", func(t *testing.T) {
		input := "plain log line without docker timestamp"
		before := time.Now().UTC().Add(-time.Second)
		parsedTime, msg := parseDockerLogLine(input)
		after := time.Now().UTC().Add(time.Second)

		assert.True(t, parsedTime.After(before) && parsedTime.Before(after))
		assert.Equal(t, input, msg)
	})
}

// Package concurrency provides helpers for safe concurrent programming.
package concurrency

import (
	"go.uber.org/zap"
)

// Go runs the given function in a new goroutine with panic recovery.
func Go(log *zap.Logger, fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error("recovered from panic in background goroutine",
					zap.Any("panic", r),
				)
			}
		}()
		fn()
	}()
}

//go:generate moq -out logger_mock.go -pkg loggertest ../. Logger
package loggertest

import "context"

// NewLoggerMock returns a LoggerMock with no-op implementations.
func NewLoggerMock() *LoggerMock {
	return &LoggerMock{
		InfoFunc:       func(ctx context.Context, msg string, kv ...any) {},
		DebugFunc:      func(ctx context.Context, msg string, kv ...any) {},
		TraceFunc:      func(ctx context.Context, msg string, kv ...any) {},
		ErrorFunc:      func(ctx context.Context, msg string, err error, kv ...any) {},
		WithFieldsFunc: func(ctx context.Context, kv ...any) context.Context { return ctx },
		SetLevelFunc:   func(ctx context.Context, level string) error { return nil },
		GetLevelFunc:   func() string { return "info" },
	}
}

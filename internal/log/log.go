// Package log is the CLI's diagnostics channel: warnings and errors about
// things the user cannot see happening (unreadable paths, a cache entry that
// could not be written) as structured records on stderr.
//
// It is deliberately narrow. Human-facing output stays in the output package on
// stdout; this is not a general logging framework, has no levels to configure,
// no sinks and no file output, and must stay that way.
package log

import (
	"io"
	"log/slog"
	"os"
	"sync"
)

var (
	mu       sync.Mutex
	writer   io.Writer = os.Stderr
	jsonFlag bool
	// guarded by mu; swapped rather than mutated so readers never see a
	// half-configured handler
	current *slog.Logger = newLogger()
)

func newLogger() *slog.Logger {
	options := &slog.HandlerOptions{Level: slog.LevelWarn}
	if jsonFlag {
		return slog.New(slog.NewJSONHandler(writer, options))
	}
	return slog.New(slog.NewTextHandler(writer, options))
}

// SetJSON switches the record format. The CLI enables it with --verbose so a
// machine can read what a human asked to see more of.
func SetJSON(value bool) {
	mu.Lock()
	defer mu.Unlock()
	jsonFlag = value
	current = newLogger()
}

// SetWriter redirects diagnostics, primarily so tests can assert on records.
func SetWriter(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	writer = w
	current = newLogger()
}

// Warn reports something that did not stop the run.
func Warn(msg string, args ...any) {
	logger().Warn(msg, args...)
}

// Error reports something that stopped part of the run from succeeding.
func Error(msg string, args ...any) {
	logger().Error(msg, args...)
}

func logger() *slog.Logger {
	mu.Lock()
	defer mu.Unlock()
	return current
}

package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// capture points diagnostics at a buffer for one test and restores the package
// defaults afterwards.
func capture(t *testing.T, asJSON bool) *bytes.Buffer {
	t.Helper()

	mu.Lock()
	previousWriter, previousJSON := writer, jsonFlag
	mu.Unlock()

	buf := &bytes.Buffer{}
	SetWriter(buf)
	SetJSON(asJSON)
	t.Cleanup(func() {
		SetWriter(previousWriter)
		SetJSON(previousJSON)
	})
	return buf
}

func TestWarnTextRecord(t *testing.T) {
	buf := capture(t, false)

	Warn("path could not be read", "path", "blocked", "err", "permission denied")

	out := buf.String()
	for _, want := range []string{"level=WARN", "msg=\"path could not be read\"", "path=blocked", "err=\"permission denied\""} {
		if !strings.Contains(out, want) {
			t.Fatalf("record missing %q:\n%s", want, out)
		}
	}
}

func TestErrorRecordIsJSON(t *testing.T) {
	buf := capture(t, true)

	Error("cache write failed", "stack", "gleam", "attempt", 2)

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &record); err != nil {
		t.Fatalf("not valid JSON (%v):\n%s", err, buf.String())
	}
	if record["level"] != "ERROR" || record["msg"] != "cache write failed" {
		t.Fatalf("unexpected record: %#v", record)
	}
	if record["stack"] != "gleam" {
		t.Fatalf("key-value context lost: %#v", record)
	}
}

func TestConcurrentRecordsAreNotTorn(t *testing.T) {
	buf := capture(t, true)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			Warn("concurrent", "worker", i)
		}(i)
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 20 {
		t.Fatalf("expected 20 records, got %d:\n%s", len(lines), buf.String())
	}
	for _, line := range lines {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("torn record (%v): %q", err, line)
		}
	}
}

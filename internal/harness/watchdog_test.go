package harness

import (
	"bytes"
	"testing"
	"time"
)

func TestWatchdog_TracksIdleTime(t *testing.T) {
	var buf bytes.Buffer
	w := NewWatchdog(&buf)

	// Immediately after creation, idle time should be near zero
	if idle := w.IdleSince(); idle > 100*time.Millisecond {
		t.Errorf("initial idle = %v, want near zero", idle)
	}

	// Wait a bit, check idle grows
	time.Sleep(50 * time.Millisecond)
	if idle := w.IdleSince(); idle < 40*time.Millisecond {
		t.Errorf("idle after sleep = %v, want >= 40ms", idle)
	}

	// Write resets idle
	w.Write([]byte("hello"))
	if idle := w.IdleSince(); idle > 10*time.Millisecond {
		t.Errorf("idle after write = %v, want near zero", idle)
	}

	// Underlying writer received the data
	if buf.String() != "hello" {
		t.Errorf("buf = %q, want %q", buf.String(), "hello")
	}
}

func TestWatchdog_LastWrite(t *testing.T) {
	var buf bytes.Buffer
	w := NewWatchdog(&buf)

	before := time.Now()
	time.Sleep(10 * time.Millisecond)
	w.Write([]byte("x"))
	after := time.Now()

	lw := w.LastWrite()
	if lw.Before(before) || lw.After(after) {
		t.Errorf("LastWrite = %v, want between %v and %v", lw, before, after)
	}
}

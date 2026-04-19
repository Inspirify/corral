package harness

import (
	"io"
	"sync"
	"time"
)

// Watchdog wraps an io.Writer to track output activity and detect idle agents.
type Watchdog struct {
	inner     io.Writer
	mu        sync.Mutex
	lastWrite time.Time
}

// NewWatchdog wraps a writer with idle-time tracking.
func NewWatchdog(w io.Writer) *Watchdog {
	return &Watchdog{
		inner:     w,
		lastWrite: time.Now(),
	}
}

// Write implements io.Writer. Records the timestamp of each write.
func (w *Watchdog) Write(p []byte) (n int, err error) {
	n, err = w.inner.Write(p)
	if n > 0 {
		w.mu.Lock()
		w.lastWrite = time.Now()
		w.mu.Unlock()
	}
	return n, err
}

// IdleSince returns the duration since the last write.
func (w *Watchdog) IdleSince() time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()
	return time.Since(w.lastWrite)
}

// LastWrite returns the last write timestamp.
func (w *Watchdog) LastWrite() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastWrite
}

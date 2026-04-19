package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// Lock provides file-based single-instance locking with PID tracking.
type Lock struct {
	path string
}

// NewLock creates a lock for the given agent in the log directory.
func NewLock(logDir, agentName string) *Lock {
	return &Lock{
		path: filepath.Join(logDir, agentName+".lock"),
	}
}

// Acquire attempts to take the lock. Returns nil if acquired.
// If another process holds the lock and is alive, returns an error.
// If the holding process is dead, steals the lock.
func (l *Lock) Acquire() error {
	if err := os.MkdirAll(filepath.Dir(l.path), 0755); err != nil {
		return fmt.Errorf("creating lock directory: %w", err)
	}

	// Check existing lock
	data, err := os.ReadFile(l.path)
	if err == nil {
		pid, err := strconv.Atoi(string(data))
		if err == nil && processAlive(pid) {
			return fmt.Errorf("agent already running (pid %d)", pid)
		}
		// Stale lock — steal it
	}

	// Write our PID
	return os.WriteFile(l.path, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// Release removes the lock file.
func (l *Lock) Release() error {
	return os.Remove(l.path)
}

// processAlive checks if a process with the given PID is running.
func processAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence without actually signaling
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

package harness

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestLock_AcquireRelease(t *testing.T) {
	dir := t.TempDir()
	lock := NewLock(dir, "test-agent")

	// Acquire should succeed
	if err := lock.Acquire(); err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	// Lock file should exist with our PID
	data, err := os.ReadFile(filepath.Join(dir, "test-agent.lock"))
	if err != nil {
		t.Fatalf("reading lock file: %v", err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatalf("parsing PID: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("lock PID = %d, want %d", pid, os.Getpid())
	}

	// Release should remove the file
	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "test-agent.lock")); !os.IsNotExist(err) {
		t.Error("lock file should be removed after release")
	}
}

func TestLock_StaleLockIsStolen(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test-agent.lock")

	// Write a lock with a PID that doesn't exist (use a very high PID)
	os.WriteFile(lockPath, []byte("9999999"), 0644)

	lock := NewLock(dir, "test-agent")
	if err := lock.Acquire(); err != nil {
		t.Fatalf("Acquire() should steal stale lock, got: %v", err)
	}

	// Verify our PID is now in the lock
	data, _ := os.ReadFile(lockPath)
	pid, _ := strconv.Atoi(string(data))
	if pid != os.Getpid() {
		t.Errorf("lock PID = %d, want %d (our process)", pid, os.Getpid())
	}

	lock.Release()
}

func TestLock_LiveProcessBlocks(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test-agent.lock")

	// Write a lock with our own PID (definitely alive)
	os.WriteFile(lockPath, []byte(strconv.Itoa(os.Getpid())), 0644)

	lock := NewLock(dir, "test-agent")
	err := lock.Acquire()
	if err == nil {
		t.Fatal("Acquire() should fail when lock held by live process")
	}
	if !contains(err.Error(), "already running") {
		t.Errorf("error = %q, want to contain 'already running'", err)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

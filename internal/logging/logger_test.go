package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRunLog_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	rl, err := NewRunLog(dir, "test-agent")
	if err != nil {
		t.Fatalf("NewRunLog() error: %v", err)
	}
	defer rl.Close()

	// Path should be under {dir}/test-agent/
	if !strings.HasPrefix(rl.Path(), filepath.Join(dir, "test-agent")) {
		t.Errorf("path = %q, want prefix %q", rl.Path(), filepath.Join(dir, "test-agent"))
	}

	// Path should end in .log
	if !strings.HasSuffix(rl.Path(), ".log") {
		t.Errorf("path = %q, want .log suffix", rl.Path())
	}

	// File should exist
	if _, err := os.Stat(rl.Path()); err != nil {
		t.Errorf("log file should exist: %v", err)
	}
}

func TestNewRunLog_WriterWorks(t *testing.T) {
	dir := t.TempDir()
	rl, err := NewRunLog(dir, "writer-test")
	if err != nil {
		t.Fatalf("NewRunLog() error: %v", err)
	}

	_, err = rl.Writer().Write([]byte("hello world\n"))
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	rl.Close()

	data, err := os.ReadFile(rl.Path())
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("log content = %q, want %q", string(data), "hello world\n")
	}
}

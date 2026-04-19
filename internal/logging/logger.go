// Package logging provides per-agent, per-run log file management.
package logging

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// RunLog manages logging for a single agent run.
type RunLog struct {
	path string
	file *os.File
}

// NewRunLog creates a log file at {logDir}/{agentName}/{timestamp}.log.
func NewRunLog(logDir, agentName string) (*RunLog, error) {
	dir := filepath.Join(logDir, agentName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	ts := time.Now().Format("2006-01-02-150405")
	path := filepath.Join(dir, ts+".log")
	f, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("creating log file: %w", err)
	}

	return &RunLog{path: path, file: f}, nil
}

// Path returns the log file path.
func (l *RunLog) Path() string {
	return l.path
}

// Writer returns an io.Writer that writes to the log file.
func (l *RunLog) Writer() io.Writer {
	return l.file
}

// Close closes the log file.
func (l *RunLog) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

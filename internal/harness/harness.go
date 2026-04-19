// Package harness provides the agent execution lifecycle: lock, exec, watchdog, continuation.
package harness

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Inspirify/corral/internal/config"
	"github.com/Inspirify/corral/internal/logging"
)

// Harness manages the lifecycle of a single agent execution.
type Harness struct {
	name  string
	agent config.AgentConfig
}

// New creates a Harness for the given agent.
func New(name string, agent config.AgentConfig) *Harness {
	return &Harness{name: name, agent: agent}
}

// Run executes the agent with the full harness pipeline:
// lock → exec → watchdog → continuation → cleanup.
func (h *Harness) Run(ctx context.Context) error {
	// 1. Acquire lock if enabled
	var lock *Lock
	if h.agent.Lock() {
		logDir := h.agent.LogDir
		if logDir == "" {
			logDir = "."
		}
		lock = NewLock(logDir, h.name)
		if err := lock.Acquire(); err != nil {
			return fmt.Errorf("lock: %w", err)
		}
		defer lock.Release()
	}

	// 2. Create log file
	logDir := h.agent.LogDir
	if logDir == "" {
		logDir = "."
	}
	runLog, err := logging.NewRunLog(logDir, h.name)
	if err != nil {
		return fmt.Errorf("logging: %w", err)
	}
	defer runLog.Close()

	fmt.Printf("[corral] agent=%s log=%s\n", h.name, runLog.Path())

	// 3. Run with continuation loop
	maxCont := h.agent.MaxContinuations
	if maxCont <= 0 {
		maxCont = 1 // at least one run
	}

	for attempt := 0; attempt < maxCont; attempt++ {
		if attempt > 0 {
			fmt.Printf("[corral] agent=%s continuation=%d/%d\n", h.name, attempt+1, maxCont)
		}

		done, err := h.execOnce(ctx, runLog.Writer())
		if err != nil {
			return fmt.Errorf("run %d: %w", attempt+1, err)
		}
		if done {
			fmt.Printf("[corral] agent=%s done_signal detected\n", h.name)
			return nil
		}
	}

	fmt.Printf("[corral] agent=%s max continuations (%d) reached\n", h.name, maxCont)
	return nil
}

// execOnce runs the command once and monitors it with the watchdog.
// Returns (true, nil) if the done signal was detected.
func (h *Harness) execOnce(ctx context.Context, logWriter io.Writer) (bool, error) {
	cmdStr := h.agent.Command()
	cmd := exec.CommandContext(ctx, "sh", "-c", cmdStr)

	if h.agent.WorkingDir != "" {
		cmd.Dir = h.agent.WorkingDir
	}

	// Build environment
	cmd.Env = os.Environ()
	for k, v := range h.agent.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	// Pipe stdout and stderr through the watchdog
	watchdog := NewWatchdog(io.MultiWriter(logWriter, os.Stdout))
	stderrWatchdog := NewWatchdog(io.MultiWriter(logWriter, os.Stderr))

	// Use pipes for done signal scanning
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return false, fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return false, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return false, fmt.Errorf("start: %w", err)
	}

	// Scan output for done signal
	doneSignal := h.agent.DoneSignal
	doneCh := make(chan bool, 1)

	// Copy stderr in background
	go func() {
		io.Copy(stderrWatchdog, stderrPipe)
	}()

	// Scan stdout for done signal while copying
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		found := false
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintln(watchdog, line)
			if doneSignal != "" && strings.Contains(line, doneSignal) {
				found = true
			}
		}
		doneCh <- found
	}()

	// Set up idle timeout and max runtime watchdogs
	idleTimeout := h.agent.IdleTimeout.Duration
	maxRuntime := h.agent.MaxRuntime.Duration

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	started := time.Now()
	killed := false

	go func() {
		for range ticker.C {
			if maxRuntime > 0 && time.Since(started) > maxRuntime {
				fmt.Printf("[corral] agent=%s max_runtime exceeded, killing\n", h.name)
				cmd.Process.Signal(os.Interrupt)
				killed = true
				return
			}
			if idleTimeout > 0 && watchdog.IdleSince() > idleTimeout {
				fmt.Printf("[corral] agent=%s idle_timeout exceeded (%v idle), killing\n",
					h.name, watchdog.IdleSince().Round(time.Second))
				cmd.Process.Signal(os.Interrupt)
				killed = true
				return
			}
		}
	}()

	// Wait for output scanning to finish
	foundDone := <-doneCh

	// Wait for process
	err = cmd.Wait()
	if killed {
		return false, nil // timeout-killed is not an error for continuation
	}
	if err != nil {
		return false, fmt.Errorf("process exited: %w", err)
	}

	return foundDone, nil
}

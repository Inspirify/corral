package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLaunchdPlist_Render(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd only on macOS")
	}

	opts := Options{
		Label:      "com.corral.scheduler",
		BinaryPath: "/usr/local/bin/corral",
		ConfigPath: "/Users/test/.config/corral/corral.toml",
		LogDir:     "/Users/test/.config/corral/logs",
	}

	content, err := RenderLaunchd(opts)
	if err != nil {
		t.Fatalf("RenderLaunchd() error: %v", err)
	}

	// Should contain the label
	if !strings.Contains(content, "com.corral.scheduler") {
		t.Error("plist missing label")
	}
	// Should contain binary path
	if !strings.Contains(content, "/usr/local/bin/corral") {
		t.Error("plist missing binary path")
	}
	// Should contain config path
	if !strings.Contains(content, "/Users/test/.config/corral/corral.toml") {
		t.Error("plist missing config path")
	}
	// Should contain the start command
	if !strings.Contains(content, "start") {
		t.Error("plist missing 'start' argument")
	}
	// Should have RunAtLoad
	if !strings.Contains(content, "RunAtLoad") {
		t.Error("plist missing RunAtLoad")
	}
}

func TestInstallPath_Launchd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd only on macOS")
	}

	path := LaunchdInstallPath("com.corral.scheduler")
	if !strings.Contains(path, "LaunchAgents") {
		t.Errorf("path = %q, want to contain LaunchAgents", path)
	}
	if !strings.HasSuffix(path, ".plist") {
		t.Errorf("path = %q, want .plist suffix", path)
	}
}

func TestInstall_Launchd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd only on macOS")
	}

	dir := t.TempDir()
	plistPath := filepath.Join(dir, "com.corral.test.plist")

	opts := Options{
		Label:       "com.corral.test",
		BinaryPath:  "/usr/local/bin/corral",
		ConfigPath:  "/tmp/corral.toml",
		LogDir:      "/tmp/logs",
		InstallPath: plistPath, // override for testing
	}

	if err := Install(opts); err != nil {
		t.Fatalf("Install() error: %v", err)
	}

	// Verify file was written
	data, err := os.ReadFile(plistPath)
	if err != nil {
		t.Fatalf("reading plist: %v", err)
	}
	if !strings.Contains(string(data), "com.corral.test") {
		t.Error("plist missing label")
	}
}

func TestUninstall_Launchd(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd only on macOS")
	}

	dir := t.TempDir()
	plistPath := filepath.Join(dir, "com.corral.test.plist")
	os.WriteFile(plistPath, []byte("fake plist"), 0644)

	opts := Options{
		Label:       "com.corral.test",
		InstallPath: plistPath,
	}

	if err := Uninstall(opts); err != nil {
		t.Fatalf("Uninstall() error: %v", err)
	}

	// File should be removed
	if _, err := os.Stat(plistPath); !os.IsNotExist(err) {
		t.Error("plist should be removed after uninstall")
	}
}

func TestUninstall_MissingFile(t *testing.T) {
	opts := Options{
		Label:       "com.corral.nonexistent",
		InstallPath: "/tmp/does-not-exist-corral-test.plist",
	}

	err := Uninstall(opts)
	if err == nil {
		t.Fatal("expected error when plist doesn't exist")
	}
}

func TestIsProcessAlive_Self(t *testing.T) {
	// Our own process should be alive.
	if !isProcessAlive(os.Getpid()) {
		t.Fatal("expected own process to be alive")
	}
}

func TestIsProcessAlive_Dead(t *testing.T) {
	// PID 0 is never a valid user process; signal(0) will fail.
	// Use a very high PID that almost certainly doesn't exist.
	if isProcessAlive(4194304) {
		t.Fatal("expected non-existent PID to be dead")
	}
}

func TestWaitForExit_AlreadyDead(t *testing.T) {
	// Start a process and let it exit, then waitForExit should return immediately.
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting process: %v", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Wait() // let it finish

	start := time.Now()
	err := waitForExit(pid, 5*time.Second)
	if err != nil {
		t.Fatalf("waitForExit() error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("waitForExit took %v, expected near-instant for dead process", elapsed)
	}
}

func TestWaitForExit_Timeout(t *testing.T) {
	// Start a long-running process, give a tiny timeout.
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting process: %v", err)
	}
	pid := cmd.Process.Pid
	defer cmd.Process.Kill()

	err := waitForExit(pid, 500*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "did not exit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopProcess_Graceful(t *testing.T) {
	// Start a sleep process and gracefully stop it.
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting process: %v", err)
	}
	pid := cmd.Process.Pid

	// Reap the child in the background to prevent zombie state,
	// which would cause isProcessAlive to return true indefinitely.
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	start := time.Now()
	err := stopProcess(pid, false)
	if err != nil {
		t.Fatalf("stopProcess() error: %v", err)
	}
	// Should exit quickly (SIGTERM kills sleep immediately)
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Fatalf("stopProcess took %v, expected fast exit", elapsed)
	}

	// Confirm the child was actually reaped
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("process was not reaped after stop")
	}
}

func TestStopProcess_Force(t *testing.T) {
	// Start a sleep process and force-kill it.
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting process: %v", err)
	}
	pid := cmd.Process.Pid

	// Reap the child to prevent zombie
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	err := stopProcess(pid, true)
	if err != nil {
		t.Fatalf("stopProcess(force) error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("process was not reaped after force stop")
	}
}

func TestStopScheduler_NothingRunning(t *testing.T) {
	// Skip if there's actually a corral scheduler running on this machine.
	if _, found := FindRunningProcess(); found {
		t.Skip("corral scheduler is running, cannot test 'nothing running' case")
	}

	err := StopScheduler("com.corral.nonexistent.test."+t.Name(), false)
	if err == nil {
		t.Fatal("expected error when no scheduler is running")
	}
	if !strings.Contains(err.Error(), "no running scheduler") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopLaunchdService_NotInstalled(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("launchd only on macOS")
	}
	// Skip if there's actually a corral scheduler running.
	if _, found := FindRunningProcess(); found {
		t.Skip("corral scheduler is running, cannot test 'not installed' case")
	}

	err := StopScheduler("com.corral.test.nonexistent."+t.Name(), false)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "no running scheduler") {
		t.Fatalf("unexpected error: %v", err)
	}
}

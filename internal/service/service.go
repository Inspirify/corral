// Package service manages OS service installation (launchd on macOS, systemd on Linux).
package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"text/template"
	"time"
)

// Options configures service installation.
type Options struct {
	Label       string            // e.g., "com.corral.scheduler"
	BinaryPath  string            // absolute path to corral binary
	ConfigPath  string            // absolute path to corral.toml
	LogDir      string            // log directory
	EnvVars     map[string]string // environment variables to set in the service
	InstallPath string            // override install path (for testing)
}

const launchdTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.BinaryPath}}</string>
		<string>start</string>
		<string>--config</string>
		<string>{{.ConfigPath}}</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>{{.LogDir}}/corral-scheduler.out.log</string>
	<key>StandardErrorPath</key>
	<string>{{.LogDir}}/corral-scheduler.err.log</string>
	<key>WorkingDirectory</key>
	<string>{{dir .ConfigPath}}</string>{{if .EnvVars}}
	<key>EnvironmentVariables</key>
	<dict>{{range $k, $v := .EnvVars}}
		<key>{{$k}}</key>
		<string>{{$v}}</string>{{end}}
	</dict>{{end}}
</dict>
</plist>
`

var launchdTmpl = template.Must(template.New("launchd").Funcs(template.FuncMap{
	"dir": filepath.Dir,
}).Parse(launchdTemplate))

// RenderLaunchd generates a launchd plist from the options.
func RenderLaunchd(opts Options) (string, error) {
	var buf bytes.Buffer
	if err := launchdTmpl.Execute(&buf, opts); err != nil {
		return "", fmt.Errorf("rendering plist: %w", err)
	}
	return buf.String(), nil
}

// LaunchdInstallPath returns the default install path for a launchd agent.
func LaunchdInstallPath(label string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", label+".plist")
}

// Install writes the service file and loads it.
func Install(opts Options) error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("service install only supported on macOS (launchd) currently")
	}

	content, err := RenderLaunchd(opts)
	if err != nil {
		return err
	}

	path := opts.InstallPath
	if path == "" {
		path = LaunchdInstallPath(opts.Label)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing plist: %w", err)
	}

	// Don't run launchctl if using a custom install path (testing)
	if opts.InstallPath == "" {
		cmd := exec.Command("launchctl", "load", path)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl load: %s: %w", string(out), err)
		}
	}

	return nil
}

// Status checks whether a launchd service is installed and running.
// Returns (installed, running, pid, error).
func Status(label string) (installed bool, running bool, pid int, err error) {
	if runtime.GOOS != "darwin" {
		return false, false, 0, fmt.Errorf("service status only supported on macOS currently")
	}

	path := LaunchdInstallPath(label)
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return false, false, 0, nil
	}
	installed = true

	// launchctl list <label> exits 0 if the service is loaded (even if not running).
	cmd := exec.Command("launchctl", "list", label)
	out, listErr := cmd.CombinedOutput()
	if listErr != nil {
		// Not loaded
		return installed, false, 0, nil
	}

	// Parse PID from output. Format:
	// {
	//     "PID" = 12345;
	//     ...
	// }
	// Or "PID" might be missing if the service isn't currently running a process.
	for _, line := range bytes.Split(out, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if bytes.HasPrefix(line, []byte(`"PID"`)) {
			var p int
			if _, err := fmt.Sscanf(string(line), `"PID" = %d;`, &p); err == nil && p > 0 {
				return installed, true, p, nil
			}
		}
	}

	// Loaded but no PID — waiting for KeepAlive or RunAtLoad
	return installed, false, 0, nil
}

// FindRunningProcess looks for a running "corral start" process.
// Returns (pid, found). This detects direct invocations that bypass launchd/systemd.
func FindRunningProcess() (int, bool) {
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		return 0, false
	}
	// pgrep -f matches against the full command line.
	cmd := exec.Command("pgrep", "-f", "corral start")
	out, err := cmd.Output()
	if err != nil {
		return 0, false
	}
	myPID := os.Getpid()
	for _, line := range bytes.Split(bytes.TrimSpace(out), []byte("\n")) {
		var p int
		if _, err := fmt.Sscanf(string(line), "%d", &p); err == nil && p > 0 && p != myPID {
			return p, true
		}
	}
	return 0, false
}

// StopScheduler finds and stops the running scheduler process.
// It checks for a launchd-managed service first, then falls back
// to finding a direct "corral start" process.
func StopScheduler(label string, force bool) error {
	// Check for launchd service first (macOS)
	if runtime.GOOS == "darwin" {
		installed, running, pid, err := Status(label)
		if err == nil && installed && running {
			return stopLaunchdService(label, pid, force)
		}
	}

	// Fall back to direct process lookup
	pid, found := FindRunningProcess()
	if !found {
		return fmt.Errorf("no running scheduler found")
	}
	return stopProcess(pid, force)
}

// stopLaunchdService stops a launchd-managed scheduler by unloading
// the service plist. This prevents KeepAlive from restarting the process.
// The plist file is preserved on disk so "corral install" can reload it.
func stopLaunchdService(label string, pid int, force bool) error {
	if force {
		// Send SIGKILL directly before unloading
		if p, err := os.FindProcess(pid); err == nil {
			_ = p.Signal(syscall.SIGKILL)
		}
	}

	// Unload the service — stops the process and prevents KeepAlive restart.
	// The plist remains on disk (unlike Uninstall which deletes it).
	path := LaunchdInstallPath(label)
	cmd := exec.Command("launchctl", "unload", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl unload: %s: %w", string(bytes.TrimSpace(out)), err)
	}

	return waitForExit(pid, 30*time.Second)
}

// stopProcess sends a signal to a directly-invoked scheduler process.
// Without --force: SIGTERM → wait 30s → SIGKILL escalation.
// With --force: SIGKILL immediately.
func stopProcess(pid int, force bool) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}

	if force {
		if err := p.Signal(syscall.SIGKILL); err != nil {
			return fmt.Errorf("killing process %d: %w", pid, err)
		}
		return waitForExit(pid, 5*time.Second)
	}

	// Graceful: SIGTERM first
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("signaling process %d: %w", pid, err)
	}

	// Wait for graceful shutdown
	if err := waitForExit(pid, 30*time.Second); err == nil {
		return nil
	}

	// Escalate to SIGKILL
	fmt.Printf("Process %d did not exit gracefully, sending SIGKILL...\n", pid)
	if err := p.Signal(syscall.SIGKILL); err != nil {
		// Process may have exited between the timeout and now
		if !isProcessAlive(pid) {
			return nil
		}
		return fmt.Errorf("killing process %d: %w", pid, err)
	}
	return waitForExit(pid, 5*time.Second)
}

// waitForExit polls until the process is no longer alive or the timeout expires.
func waitForExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return fmt.Errorf("process %d did not exit within %v", pid, timeout)
}

// isProcessAlive checks whether a process with the given PID exists.
func isProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 checks existence without actually signaling.
	return p.Signal(syscall.Signal(0)) == nil
}

// Uninstall removes the service file and unloads it.
func Uninstall(opts Options) error {
	path := opts.InstallPath
	if path == "" {
		path = LaunchdInstallPath(opts.Label)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("service file not found: %s", path)
	}

	// Don't run launchctl if using a custom install path (testing)
	if opts.InstallPath == "" {
		cmd := exec.Command("launchctl", "unload", path)
		_, _ = cmd.CombinedOutput() // best-effort
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing plist: %w", err)
	}

	return nil
}

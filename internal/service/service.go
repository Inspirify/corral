// Package service manages OS service installation (launchd on macOS, systemd on Linux).
package service

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"
)

// Options configures service installation.
type Options struct {
	Label       string // e.g., "com.corral.scheduler"
	BinaryPath  string // absolute path to corral binary
	ConfigPath  string // absolute path to corral.toml
	LogDir      string // log directory
	InstallPath string // override install path (for testing)
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
	<string>{{dir .ConfigPath}}</string>
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
		cmd.CombinedOutput() // best-effort
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing plist: %w", err)
	}

	return nil
}

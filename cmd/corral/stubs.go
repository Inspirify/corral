package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Inspirify/corral/internal/config"
	"github.com/Inspirify/corral/internal/scheduler"
	"github.com/Inspirify/corral/internal/service"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the scheduler (foreground)",
		Long:  "Start the cron scheduler in the foreground. Use 'corral install' for background service.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			s := scheduler.New(cfg)
			return s.Start(cmd.Context())
		},
	}
}

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the scheduler daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println("Not yet implemented — use Ctrl-C or launchctl/systemctl")
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "status [agent]",
		Short: "Show service and agent status",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			// Show service status
			installed, running, pid, svcErr := service.Status(label)
			switch {
			case svcErr != nil:
				fmt.Printf("Service: unknown (%v)\n", svcErr)
			case !installed:
				fmt.Printf("Service: not installed (run 'corral install')\n")
			case running:
				fmt.Printf("Service: running (pid %d)\n", pid)
			default:
				fmt.Printf("Service: installed but not running\n")
			}
			fmt.Println()

			if len(args) == 1 {
				return showAgentStatus(cfg, args[0])
			}

			// Show all agents
			for name, agent := range cfg.Agents {
				schedule := agent.Schedule()
				if schedule == "" {
					schedule = "(manual)"
				}
				logDir := agent.LogDir
				if logDir == "" {
					logDir = "."
				}
				lastLog := findLastLog(logDir, name)
				fmt.Printf("%-20s schedule=%-20s last_log=%s\n", name, schedule, lastLog)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "com.corral.scheduler", "service label")
	return cmd
}

func newLogsCmd() *cobra.Command {
	var tail int
	cmd := &cobra.Command{
		Use:   "logs <agent>",
		Short: "Show agent logs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}
			agent, ok := cfg.Agents[args[0]]
			if !ok {
				return fmt.Errorf("agent %q not found", args[0])
			}
			logDir := agent.LogDir
			if logDir == "" {
				logDir = "."
			}
			logPath := findLastLog(logDir, args[0])
			if logPath == "(none)" {
				fmt.Println("No logs found")
				return nil
			}
			return tailFile(logPath, tail)
		},
	}
	cmd.Flags().IntVarP(&tail, "tail", "n", 50, "number of lines to show")
	return cmd
}

func newInstallCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install corral as a system service (launchd/systemd)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgFile)
			if err != nil {
				return err
			}

			binaryPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("finding binary path: %w", err)
			}

			configPath := cfgFile
			if configPath == "" {
				configPath = "corral.toml"
			}
			configPath, _ = filepath.Abs(configPath)

			logDir := cfg.Defaults.LogDir
			if logDir == "" {
				logDir = filepath.Join(filepath.Dir(configPath), "logs")
			}

			opts := service.Options{
				Label:      label,
				BinaryPath: binaryPath,
				ConfigPath: configPath,
				LogDir:     logDir,
				EnvVars:    collectEnvRefs(configPath),
			}

			if err := service.Install(opts); err != nil {
				return err
			}
			fmt.Printf("Service installed: %s\n", label)
			fmt.Printf("Plist: %s\n", service.LaunchdInstallPath(label))
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "com.corral.scheduler", "service label")
	return cmd
}

func newUninstallCmd() *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the system service",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := service.Options{
				Label: label,
			}
			if err := service.Uninstall(opts); err != nil {
				return err
			}
			fmt.Printf("Service removed: %s\n", label)
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "com.corral.scheduler", "service label")
	return cmd
}

// findLastLog returns the path of the most recent log file for an agent.
func findLastLog(logDir, agentName string) string {
	dir := filepath.Join(logDir, agentName)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "(none)"
	}
	var logs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			logs = append(logs, e.Name())
		}
	}
	if len(logs) == 0 {
		return "(none)"
	}
	sort.Strings(logs)
	return filepath.Join(dir, logs[len(logs)-1])
}

// showAgentStatus shows detailed status for a single agent.
func showAgentStatus(cfg *config.Config, name string) error {
	agent, ok := cfg.Agents[name]
	if !ok {
		return fmt.Errorf("agent %q not found", name)
	}
	fmt.Printf("Agent: %s\n", name)
	fmt.Printf("  Command:     %s\n", agent.Command())
	fmt.Printf("  Schedule:    %s\n", agent.Schedule())
	fmt.Printf("  MaxRuntime:  %s\n", agent.MaxRuntime)
	fmt.Printf("  IdleTimeout: %s\n", agent.IdleTimeout)
	fmt.Printf("  Lock:        %v\n", agent.Lock())

	logDir := agent.LogDir
	if logDir == "" {
		logDir = "."
	}
	lastLog := findLastLog(logDir, name)
	fmt.Printf("  LastLog:     %s\n", lastLog)
	return nil
}

// tailFile prints the last n lines of a file.
func tailFile(path string, n int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	for _, line := range lines[start:] {
		fmt.Println(line)
	}
	return nil
}

// collectEnvRefs scans a config file for ${VAR} references and returns
// a map of VAR→value from the current process environment.
// Unset vars are silently skipped.
func collectEnvRefs(configPath string) map[string]string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}
	envPattern := regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)
	matches := envPattern.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		return nil
	}
	result := make(map[string]string)
	for _, m := range matches {
		name := string(m[1])
		if val, ok := os.LookupEnv(name); ok {
			result[name] = val
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

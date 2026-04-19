package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// binaryPath returns the path to the compiled corral binary.
// Tests call TestMain to build it once before running.
var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all e2e tests
	tmp, err := os.MkdirTemp("", "corral-e2e-*")
	if err != nil {
		panic("creating temp dir: " + err.Error())
	}
	binaryPath = filepath.Join(tmp, "corral")
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/corral")
	cmd.Dir = findProjectRoot()
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("building corral binary: " + err.Error())
	}

	code := m.Run()

	os.RemoveAll(tmp)
	os.Exit(code)
}

// findProjectRoot walks up from the current dir to find go.mod.
func findProjectRoot() string {
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			panic("could not find project root (go.mod)")
		}
		dir = parent
	}
}

// runCorral executes the corral binary with the given args in the given dir.
func runCorral(t *testing.T, dir string, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = dir
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("running corral: %v", err)
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

// --- Tests ---

func TestInit_CreatesConfigFile(t *testing.T) {
	dir := t.TempDir()

	stdout, _, code := runCorral(t, dir, "init")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Created corral.toml") {
		t.Errorf("stdout = %q, want to contain 'Created corral.toml'", stdout)
	}

	// Verify file exists and is valid TOML with expected content
	data, err := os.ReadFile(filepath.Join(dir, "corral.toml"))
	if err != nil {
		t.Fatalf("reading corral.toml: %v", err)
	}
	if !strings.Contains(string(data), "[defaults]") {
		t.Error("corral.toml missing [defaults] section")
	}
}

func TestInit_RefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte("existing"), 0644)

	_, stderr, code := runCorral(t, dir, "init")
	if code == 0 {
		t.Fatal("expected non-zero exit code when corral.toml exists")
	}
	combined := stderr // cobra prints errors to stderr
	if !strings.Contains(combined, "already exists") {
		t.Errorf("output = %q, want to contain 'already exists'", combined)
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[agents.echo]
command = "echo hello"
`), 0644)

	stdout, _, code := runCorral(t, dir, "validate")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "1 agent(s) configured") {
		t.Errorf("stdout = %q, want agent count", stdout)
	}
	if !strings.Contains(stdout, "echo") {
		t.Errorf("stdout = %q, want agent name 'echo'", stdout)
	}
}

func TestValidate_InvalidConfig(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[agents.bad]
schedule = "* * * * *"
`), 0644)

	_, _, code := runCorral(t, dir, "validate")
	if code == 0 {
		t.Fatal("expected non-zero exit for agent missing command")
	}
}

func TestValidate_MissingConfig(t *testing.T) {
	dir := t.TempDir()

	_, _, code := runCorral(t, dir, "validate")
	if code == 0 {
		t.Fatal("expected non-zero exit when corral.toml is missing")
	}
}

func TestList_ShowsAgents(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[agents.dev]
command = "claude -p 'do stuff'"
schedule = "*/30 * * * *"

[agents.cpo]
command = "claude -p 'cpo scan'"
`), 0644)

	stdout, _, code := runCorral(t, dir, "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "dev") {
		t.Errorf("stdout missing 'dev': %q", stdout)
	}
	if !strings.Contains(stdout, "cpo") {
		t.Errorf("stdout missing 'cpo': %q", stdout)
	}
	if !strings.Contains(stdout, "NAME") {
		t.Errorf("stdout missing header 'NAME': %q", stdout)
	}
}

func TestList_NoAgents(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
idle_timeout = "5m"
`), 0644)

	stdout, _, code := runCorral(t, dir, "list")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "No agents") {
		t.Errorf("stdout = %q, want 'No agents'", stdout)
	}
}

func TestRun_ExecutesAgent(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
log_dir = "`+logDir+`"
lock = false

[agents.echo]
command = "echo HELLO_CORRAL"
`), 0644)

	stdout, _, code := runCorral(t, dir, "run", "echo")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "HELLO_CORRAL") {
		t.Errorf("stdout = %q, want to contain 'HELLO_CORRAL'", stdout)
	}

	// Verify log file was created
	entries, err := os.ReadDir(filepath.Join(logDir, "echo"))
	if err != nil {
		t.Fatalf("reading log dir: %v", err)
	}
	if len(entries) == 0 {
		t.Error("expected at least one log file")
	}

	// Verify log content
	logData, err := os.ReadFile(filepath.Join(logDir, "echo", entries[0].Name()))
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(logData), "HELLO_CORRAL") {
		t.Errorf("log content = %q, want to contain 'HELLO_CORRAL'", string(logData))
	}
}

func TestRun_UnknownAgent(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[agents.dev]
command = "echo hi"
`), 0644)

	_, stderr, code := runCorral(t, dir, "run", "nonexistent")
	if code == 0 {
		t.Fatal("expected non-zero exit for unknown agent")
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q, want 'not found'", stderr)
	}
}

func TestRun_DoneSignalStopsContinuation(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	// Agent that echoes done signal on first run — should NOT continue
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
log_dir = "`+logDir+`"
lock = false

[agents.done]
command = "echo AGENT_DONE"
done_signal = "AGENT_DONE"
max_continuations = 5
`), 0644)

	stdout, _, code := runCorral(t, dir, "run", "done")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "done_signal detected") {
		t.Errorf("stdout = %q, want 'done_signal detected'", stdout)
	}
	// Should not show continuation messages
	if strings.Contains(stdout, "continuation=2") {
		t.Error("should not have continued after done signal")
	}
}

func TestRun_LockPreventsParallel(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	os.MkdirAll(logDir, 0755)

	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
log_dir = "`+logDir+`"

[agents.slow]
command = "sleep 10"
lock = true
`), 0644)

	// Start first run in background
	cmd1 := exec.Command(binaryPath, "run", "slow")
	cmd1.Dir = dir
	if err := cmd1.Start(); err != nil {
		t.Fatalf("starting first run: %v", err)
	}
	defer cmd1.Process.Kill()

	// Wait for lock to be acquired
	lockPath := filepath.Join(logDir, "slow.lock")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(lockPath); err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Second run should fail with lock error
	_, stderr, code := runCorral(t, dir, "run", "slow")
	if code == 0 {
		t.Fatal("expected non-zero exit when lock is held")
	}
	if !strings.Contains(stderr, "already running") {
		t.Errorf("stderr = %q, want 'already running'", stderr)
	}
}

func TestRun_AgentFromAgentsDir(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")

	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
log_dir = "`+logDir+`"
lock = false
`), 0644)

	// Create agents/ directory with a per-agent file
	agentsDir := filepath.Join(dir, "agents")
	os.MkdirAll(agentsDir, 0755)
	os.WriteFile(filepath.Join(agentsDir, "external.toml"), []byte(`
command = "echo FROM_EXTERNAL_FILE"
description = "Loaded from agents/ dir"
`), 0644)

	stdout, _, code := runCorral(t, dir, "run", "external")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "FROM_EXTERNAL_FILE") {
		t.Errorf("stdout = %q, want 'FROM_EXTERNAL_FILE'", stdout)
	}
}

func TestRun_EnvInterpolation(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")

	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
log_dir = "`+logDir+`"
lock = false

[agents.env]
command = "echo ${CORRAL_TEST_VALUE}"
`), 0644)

	// Set env var and run
	cmd := exec.Command(binaryPath, "run", "env")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CORRAL_TEST_VALUE=INTERPOLATED_OK")
	var outBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("running corral: %v", err)
	}
	if !strings.Contains(outBuf.String(), "INTERPOLATED_OK") {
		t.Errorf("output = %q, want 'INTERPOLATED_OK'", outBuf.String())
	}
}

func TestRun_WorkingDir(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")
	workDir := filepath.Join(dir, "workdir")
	os.MkdirAll(workDir, 0755)

	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
log_dir = "`+logDir+`"
lock = false

[agents.pwd]
command = "pwd"
working_dir = "`+workDir+`"
`), 0644)

	stdout, _, code := runCorral(t, dir, "run", "pwd")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, workDir) {
		t.Errorf("stdout = %q, want working dir %q", stdout, workDir)
	}
}

func TestStatus_ShowsAgentInfo(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[agents.dev]
command = "echo hi"
schedule = "*/5 * * * *"
`), 0644)

	stdout, _, code := runCorral(t, dir, "status", "dev")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Agent: dev") {
		t.Errorf("stdout = %q, want 'Agent: dev'", stdout)
	}
	if !strings.Contains(stdout, "echo hi") {
		t.Errorf("stdout = %q, want command shown", stdout)
	}
}

func TestHelp_ShowsAllCommands(t *testing.T) {
	dir := t.TempDir()

	stdout, _, code := runCorral(t, dir, "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}

	expectedCmds := []string{"init", "run", "list", "validate", "start", "stop", "status", "logs", "install", "uninstall"}
	for _, cmd := range expectedCmds {
		if !strings.Contains(stdout, cmd) {
			t.Errorf("help missing command %q", cmd)
		}
	}
}

func TestRun_CustomEnvVarsPassed(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "logs")

	os.WriteFile(filepath.Join(dir, "corral.toml"), []byte(`
[defaults]
log_dir = "`+logDir+`"
lock = false

[defaults.env]
BASE_VAR = "from_defaults"

[agents.envtest]
command = "sh -c 'echo AGENT=$AGENT_VAR BASE=$BASE_VAR'"

[agents.envtest.env]
AGENT_VAR = "from_agent"
`), 0644)

	stdout, _, code := runCorral(t, dir, "run", "envtest")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "AGENT=from_agent") {
		t.Errorf("stdout = %q, want AGENT=from_agent", stdout)
	}
	if !strings.Contains(stdout, "BASE=from_defaults") {
		t.Errorf("stdout = %q, want BASE=from_defaults", stdout)
	}
}

func TestInit_ThenValidate_RoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Init
	_, _, code := runCorral(t, dir, "init")
	if code != 0 {
		t.Fatalf("init exit code = %d", code)
	}

	// Validate the scaffolded config — should succeed (no agents = valid, just no agents)
	// Actually, the default template has no agents defined, so validate should work
	stdout, _, code := runCorral(t, dir, "validate")
	if code != 0 {
		t.Fatalf("validate exit code = %d", code)
	}
	if !strings.Contains(stdout, "0 agent(s)") {
		t.Errorf("stdout = %q, want '0 agent(s)'", stdout)
	}
}

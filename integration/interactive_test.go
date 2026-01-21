package integration

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

type ptySession struct {
	t       *testing.T
	cmd     *exec.Cmd
	file    *os.File
	updates chan struct{}
	mu      sync.Mutex
	buf     bytes.Buffer
}

func newPtySession(t *testing.T, cmd *exec.Cmd) *ptySession {
	t.Helper()
	file, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty start: %v", err)
	}
	s := &ptySession{
		t:       t,
		cmd:     cmd,
		file:    file,
		updates: make(chan struct{}, 1),
	}
	go s.readLoop()
	return s
}

func (s *ptySession) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := s.file.Read(buf)
		if n > 0 {
			s.mu.Lock()
			s.buf.Write(buf[:n])
			s.mu.Unlock()
			select {
			case s.updates <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

func (s *ptySession) sendRaw(data string) {
	s.t.Helper()
	if _, err := s.file.Write([]byte(data)); err != nil {
		s.t.Fatalf("pty send raw: %v", err)
	}
}

func (s *ptySession) waitFor(substr string, timeout time.Duration) {
	s.t.Helper()
	deadline := time.After(timeout)
	for {
		if strings.Contains(s.output(), substr) {
			return
		}
		select {
		case <-s.updates:
		case <-deadline:
			s.t.Fatalf("timeout waiting for %q in output:\n%s", substr, s.output())
		}
	}
}

func (s *ptySession) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func (s *ptySession) close() {
	s.t.Helper()
	_ = s.file.Close()
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()
	select {
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
	case <-done:
	}
}

func TestCdPrintPathInteractive(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported")
	}

	home := t.TempDir()
	baseEnv := buildEnv(map[string]string{"HOME": home})
	repo := setupRepo(t, baseEnv)
	worktreePath := createWorktree(t, baseEnv, repo, "feature")

	cmd := exec.Command(wtBinary(), "cd", "--print-path")
	cmd.Dir = repo
	cmd.Env = baseEnv
	sess := newPtySession(t, cmd)
	defer sess.close()

	sess.waitFor("ENTER to select", 5*time.Second)
	sess.sendRaw("\r")
	sess.sendRaw("\n")
	sess.waitFor(filepath.Join(".worktrees", "feature"), 5*time.Second)

	if !strings.Contains(sess.output(), filepath.Join(".worktrees", "feature")) {
		t.Fatalf("expected output to include worktree path, output:\n%s", sess.output())
	}
	if worktreePath == "" {
		t.Fatalf("expected worktree path to be set")
	}
}

func TestCdTmuxUsesNewWindow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty not supported")
	}

	home := t.TempDir()
	baseEnv := buildEnv(map[string]string{"HOME": home})
	repo := setupRepo(t, baseEnv)
	worktreePath := createWorktree(t, baseEnv, repo, "feature")

	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "bin")
	if err := os.MkdirAll(fakeBin, 0755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	argsFile := filepath.Join(tmpDir, "tmux-args")
	fakeTmuxPath := filepath.Join(fakeBin, "tmux")
	fakeTmux := "#!/bin/sh\n" +
		"echo \"$@\" > \"$TMUX_ARGS_FILE\"\n"
	if err := os.WriteFile(fakeTmuxPath, []byte(fakeTmux), 0755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}

	cmd := exec.Command(wtBinary(), "cd", "--tmux")
	cmd.Dir = repo
	cmd.Env = mergeEnv(baseEnv, map[string]string{
		"PATH":           fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TMUX":           "1",
		"TMUX_ARGS_FILE": argsFile,
	})

	sess := newPtySession(t, cmd)
	defer sess.close()

	sess.waitFor("ENTER to select", 5*time.Second)
	sess.sendRaw("\r")
	sess.sendRaw("\n")

	args, err := waitForFile(argsFile, 5*time.Second)
	if err != nil {
		t.Fatalf("read tmux args: %v", err)
	}

	want := filepath.Join(".worktrees", "feature")
	if !strings.Contains(string(args), want) {
		t.Fatalf("expected tmux args to include %q, got %q", want, strings.TrimSpace(string(args)))
	}
	if worktreePath == "" {
		t.Fatalf("expected worktree path to be set")
	}
}

func TestShellInitMatchesScripts(t *testing.T) {
	home := t.TempDir()
	baseEnv := buildEnv(map[string]string{"HOME": home})
	repoRoot := repoRootDir(t)

	cases := []struct {
		shell string
		path  string
	}{
		{shell: "bash", path: filepath.Join(repoRoot, "shell", "wt.bash")},
		{shell: "zsh", path: filepath.Join(repoRoot, "shell", "wt.zsh")},
		{shell: "fish", path: filepath.Join(repoRoot, "shell", "wt.fish")},
	}

	for _, tc := range cases {
		out := runCmdStdout(t, baseEnv, repoRoot, wtBinary(), "shell-init", tc.shell)
		want, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("read %s: %v", tc.path, err)
		}
		if strings.TrimSpace(out) != strings.TrimSpace(string(want)) {
			t.Fatalf("shell-init %s output drifted from %s", tc.shell, tc.path)
		}
	}
}

func setupRepo(t *testing.T, env []string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	runCmdEnv(t, env, repo, "git", "init", "-b", "main")
	runCmdEnv(t, env, repo, "git", "config", "user.email", "test@example.com")
	runCmdEnv(t, env, repo, "git", "config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runCmdEnv(t, env, repo, "git", "add", "README.md")
	runCmdEnv(t, env, repo, "git", "commit", "-m", "init")
	return repo
}

func createWorktree(t *testing.T, env []string, repo, branch string) string {
	t.Helper()
	out := runCmdStdout(t, env, repo, wtBinary(), "add", branch, "--print-path")
	path := strings.TrimSpace(out)
	if path == "" {
		t.Fatalf("expected worktree path, got empty output")
	}
	return path
}

func wtBinary() string {
	name := "wt"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(wtBinDir, name)
}

func repoRootDir(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	return filepath.Dir(wd)
}

func runCmdEnv(t *testing.T, env []string, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cmd %s %s: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func runCmdStdout(t *testing.T, env []string, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("cmd %s %s: %v\n%s", name, strings.Join(args, " "), err, stderr.String())
	}
	return string(out)
}

func buildEnv(extra map[string]string) []string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	env["GIT_CONFIG_NOSYSTEM"] = "1"
	env["GIT_CONFIG_GLOBAL"] = os.DevNull
	for key, value := range extra {
		env[key] = value
	}
	return envList(env)
}

func mergeEnv(base []string, extra map[string]string) []string {
	env := envMap(base)
	for key, value := range extra {
		env[key] = value
	}
	return envList(env)
}

func envMap(base []string) map[string]string {
	env := map[string]string{}
	for _, entry := range base {
		parts := strings.SplitN(entry, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

func envList(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for key, value := range env {
		out = append(out, key+"="+value)
	}
	return out
}

func waitForFile(path string, timeout time.Duration) ([]byte, error) {
	deadline := time.Now().Add(timeout)
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(50 * time.Millisecond)
	}
}

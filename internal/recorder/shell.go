// Package recorder provides shell session recording functionality.
package recorder

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/chazuruo/faire/internal/recorder/hooks"
	"github.com/google/uuid"
)

// Exit codes
const (
	ExitCodeOK              = 0
	ExitCodeNothingCaptured = 13
	ExitCodeGitFailure      = 10
)

// Errors
var (
	ErrNoCommandsCaptured = fmt.Errorf("no commands captured")
	ErrUnsupportedShell   = fmt.Errorf("unsupported shell")
)

// Supported shells
const (
	ShellBash = "bash"
	ShellZsh  = "zsh"
	ShellSh   = "sh"
)

// Recorder handles shell session recording.
type Recorder struct {
	shell string
	cwd   string
}

// NewRecorder creates a new Recorder with the given shell.
func NewRecorder(shell string) (*Recorder, error) {
	if !isSupportedShell(shell) {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedShell, shell)
	}

	return &Recorder{
		shell: shell,
		cwd:   "", // Will use current working directory
	}, nil
}

// SetCWD sets the working directory for the recorded session.
func (r *Recorder) SetCWD(cwd string) {
	r.cwd = cwd
}

// StartRecordingSession starts a recording session by spawning a subshell.
// It blocks until the user exits the shell, then returns captured commands.
func (r *Recorder) StartRecordingSession(ctx context.Context) ([]CapturedCommand, error) {
	// 1. Create temp capture file
	captureID := uuid.New().String()
	captureFile := filepath.Join(os.TempDir(), fmt.Sprintf("gitsavvy-record-%s.log", captureID))

	file, err := os.Create(captureFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create capture file: %w", err)
	}
	file.Close()

	// 2. Generate hook script
	hookScript, err := r.generateHookScript(captureFile)
	if err != nil {
		os.Remove(captureFile)
		return nil, fmt.Errorf("failed to generate hook script: %w", err)
	}
	defer os.Remove(hookScript)

	// 3. Build shell command
	cmd := r.buildShellCommand(hookScript)

	// Set up environment
	cmd.Env = append(os.Environ(),
		"GITSAVVY_CAPTURE_FILE="+captureFile,
	)

	// Set prompt to show recording indicator
	if ps1 := os.Getenv("PS1"); ps1 != "" {
		cmd.Env = append(cmd.Env, "PS1=[REC] "+ps1)
	}

	// Connect terminal
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// 4. Run shell (blocks until exit)
	fmt.Printf("\n🎙️  Recording session started (shell: %s)\n", r.shell)
	fmt.Printf("📝 Capture file: %s\n", captureFile)
	fmt.Printf("🚪 Type 'exit' or Ctrl-D to stop recording\n\n")

	runErr := cmd.Run()

	// 5. Parse capture file
	commands, err := r.parseCaptureFile(captureFile)
	if err != nil {
		if err == ErrNoCommandsCaptured {
			return nil, ErrNoCommandsCaptured
		}
		return nil, fmt.Errorf("failed to parse capture file: %w", err)
	}

	// Check shell exit error (non-zero is ok, user might have exited with error)
	if runErr != nil {
		// If it's an ExitError, the shell exited with non-zero - that's fine
		if _, ok := runErr.(*exec.ExitError); !ok {
			// Some other error occurred
			return nil, fmt.Errorf("shell execution error: %w", runErr)
		}
	}

	return commands, nil
}

// generateHookScript generates the shell-specific hook script.
func (r *Recorder) generateHookScript(captureFile string) (string, error) {
	var content string

	switch r.shell {
	case ShellBash:
		content = hooks.BashHookTemplate(captureFile)
	case ShellZsh:
		content = hooks.ZshHookTemplate(captureFile)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedShell, r.shell)
	}

	// Write to temp file
	tmpFile, err := os.CreateTemp("", "gitsavvy-hook-*.sh")
	if err != nil {
		return "", fmt.Errorf("failed to create temp hook file: %w", err)
	}

	if _, err := tmpFile.WriteString(content); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write hook script: %w", err)
	}

	tmpFile.Close()
	return tmpFile.Name(), nil
}

// buildShellCommand builds the command to spawn the shell with hooks.
func (r *Recorder) buildShellCommand(hookScript string) *exec.Cmd {
	cwd := r.cwd
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	switch r.shell {
	case ShellBash:
		// bash --rcfile <(cat hook.sh) -i
		// Use process substitution to source the hook
		return exec.Command("bash", "--rcfile", hookScript, "-i")
	case ShellZsh:
		// zsh -i -c 'source hook.sh; exec zsh'
		return exec.Command("zsh", "-i", "-c", fmt.Sprintf("source %s; exec zsh", hookScript))
	default:
		// For sh, just spawn normally (no hooks available)
		return exec.Command(r.shell)
	}
}

// isSupportedShell checks if the shell is supported for recording.
func isSupportedShell(shell string) bool {
	switch shell {
	case ShellBash, ShellZsh, ShellSh:
		return true
	default:
		return false
	}
}

// DetectShell attempts to detect the current shell from environment.
func DetectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return ShellBash // Default to bash
	}

	// Extract shell name from path
	base := filepath.Base(shell)
	switch {
	case strings.Contains(base, "zsh"):
		return ShellZsh
	case strings.Contains(base, "bash"):
		return ShellBash
	default:
		return ShellBash // Default fallback
	}
}

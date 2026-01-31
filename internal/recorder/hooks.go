// Package recorder provides shell session recording functionality.
package recorder

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/google/uuid"
)

// HookGenerator generates shell hooks for capturing commands.
type HookGenerator struct {
	CaptureFile string
	SessionID   string
}

// NewHookGenerator creates a new hook generator.
func NewHookGenerator(captureFile string) *HookGenerator {
	return &HookGenerator{
		CaptureFile: captureFile,
		SessionID:   uuid.New().String(),
	}
}

// GenerateBashHook generates a bash shell hook script.
func (h *HookGenerator) GenerateBashHook() string {
	// Bash uses PROMPT_COMMAND which runs before each prompt
	return fmt.Sprintf(`# svf recording hook
GITSAVVY_CAPTURE_FILE="%s"
GITSAVVY_SESSION_ID="%s"
GITSAVVY_LAST_CMD=""

_gitsavvy_capture() {
    local cmd="$BASH_COMMAND"
    local cwd="$(pwd)"
    local ts="$(date +%%s)"

    # Skip empty commands, duplicates, built-ins
    [[ -z "$cmd" ]] && return
    [[ "$cmd" == "$_GITSAVVY_LAST_CMD" ]] && return

    # Built-ins to skip (navigation, job control, etc.)
    case "$cmd" in
        cd|pushd|popd|dirs|pwd|ls|la|ll|clear|history|exit|logout|jobs|fg|bg) return ;;
    esac

    # Unit separator (0x1F) is unlikely to appear in commands
    echo "${ts}${'\x1F'}${cwd}${'\x1F'}${cmd}" >> "$GITSAVVY_CAPTURE_FILE"
    _GITSAVVY_LAST_CMD="$cmd"
}

# Hook into prompt
PROMPT_COMMAND="_gitsavvy_capture${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
`, h.CaptureFile, h.SessionID)
}

// GenerateZshHook generates a zsh shell hook script.
func (h *HookGenerator) GenerateZshHook() string {
	// Zsh uses precmd_functions which runs before each prompt
	return fmt.Sprintf(`# svf recording hook
GITSAVVY_CAPTURE_FILE="%s"
GITSAVVY_SESSION_ID="%s"
GITSAVVY_LAST_CMD=""

_gitsavvy_precmd() {
    local cmd="$history[$((HISTCMD-1))]"
    local cwd="$(pwd)"
    local ts="$(date +%%s)"

    # Skip empty, duplicates, built-ins
    [[ -z "$cmd" ]] && return
    [[ "$cmd" == "$GITSAVVY_LAST_CMD" ]] && return

    # Built-ins to skip
    case "$cmd" in
        cd|pushd|popd|dirs|pwd|ls|la|ll|clear|history|exit|logout|jobs|fg|bg) return ;;
    esac

    # Unit separator (0x1F) is unlikely to appear in commands
    echo "${ts}${'\x1F'}${cwd}${'\x1F'}${cmd}" >> "$GITSAVVY_CAPTURE_FILE"
    _GITSAVVY_LAST_CMD="$cmd"
}

# Hook into zsh
precmd_functions+=(_gitsavvy_precmd)
`, h.CaptureFile, h.SessionID)
}

// InitScriptTemplate is the template for shell init scripts.
type InitScriptTemplate struct {
	HookContent string
	Shell       string
}

// GenerateInitScript generates an init script for the given shell.
func (h *HookGenerator) GenerateInitScript(shell string) (string, error) {
	var hookContent string
	switch shell {
	case "bash":
		hookContent = h.GenerateBashHook()
	case "zsh":
		hookContent = h.GenerateZshHook()
	default:
		return "", fmt.Errorf("unsupported shell: %s", shell)
	}

	return hookContent, nil
}

// SessionConfig represents a recording session configuration.
type SessionConfig struct {
	CaptureFile string
	SessionID   string
	Shell       string
	Prompt      string
}

// NewSession creates a new recording session.
func NewSession(shell string) (*SessionConfig, error) {
	if shell == "" {
		shell = detectShell()
	}

	// Create temp capture file
	tmpFile, err := os.CreateTemp("", "svf-record-*.log")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	_ = tmpFile.Close()

	return &SessionConfig{
		CaptureFile: tmpFile.Name(),
		SessionID:   uuid.New().String(),
		Shell:      shell,
		Prompt:     "[REC] ",
	}, nil
}

// Cleanup removes the capture file.
func (s *SessionConfig) Cleanup() error {
	if s.CaptureFile != "" {
		return os.Remove(s.CaptureFile)
	}
	return nil
}

// GetShellArgs returns the shell arguments to start a recorded session.
func (s *SessionConfig) GetShellArgs() ([]string, error) {
	gen := NewHookGenerator(s.CaptureFile)

	// Generate hook content
	hookContent, err := gen.GenerateInitScript(s.Shell)
	if err != nil {
		return nil, err
	}

	// Create a temp file for the init script
	initScript, err := os.CreateTemp("", "svf-init-*.sh")
	if err != nil {
		return nil, fmt.Errorf("failed to create init script: %w", err)
	}

	// Write hook to init script
	if _, err := initScript.WriteString(hookContent); err != nil {
		_ = initScript.Close()
		return nil, fmt.Errorf("failed to write init script: %w", err)
	}
	_ = initScript.Close()

	// Build command based on shell
	switch s.Shell {
	case "bash":
		// Use --rcfile to inject our hooks
		return []string{"--rcfile", initScript.Name(), "-i"}, nil

	case "zsh":
		// Zsh doesn't have --rcfile, use -c to source then exec
		// We need to be careful here - we want to source the init script
		// and then exec zsh to replace the current process
		return []string{"-i", "-c", fmt.Sprintf("source %s; exec zsh", initScript.Name())}, nil

	default:
		return nil, fmt.Errorf("unsupported shell: %s", s.Shell)
	}
}

// GetEnv returns environment variables for the recorded session.
func (s *SessionConfig) GetEnv() []string {
	return []string{
		fmt.Sprintf("GITSAVVY_CAPTURE_FILE=%s", s.CaptureFile),
		fmt.Sprintf("GITSAVVY_SESSION_ID=%s", s.SessionID),
		fmt.Sprintf("PS1=%s%s", s.Prompt, os.Getenv("PS1")),
	}
}

// detectShell detects the user's default shell.
func detectShell() string {
	// Check SHELL env var
	if shell := os.Getenv("SHELL"); shell != "" {
		return filepath.Base(shell)
	}

	// Fallback to platform defaults
	switch runtime.GOOS {
	case "windows":
		return "bash" // Assuming Git Bash or WSL
	default:
		return "bash"
	}
}

// ExpandAliases expands shell aliases for a command.
// This runs the shell's alias command and parses the output.
func ExpandAliases(shell string, command string) (string, error) {
	var aliasCmd string
	switch shell {
	case "bash":
		aliasCmd = "alias"
	case "zsh":
		aliasCmd = "alias -L"
	default:
		return command, nil // Return as-is for unknown shells
	}

	// Run alias command
	cmd := exec.Command(shell, "-c", aliasCmd)
	output, err := cmd.Output()
	if err != nil {
		return command, err // Return original if expansion fails
	}

	// Parse alias definitions
	aliases := parseAliasOutput(string(output))

	// Simple prefix expansion (doesn't handle all edge cases)
	for name, expansion := range aliases {
		if strings.HasPrefix(command, name+" ") || command == name {
			return strings.Replace(command, name, expansion, 1), nil
		}
	}

	return command, nil
}

// parseAliasOutput parses the output of the alias command.
func parseAliasOutput(output string) map[string]string {
	aliases := make(map[string]string)

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse alias format: alias name='value' or alias name=value
		// Bash: alias ll='ls -la'
		// Zsh: alias -L ll='ls -la'
		line = strings.TrimPrefix(line, "alias ")
		line = strings.TrimPrefix(line, "-L ")

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			name := parts[0]
			value := strings.Trim(parts[1], "'\"")
			aliases[name] = value
		}
	}

	return aliases
}

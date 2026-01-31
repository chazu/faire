// Package hooks provides shell hook templates for command capture.
package hooks

// ZshHookTemplate returns the zsh hook script for capturing commands.
func ZshHookTemplate(captureFile string) string {
	return `
export GITSAVVY_CAPTURE_FILE="` + captureFile + `"
export GITSAVVY_LAST_LINE=""

_gitsavvy_precmd() {
    local cmd="$history[$((HISTCMD-1))]"
    local cwd="$(pwd)"
    local ts="$(date +%s)"

    [[ -z "$cmd" ]] && return
    [[ "$cmd" == "$GITSAVVY_LAST_LINE" ]] && return

    # Skip built-ins and common non-work commands
    case "$cmd" in
        cd|pushd|popd|dirs|pwd|ls|la|ll|clear|history|exit|logout|jobs|fg|bg) return ;;
    esac

    echo "${ts}|${cwd}|${cmd}" >> "$GITSAVVY_CAPTURE_FILE"
    GITSAVVY_LAST_LINE="$cmd"
}

# Hook into precmd_functions
precmd_functions+=(_gitsavvy_precmd)

# Set recording indicator in prompt
export PROMPT="[REC] ${PROMPT}"
`
}

// Package hooks provides shell hook templates for command capture.
package hooks

// BashHookTemplate returns the bash hook script for capturing commands.
func BashHookTemplate(captureFile string) string {
	return `
export GITSAVVY_CAPTURE_FILE="` + captureFile + `"
export GITSAVVY_LAST_LINE=""

_gitsavvy_prompt_command() {
    local cmd=""
    local cwd="${PWD}"
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

# Hook into PROMPT_COMMAND
if [[ -n "$PROMPT_COMMAND" ]]; then
    PROMPT_COMMAND="_gitsavvy_prompt_command;$PROMPT_COMMAND"
else
    PROMPT_COMMAND="_gitsavvy_prompt_command"
fi

# Set recording indicator in prompt
export PS1="[REC] $PS1"
`
}

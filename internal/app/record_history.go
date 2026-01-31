package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/chazuruo/faire/internal/history"
	"github.com/chazuruo/faire/internal/workflows"
)

// RecordHistoryOptions contains options for recording history to a workflow
type RecordHistoryOptions struct {
	Shell      string        // Shell type override (bash, zsh, pwsh)
	Limit      int           // Max commands to load (default 500)
	Since      time.Duration // Time filter (e.g., "1h", "1d")
	Title      string        // Workflow title
	Desc       string        // Workflow description
	Tags       []string      // Workflow tags
	Identity   string        // Override identity path
	Draft      bool          // Save to drafts/ instead of workflows/
	NoCommit   bool          // Skip git commit
	StepPrefix string        // Prefix for step names (default auto-generated)
}

// CommandsToSteps converts shell commands to workflow steps
func CommandsToSteps(cmds []string) []workflows.Step {
	steps := make([]workflows.Step, len(cmds))
	for i, cmd := range cmds {
		steps[i] = workflows.Step{
			Name:    GenerateStepName(cmd),
			Command: cmd,
			// inherit defaults from workflow
		}
	}
	return steps
}

// GenerateStepName creates a readable step name from a command
func GenerateStepName(cmd string) string {
	// Extract first word (the command)
	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return "Unnamed command"
	}

	command := fields[0]

	// Common command patterns with descriptive names
	patterns := map[string]string{
		"kubectl":  "Kubernetes",
		"k":        "Kubernetes",
		"docker":   "Docker",
		"git":      "Git",
		"gh":       "GitHub CLI",
		"npm":      "NPM",
		"yarn":     "Yarn",
		"make":     "Make",
		"cargo":    "Cargo",
		"go":       "Go",
		"python":   "Python",
		"python3":  "Python",
		"pip":      "Pip",
		"apt":      "APT",
		"brew":     "Homebrew",
		"terraform": "Terraform",
		"tf":       "Terraform",
		"ansible":  "Ansible",
		"vagrant":  "Vagrant",
		"aws":      "AWS",
		"az":       "Azure",
		"gcloud":   "GCloud",
		"curl":     "HTTP request",
		"wget":     "Download",
		"ssh":      "SSH",
		"scp":      "SCP",
		"rsync":    "Rsync",
	}

	// Check for known patterns
	if base, ok := patterns[command]; ok {
		// For simple tools like terraform, aws, etc., just return the base name
		if base == "Terraform" || base == "AWS" || base == "Azure" || base == "GCloud" || base == "Ansible" || base == "Vagrant" {
			return base
		}
		// Add action from second word if available
		if len(fields) > 1 {
			action := fields[1]
			return fmt.Sprintf("%s %s", base, action)
		}
		return base
	}

	// Git subcommands
	if command == "git" && len(fields) > 1 {
		subcommand := fields[1]
		// Handle flags before subcommand (git -C path status)
		if strings.HasPrefix(subcommand, "-") {
			if len(fields) > 2 {
				subcommand = fields[2]
			}
		}
		return fmt.Sprintf("Git %s", subcommand)
	}

	//kubectl subcommands
	if command == "kubectl" && len(fields) > 1 {
		subcommand := fields[1]
		return fmt.Sprintf("Kubernetes %s", subcommand)
	}

	// Docker commands
	if command == "docker" && len(fields) > 1 {
		subcommand := fields[1]
		return fmt.Sprintf("Docker %s", subcommand)
	}

	// Default: capitalize first word
	return strings.ToUpper(command[:1]) + command[1:]
}

// DetectTagsFromCommands analyzes commands to suggest workflow tags
func DetectTagsFromCommands(cmds []string) []string {
	tagSet := make(map[string]bool)

	for _, cmd := range cmds {
		cmd = strings.ToLower(cmd)

		// Detect cloud/platform
		if strings.Contains(cmd, "kubectl") || strings.Contains(cmd, "k8s") {
			tagSet["kubernetes"] = true
		}
		if strings.Contains(cmd, "docker") {
			tagSet["docker"] = true
		}
		if strings.Contains(cmd, "terraform") || strings.Contains(cmd, "tf ") {
			tagSet["terraform"] = true
		}
		if strings.Contains(cmd, "aws") {
			tagSet["aws"] = true
		}
		if strings.Contains(cmd, "gcloud") {
			tagSet["gcp"] = true
		}

		// Detect actions
		if strings.Contains(cmd, "deploy") {
			tagSet["deployment"] = true
		}
		if strings.Contains(cmd, "build") {
			tagSet["build"] = true
		}
		if strings.Contains(cmd, "test") {
			tagSet["testing"] = true
		}
		if strings.Contains(cmd, "release") {
			tagSet["release"] = true
		}
	}

	// Convert map to slice
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	return tags
}

// LoadHistoryForRecording loads shell history with filtering
func LoadHistoryForRecording(opts RecordHistoryOptions) ([]history.HistoryLine, error) {
	// Detect shell type
	var shellType history.ShellType
	if opts.Shell != "" {
		shellType = history.ShellType(opts.Shell)
	} else {
		shellType = history.DetectShell()
		if shellType == history.ShellUnknown {
			return nil, fmt.Errorf("could not detect shell type, please specify --shell")
		}
	}

	// Create parser and detect path
	parser, err := history.NewParser(shellType)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser: %w", err)
	}

	path, err := parser.DetectPath()
	if err != nil {
		return nil, fmt.Errorf("failed to detect history file: %w", err)
	}

	// Parse history
	lines, err := parser.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse history: %w", err)
	}

	// Apply filters
	filterOpts := history.FilterOptions{
		RemoveDup: true,
	}

	if opts.Limit > 0 {
		filterOpts.MaxLines = opts.Limit
	}

	if opts.Since > 0 {
		filterOpts.Since = time.Now().Add(-opts.Since)
	}

	filtered := history.FilterLines(lines, filterOpts)

	return filtered, nil
}

// ParseDuration parses a duration string like "1h", "1d", "1w"
func ParseDuration(s string) (time.Duration, error) {
	// First try standard time.ParseDuration
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}

	// Handle custom units
	s = strings.ToLower(s)
	re := regexp.MustCompile(`^(\d+)([mhdw])$`)
	matches := re.FindStringSubmatch(s)
	if matches == nil {
		return 0, fmt.Errorf("invalid duration format: %s (use: 1h, 1d, 1w)", s)
	}

	value, _ := strconv.Atoi(matches[1])
	unit := matches[2]

	switch unit {
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown duration unit: %s", unit)
	}
}

// GenerateWorkflowFromCommands creates a workflow from selected commands
func GenerateWorkflowFromCommands(cmds []string, opts RecordHistoryOptions) *workflows.Workflow {
	// Generate title if not provided
	title := opts.Title
	if title == "" && len(cmds) > 0 {
		title = GenerateStepName(cmds[0])
		if len(cmds) > 1 {
			title += " and more"
		}
	}

	// Detect tags if not provided
	tags := opts.Tags
	if len(tags) == 0 {
		tags = DetectTagsFromCommands(cmds)
	}

	// Convert commands to steps
	steps := CommandsToSteps(cmds)

	return &workflows.Workflow{
		SchemaVersion: workflows.SchemaVersion,
		Title:         title,
		Description:   opts.Desc,
		Tags:          tags,
		Steps:         steps,
	}
}

// Package explain provides rule-based command explanation.
package explain

import (
	"fmt"
	"regexp"
	"strings"
)

// Explainer provides rule-based command explanations.
type Explainer struct {
	rules []Rule
}

// Rule defines an explanation rule for command patterns.
type Rule struct {
	Pattern   string         // Regex pattern to match
	Explanation string       // Explanation template
	Risk      string         // Risk level (safe, low, medium, high)
	Category  string         // Command category (git, docker, kubectl, etc.)
}

// NewExplainer creates a new explainer with built-in rules.
func NewExplainer() *Explainer {
	return &Explainer{
		rules: builtinRules(),
	}
}

// ExplainCommand explains a single command.
func (e *Explainer) ExplainCommand(cmd string) Explanation {
	return e.explain(cmd, CommandExplanation)
}

// ExplainWorkflowStep explains a workflow step in context.
func (e *Explainer) ExplainWorkflowStep(stepName, command string, stepIndex int) Explanation {
	explanation := e.explain(command, CommandExplanation)

	// Add context
	explanation.Context = fmt.Sprintf("Step %d: %s", stepIndex+1, stepName)
	explanation.StepName = stepName
	explanation.StepIndex = stepIndex

	return explanation
}

// ExplainType is what to explain.
type ExplainType int

const (
	// CommandExplanation explains a single command.
	CommandExplanation ExplainType = iota
	// StepExplanation explains a workflow step.
	StepExplanation
)

// Explanation represents a command explanation.
type Explanation struct {
	Command      string
	Explanation  string
	Risk         string
	Category     string
	Context      string // Additional context (for workflow steps)
	StepName     string
	StepIndex    int
	Alternatives []string
	SeeAlso      []string
}

// explain performs the explanation.
func (e *Explainer) explain(cmd string, explainType ExplainType) Explanation {
	cmd = strings.TrimSpace(cmd)

	// Find matching rule
	for _, rule := range e.rules {
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		if re.MatchString(cmd) {
			return Explanation{
				Command:     cmd,
				Explanation: rule.Explanation,
				Risk:        rule.Risk,
				Category:    rule.Category,
			}
		}
	}

	// No match found, return generic explanation
	return Explanation{
		Command:     cmd,
		Explanation: "Executes the specified command. No specific explanation available.",
		Risk:        "unknown",
		Category:    "other",
	}
}

// builtinRules returns built-in explanation rules.
func builtinRules() []Rule {
	return []Rule{
		// Git commands
		{
			Pattern:     `(?i)\bgit\s+(status|st)`,
			Explanation: "Shows the working tree status. Displays changed files, staged changes, and untracked files.",
			Risk:       "safe",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+(add|update-index)`,
			Explanation: "Adds file contents to the staging area (index). Prepares changes for the next commit.",
			Risk:       "safe",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+commit`,
			Explanation: "Creates a new commit with staged changes. Records a snapshot of the project's history.",
			Risk:       "safe",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+(push|publish)`,
			Explanation: "Uploads local commits to a remote repository. Shares changes with others.",
			Risk:       "low",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+(pull|fetch)`,
			Explanation: "Fetches changes from a remote repository and integrates them into the current branch.",
			Risk:       "low",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+branch\s+-D`,
			Explanation: "Deletes a git branch, even if it hasn't been merged. This cannot be easily undone.",
			Risk:       "medium",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+push.*--force`,
			Explanation: "Force pushes to a remote repository, potentially overwriting remote history. Can cause data loss for others.",
			Risk:       "high",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+reset.*--hard`,
			Explanation: "Resets the current branch to a specific state, discarding all uncommitted changes. Cannot be undone.",
			Risk:       "high",
			Category:   "git",
		},
		{
			Pattern:     `(?i)\bgit\s+rebase`,
			Explanation: "Reapplies commits on top of another base branch. Rewrites history and can cause conflicts.",
			Risk:       "medium",
			Category:   "git",
		},

		// Docker commands
		{
			Pattern:     `(?i)\bdocker\s+ps`,
			Explanation: "Lists running Docker containers.",
			Risk:       "safe",
			Category:   "docker",
		},
		{
			Pattern:     `(?i)\bdocker\s+run`,
			Explanation: "Runs a command in a new Docker container. Creates and starts a container from an image.",
			Risk:       "low",
			Category:   "docker",
		},
		{
			Pattern:     `(?i)\bdocker\s+(rm|remove)\s+.*`,
			Explanation: "Removes one or more Docker containers. Deletes the container but not the image.",
			Risk:       "low",
			Category:   "docker",
		},
		{
			Pattern:     `(?i)\bdocker\s+rmi\s+.*`,
			Explanation: "Removes one or more Docker images. Deletes the image from local storage.",
			Risk:       "low",
			Category:   "docker",
		},
		{
			Pattern:     `(?i)\bdocker\s+build`,
			Explanation: "Builds a Docker image from a Dockerfile. Packages an application and its dependencies.",
			Risk:       "safe",
			Category:   "docker",
		},

		// Kubernetes (kubectl) commands
		{
			Pattern:     `(?i)\bkubectl\s+get\s+(pods|po|services|svc|deployments|deploy)`,
			Explanation: "Lists Kubernetes resources. Shows the status of pods, services, or deployments.",
			Risk:       "safe",
			Category:   "kubectl",
		},
		{
			Pattern:     `(?i)\bkubectl\s+apply\s+-f`,
			Explanation: "Applies a configuration to a Kubernetes cluster. Creates or updates resources from a YAML or JSON file.",
			Risk:       "low",
			Category:   "kubectl",
		},
		{
			Pattern:     `(?i)\bkubectl\s+delete\s+.*`,
			Explanation: "Deletes Kubernetes resources. Pods, services, or other resources are removed from the cluster.",
			Risk:       "medium",
			Category:   "kubectl",
		},
		{
			Pattern:     `(?i)\bkubectl\s+(logs|log)`,
			Explanation: "Prints logs from a container in a pod. Useful for debugging and monitoring applications.",
			Risk:       "safe",
			Category:   "kubectl",
		},
		{
			Pattern:     `(?i)\bkubectl\s+exec`,
			Explanation: "Executes a command in a container. Opens an interactive shell or runs a single command.",
			Risk:       "low",
			Category:   "kubectl",
		},

		// File system commands
		{
			Pattern:     `(?i)\bls\s+.*`,
			Explanation: "Lists directory contents. Shows files and subdirectories.",
			Risk:       "safe",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\bcd\s+.*`,
			Explanation: "Changes the current working directory. navigates to a different directory in the filesystem.",
			Risk:       "safe",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\bcp\s+.*`,
			Explanation: "Copies files or directories. Creates a duplicate of the source at the destination.",
			Risk:       "low",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\bmv\s+.*`,
			Explanation: "Moves or renames files and directories. Changes the location or name of the source.",
			Risk:       "low",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\brm\s+.*`,
			Explanation: "Removes (deletes) files or directories. This operation is generally irreversible.",
			Risk:       "medium",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\brm\s+(-rf|-r).*\/`,
			Explanation: "Recursively deletes files and directories. Extremely dangerous - destroys all data in the target path.",
			Risk:       "high",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\bchmod\s+.*`,
			Explanation: "Changes file permissions. Modifies who can read, write, or execute a file.",
			Risk:       "low",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\bchown\s+.*`,
			Explanation: "Changes file owner and group. Modifies user and group ownership of files.",
			Risk:       "low",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\bfind\s+.*`,
			Explanation: "Searches for files in a directory hierarchy. Finds files matching specific criteria.",
			Risk:       "safe",
			Category:   "filesystem",
		},
		{
			Pattern:     `(?i)\bchmod\s+.*\s+777`,
			Explanation: "Sets files to world-writable (anyone can read, write, execute). Security risk as it allows unrestricted access.",
			Risk:       "high",
			Category:   "filesystem",
		},

		// Package managers
		{
			Pattern:     `(?i)\b(npm|pnpm|yarn)\s+(install|i)`,
			Explanation: "Installs Node.js dependencies from package.json. Downloads and sets up required packages.",
			Risk:       "safe",
			Category:   "package-manager",
		},
		{
			Pattern:     `(?i)\bbrew\s+(install|inst)`,
			Explanation: "Installs a software package using Homebrew (macOS/Linux). Downloads and sets up the application.",
			Risk:       "low",
			Category:   "package-manager",
		},
		{
			Pattern:     `(?i)\bapt-get\s+install`,
			Explanation: "Installs packages on Debian/Ubuntu Linux. Downloads and sets up system software.",
			Risk:       "low",
			Category:   "package-manager",
		},
		{
			Pattern:     `(?i)\bpip\s+install`,
			Explanation: "Installs Python packages from PyPI. Downloads and sets up required libraries.",
			Risk:       "low",
			Category:   "package-manager",
		},
		{
			Pattern:     `(?i)\bgo\s+get`,
			Explanation: "Downloads and installs Go packages. Adds dependencies to the Go module.",
			Risk:       "safe",
			Category:   "package-manager",
		},

		// Shell builtins
		{
			Pattern:     `(?i)^\s*echo\s+`,
			Explanation: "Prints text to standard output. Displays messages or variable values.",
			Risk:       "safe",
			Category:   "shell",
		},
		{
			Pattern:     `(?i)^\s*cat\s+`,
			Explanation: "Reads and displays file contents. Outputs the entire file to the terminal.",
			Risk:       "safe",
			Category:   "shell",
		},
		{
			Pattern:     `(?i)^\s*(grep|egrep|rg)\s+`,
			Explanation: "Searches for patterns in text. Finds lines matching a regular expression.",
			Risk:       "safe",
			Category:   "shell",
		},
		{
			Pattern:     `(?i)^\s*(less|more)\s+`,
			Explanation: "Views file contents one page at a time. Allows scrolling through long files.",
			Risk:       "safe",
			Category:   "shell",
		},
		{
			Pattern:     `(?i)^\s*tail\s+`,
			Explanation: "Displays the end of a file. Shows the last few lines, useful for monitoring logs.",
			Risk:       "safe",
			Category:   "shell",
		},
		{
			Pattern:     `(?i)^\s*head\s+`,
			Explanation: "Displays the beginning of a file. Shows the first few lines.",
			Risk:       "safe",
			Category:   "shell",
		},

		// System commands
		{
			Pattern:     `(?i)\bshutdown\b`,
			Explanation: "Shuts down the system. Powers off the computer.",
			Risk:       "high",
			Category:   "system",
		},
		{
			Pattern:     `(?i)\breboot\b`,
			Explanation: "Restarts the system. Reboots the computer.",
			Risk:       "high",
			Category:   "system",
		},
		{
			Pattern:     `(?i)\b systemctl\s+(start|stop|restart)`,
			Explanation: "Controls systemd services. Starts, stops, or restarts system services.",
			Risk:       "low",
			Category:   "system",
		},
	}
}

// AddRule adds a custom explanation rule.
func (e *Explainer) AddRule(rule Rule) error {
	// Validate pattern
	_, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}

	e.rules = append(e.rules, rule)
	return nil
}

// RiskLevel represents a risk level.
type RiskLevel int

const (
	RiskUnknown RiskLevel = iota
	RiskSafe
	RiskLow
	RiskMedium
	RiskHigh
)

// ParseRiskLevel parses a risk level string.
func ParseRiskLevel(s string) RiskLevel {
	switch strings.ToLower(s) {
	case "safe":
		return RiskSafe
	case "low":
		return RiskLow
	case "medium":
		return RiskMedium
	case "high":
		return RiskHigh
	default:
		return RiskUnknown
	}
}

// String returns the string representation of a risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskSafe:
		return "safe"
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	default:
		return "unknown"
	}
}

// Color returns a terminal color code for the risk level.
func (r RiskLevel) Color() string {
	switch r {
	case RiskSafe:
		return "green"
	case RiskLow:
		return "blue"
	case RiskMedium:
		return "yellow"
	case RiskHigh:
		return "red"
	default:
		return "gray"
	}
}

// Icon returns an emoji icon for the risk level.
func (r RiskLevel) Icon() string {
	switch r {
	case RiskSafe:
		return "✓"
	case RiskLow:
		return "ⓘ"
	case RiskMedium:
		return "⚠"
	case RiskHigh:
		return "☠"
	default:
		return "?"
	}
}

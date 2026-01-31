// Package app provides high-level application logic for git-savvy commands.
package app

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/chazuruo/faire/internal/config"
)

// WhoamiOutput contains the information displayed by the whoami command.
type WhoamiOutput struct {
	ConfigPath  string         `json:"config_path"`
	RepoPath    string         `json:"repo_path"`
	IdentityPath string        `json:"identity_path"`
	Mode        string         `json:"mode"`
	Author      AuthorInfo     `json:"author"`
}

// AuthorInfo contains author name and email.
type AuthorInfo struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Whoami loads the config and returns identity information.
// If configPath is empty, it uses the default XDG config path.
// Returns an error if the config file cannot be found or loaded.
func Whoami(configPath string) (*WhoamiOutput, error) {
	var cfg *config.Config
	var err error
	var actualConfigPath string

	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
		actualConfigPath = configPath
	} else {
		// Detect the config file path
		actualConfigPath = config.DetectConfigPath()
		if actualConfigPath == "" {
			return nil, fmt.Errorf("config file not found (expected ~/.config/gitsavvy/config.toml)")
		}
		cfg, err = config.Load(actualConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	return &WhoamiOutput{
		ConfigPath:   actualConfigPath,
		RepoPath:     cfg.Repo.Path,
		IdentityPath: cfg.Identity.Path,
		Mode:         cfg.Identity.Mode,
		Author: AuthorInfo{
			Name:  cfg.Git.AuthorName,
			Email: cfg.Git.AuthorEmail,
		},
	}, nil
}

// PrintWhoami prints whoami information in plain text format.
func PrintWhoami(output *WhoamiOutput) {
	fmt.Printf("Config: %s\n", output.ConfigPath)
	fmt.Printf("Repo: %s\n", output.RepoPath)
	fmt.Printf("Identity Path: %s\n", output.IdentityPath)
	fmt.Printf("Mode: %s\n", output.Mode)
	fmt.Printf("Author: %s <%s>\n", output.Author.Name, output.Author.Email)
}

// PrintWhoamiJSON prints whoami information in JSON format.
func PrintWhoamiJSON(output *WhoamiOutput) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

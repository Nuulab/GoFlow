// Package tools provides shell/command execution tools for agents.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ShellConfig configures shell tool behavior.
type ShellConfig struct {
	// AllowedCommands restricts which commands can be run (empty = all allowed).
	AllowedCommands []string
	// BlockedCommands prevents specific commands from running.
	BlockedCommands []string
	// WorkingDir sets the working directory for commands.
	WorkingDir string
	// Timeout limits command execution time.
	Timeout time.Duration
	// MaxOutputSize limits output size in bytes.
	MaxOutputSize int
}

// DefaultShellConfig returns safe defaults.
func DefaultShellConfig() ShellConfig {
	return ShellConfig{
		BlockedCommands: []string{"rm", "sudo", "chmod", "chown", "mkfs", "dd", "shutdown", "reboot"},
		Timeout:         30 * time.Second,
		MaxOutputSize:   1024 * 1024, // 1MB
	}
}

// ShellToolkit returns tools for command execution.
func ShellToolkit(config ShellConfig) *Toolkit {
	return &Toolkit{
		Name:        "shell",
		Description: "Tools for executing shell commands",
		Tools: []*Tool{
			runCommandTool(config),
			whichTool(),
			envTool(),
		},
	}
}

func runCommandTool(config ShellConfig) *Tool {
	return Build("run_command").
		Description("Execute a shell command and return its output").
		Param("command", "string", "The command to execute").
		OptionalParam("args", "array", "Command arguments").
		OptionalParam("working_dir", "string", "Working directory").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Command    string   `json:"command"`
				Args       []string `json:"args"`
				WorkingDir string   `json:"working_dir"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				// Simple command string
				params.Command = input
			}

			// Security: Check blocked commands
			cmdName := strings.Split(params.Command, " ")[0]
			for _, blocked := range config.BlockedCommands {
				if cmdName == blocked {
					return "", fmt.Errorf("command '%s' is blocked for security reasons", cmdName)
				}
			}

			// Security: Check allowed commands
			if len(config.AllowedCommands) > 0 {
				allowed := false
				for _, cmd := range config.AllowedCommands {
					if cmdName == cmd {
						allowed = true
						break
					}
				}
				if !allowed {
					return "", fmt.Errorf("command '%s' is not in the allowed list", cmdName)
				}
			}

			// Set timeout
			timeout := config.Timeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}
			ctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()

			// Build command
			var cmd *exec.Cmd
			if len(params.Args) > 0 {
				cmd = exec.CommandContext(ctx, params.Command, params.Args...)
			} else {
				// Parse command string
				parts := strings.Fields(params.Command)
				if len(parts) == 0 {
					return "", fmt.Errorf("empty command")
				}
				cmd = exec.CommandContext(ctx, parts[0], parts[1:]...)
			}

			// Set working directory
			if params.WorkingDir != "" {
				cmd.Dir = params.WorkingDir
			} else if config.WorkingDir != "" {
				cmd.Dir = config.WorkingDir
			}

			// Capture output
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()

			// Build result
			result := map[string]any{
				"stdout":    limitString(stdout.String(), config.MaxOutputSize),
				"stderr":    limitString(stderr.String(), config.MaxOutputSize),
				"exit_code": 0,
			}

			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					result["exit_code"] = exitErr.ExitCode()
				} else {
					return "", fmt.Errorf("command execution failed: %w", err)
				}
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			return string(out), nil
		}).
		Create()
}

func limitString(s string, maxLen int) string {
	if maxLen <= 0 || len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (output truncated)"
}

func whichTool() *Tool {
	return Build("which").
		Description("Find the path of an executable").
		Param("command", "string", "Command name to locate").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.Command = input
			}

			path, err := exec.LookPath(params.Command)
			if err != nil {
				return "", fmt.Errorf("command not found: %s", params.Command)
			}

			return path, nil
		}).
		Create()
}

func envTool() *Tool {
	return Build("get_env").
		Description("Get environment variable value").
		Param("name", "string", "Environment variable name").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.Name = input
			}

			// Security: Block sensitive env vars
			blocked := []string{"PASSWORD", "SECRET", "TOKEN", "KEY", "CREDENTIAL"}
			upper := strings.ToUpper(params.Name)
			for _, b := range blocked {
				if strings.Contains(upper, b) {
					return "", fmt.Errorf("access to sensitive environment variable '%s' is blocked", params.Name)
				}
			}

			cmd := exec.Command("printenv", params.Name)
			out, err := cmd.Output()
			if err != nil {
				return "", fmt.Errorf("environment variable not set: %s", params.Name)
			}

			return strings.TrimSpace(string(out)), nil
		}).
		Create()
}

// GitToolkit returns tools for Git operations.
func GitToolkit(repoPath string) *Toolkit {
	return &Toolkit{
		Name:        "git",
		Description: "Tools for Git version control operations",
		Tools: []*Tool{
			gitStatusTool(repoPath),
			gitLogTool(repoPath),
			gitDiffTool(repoPath),
			gitBranchTool(repoPath),
		},
	}
}

func gitStatusTool(repoPath string) *Tool {
	return Build("git_status").
		Description("Get the current Git status").
		Handler(func(ctx context.Context, input string) (string, error) {
			cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
			cmd.Dir = repoPath
			out, err := cmd.Output()
			if err != nil {
				return "", fmt.Errorf("git status failed: %w", err)
			}
			if len(out) == 0 {
				return "Working directory clean", nil
			}
			return string(out), nil
		}).
		Create()
}

func gitLogTool(repoPath string) *Tool {
	return Build("git_log").
		Description("Get recent Git commits").
		OptionalParam("count", "integer", "Number of commits to show (default: 10)").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Count int `json:"count"`
			}
			json.Unmarshal([]byte(input), &params)
			if params.Count == 0 {
				params.Count = 10
			}

			cmd := exec.CommandContext(ctx, "git", "log",
				fmt.Sprintf("-n%d", params.Count),
				"--oneline",
				"--decorate")
			cmd.Dir = repoPath
			out, err := cmd.Output()
			if err != nil {
				return "", fmt.Errorf("git log failed: %w", err)
			}
			return string(out), nil
		}).
		Create()
}

func gitDiffTool(repoPath string) *Tool {
	return Build("git_diff").
		Description("Show Git diff").
		OptionalParam("file", "string", "Specific file to diff").
		OptionalParam("staged", "boolean", "Show staged changes").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				File   string `json:"file"`
				Staged bool   `json:"staged"`
			}
			json.Unmarshal([]byte(input), &params)

			args := []string{"diff"}
			if params.Staged {
				args = append(args, "--staged")
			}
			if params.File != "" {
				args = append(args, params.File)
			}

			cmd := exec.CommandContext(ctx, "git", args...)
			cmd.Dir = repoPath
			out, err := cmd.Output()
			if err != nil {
				return "", fmt.Errorf("git diff failed: %w", err)
			}
			if len(out) == 0 {
				return "No changes", nil
			}
			return string(out), nil
		}).
		Create()
}

func gitBranchTool(repoPath string) *Tool {
	return Build("git_branch").
		Description("List Git branches").
		Handler(func(ctx context.Context, input string) (string, error) {
			cmd := exec.CommandContext(ctx, "git", "branch", "-a")
			cmd.Dir = repoPath
			out, err := cmd.Output()
			if err != nil {
				return "", fmt.Errorf("git branch failed: %w", err)
			}
			return string(out), nil
		}).
		Create()
}

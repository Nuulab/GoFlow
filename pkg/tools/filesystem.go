// Package tools provides filesystem tools for agents.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileToolkit returns tools for filesystem operations.
// Note: These tools should be used carefully with proper sandboxing.
func FileToolkit(allowedPaths ...string) *Toolkit {
	validator := newPathValidator(allowedPaths)

	return &Toolkit{
		Name:        "filesystem",
		Description: "Tools for file system operations",
		Tools: []*Tool{
			readFileTool(validator),
			writeFileTool(validator),
			listDirTool(validator),
			fileInfoTool(validator),
			searchFilesTool(validator),
		},
	}
}

// pathValidator ensures operations stay within allowed paths.
type pathValidator struct {
	allowedPaths []string
}

func newPathValidator(paths []string) *pathValidator {
	return &pathValidator{allowedPaths: paths}
}

func (v *pathValidator) validate(path string) error {
	if len(v.allowedPaths) == 0 {
		return nil // No restrictions
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	for _, allowed := range v.allowedPaths {
		allowedAbs, _ := filepath.Abs(allowed)
		if strings.HasPrefix(absPath, allowedAbs) {
			return nil
		}
	}

	return fmt.Errorf("path not in allowed directories: %s", path)
}

func readFileTool(validator *pathValidator) *Tool {
	return Build("read_file").
		Description("Read the contents of a file").
		Param("path", "string", "Path to the file to read").
		OptionalParam("encoding", "string", "Encoding (default: utf-8)").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.Path = input
			}

			if err := validator.validate(params.Path); err != nil {
				return "", err
			}

			data, err := os.ReadFile(params.Path)
			if err != nil {
				return "", fmt.Errorf("failed to read file: %w", err)
			}

			// Limit output size
			if len(data) > 100*1024 {
				return string(data[:100*1024]) + "\n... (truncated)", nil
			}

			return string(data), nil
		}).
		Create()
}

func writeFileTool(validator *pathValidator) *Tool {
	return Build("write_file").
		Description("Write content to a file").
		Param("path", "string", "Path to the file to write").
		Param("content", "string", "Content to write").
		OptionalParam("append", "boolean", "Append instead of overwrite").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Path    string `json:"path"`
				Content string `json:"content"`
				Append  bool   `json:"append"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}

			if err := validator.validate(params.Path); err != nil {
				return "", err
			}

			// Ensure directory exists
			dir := filepath.Dir(params.Path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return "", fmt.Errorf("failed to create directory: %w", err)
			}

			flags := os.O_WRONLY | os.O_CREATE
			if params.Append {
				flags |= os.O_APPEND
			} else {
				flags |= os.O_TRUNC
			}

			f, err := os.OpenFile(params.Path, flags, 0644)
			if err != nil {
				return "", fmt.Errorf("failed to open file: %w", err)
			}
			defer f.Close()

			n, err := f.WriteString(params.Content)
			if err != nil {
				return "", fmt.Errorf("failed to write: %w", err)
			}

			return fmt.Sprintf("Wrote %d bytes to %s", n, params.Path), nil
		}).
		Create()
}

func listDirTool(validator *pathValidator) *Tool {
	return Build("list_directory").
		Description("List files and directories in a path").
		Param("path", "string", "Directory path to list").
		OptionalParam("recursive", "boolean", "List recursively").
		OptionalParam("pattern", "string", "Glob pattern to filter (e.g., '*.go')").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Path      string `json:"path"`
				Recursive bool   `json:"recursive"`
				Pattern   string `json:"pattern"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.Path = input
			}

			if err := validator.validate(params.Path); err != nil {
				return "", err
			}

			var entries []string

			if params.Recursive {
				err := filepath.Walk(params.Path, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						return nil // Skip errors
					}
					if params.Pattern != "" {
						matched, _ := filepath.Match(params.Pattern, filepath.Base(path))
						if !matched {
							return nil
						}
					}
					relPath, _ := filepath.Rel(params.Path, path)
					if info.IsDir() {
						entries = append(entries, relPath+"/")
					} else {
						entries = append(entries, relPath)
					}
					return nil
				})
				if err != nil {
					return "", err
				}
			} else {
				items, err := os.ReadDir(params.Path)
				if err != nil {
					return "", fmt.Errorf("failed to read directory: %w", err)
				}

				for _, item := range items {
					name := item.Name()
					if params.Pattern != "" {
						matched, _ := filepath.Match(params.Pattern, name)
						if !matched {
							continue
						}
					}
					if item.IsDir() {
						entries = append(entries, name+"/")
					} else {
						entries = append(entries, name)
					}
				}
			}

			result, _ := json.MarshalIndent(entries, "", "  ")
			return string(result), nil
		}).
		Create()
}

func fileInfoTool(validator *pathValidator) *Tool {
	return Build("file_info").
		Description("Get information about a file or directory").
		Param("path", "string", "Path to get info for").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Path string `json:"path"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				params.Path = input
			}

			if err := validator.validate(params.Path); err != nil {
				return "", err
			}

			info, err := os.Stat(params.Path)
			if err != nil {
				return "", fmt.Errorf("failed to stat: %w", err)
			}

			result := map[string]any{
				"name":         info.Name(),
				"size":         info.Size(),
				"is_directory": info.IsDir(),
				"mode":         info.Mode().String(),
				"modified":     info.ModTime().Format("2006-01-02 15:04:05"),
			}

			out, _ := json.MarshalIndent(result, "", "  ")
			return string(out), nil
		}).
		Create()
}

func searchFilesTool(validator *pathValidator) *Tool {
	return Build("search_files").
		Description("Search for files containing text").
		Param("path", "string", "Directory to search in").
		Param("query", "string", "Text to search for").
		OptionalParam("pattern", "string", "File pattern to filter (e.g., '*.go')").
		Handler(func(ctx context.Context, input string) (string, error) {
			var params struct {
				Path    string `json:"path"`
				Query   string `json:"query"`
				Pattern string `json:"pattern"`
			}
			if err := json.Unmarshal([]byte(input), &params); err != nil {
				return "", err
			}

			if err := validator.validate(params.Path); err != nil {
				return "", err
			}

			var matches []map[string]any

			err := filepath.Walk(params.Path, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}

				if params.Pattern != "" {
					matched, _ := filepath.Match(params.Pattern, filepath.Base(path))
					if !matched {
						return nil
					}
				}

				// Skip large files
				if info.Size() > 1024*1024 {
					return nil
				}

				data, err := os.ReadFile(path)
				if err != nil {
					return nil
				}

				content := string(data)
				if strings.Contains(content, params.Query) {
					// Find line numbers
					lines := strings.Split(content, "\n")
					var matchingLines []int
					for i, line := range lines {
						if strings.Contains(line, params.Query) {
							matchingLines = append(matchingLines, i+1)
						}
					}

					relPath, _ := filepath.Rel(params.Path, path)
					matches = append(matches, map[string]any{
						"file":  relPath,
						"lines": matchingLines,
					})
				}

				return nil
			})

			if err != nil {
				return "", err
			}

			result, _ := json.MarshalIndent(matches, "", "  ")
			return string(result), nil
		}).
		Create()
}

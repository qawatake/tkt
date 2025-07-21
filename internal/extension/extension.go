package extension

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/qawatake/tkt/internal/verbose"
)

// Manager manages tkt extensions
type Manager struct{}

// NewManager creates a new extension manager
func NewManager() *Manager {
	return &Manager{}
}

// FindExtensions discovers all tkt extensions in the PATH
func (m *Manager) FindExtensions() ([]Extension, error) {
	extensions := make([]Extension, 0)

	pathEnv := os.Getenv("PATH")
	paths := strings.Split(pathEnv, string(os.PathListSeparator))

	seen := make(map[string]bool)

	for _, path := range paths {
		files, err := os.ReadDir(path)
		if err != nil {
			continue // Skip directories that can't be read
		}

		for _, file := range files {
			name := file.Name()
			if !strings.HasPrefix(name, "tkt-") {
				continue
			}

			// Extract extension name (remove "tkt-" prefix)
			extName := strings.TrimPrefix(name, "tkt-")
			if extName == "" {
				continue
			}

			if seen[extName] {
				continue // Skip duplicates
			}
			seen[extName] = true

			fullPath := filepath.Join(path, name)
			if info, err := os.Stat(fullPath); err == nil && isExecutable(info) {
				extensions = append(extensions, Extension{
					Name: extName,
					Path: fullPath,
				})
			}
		}
	}

	// Sort extensions by name
	sort.Slice(extensions, func(i, j int) bool {
		return extensions[i].Name < extensions[j].Name
	})

	return extensions, nil
}

// Execute runs an extension with the given arguments
func (m *Manager) Execute(name string, args []string) error {
	extensions, err := m.FindExtensions()
	if err != nil {
		return fmt.Errorf("failed to find extensions: %v", err)
	}

	for _, ext := range extensions {
		if ext.Name == name {
			// Filter out the extension name from args if it's there
			filteredArgs := make([]string, 0, len(args))
			for _, arg := range args {
				if arg != name {
					filteredArgs = append(filteredArgs, arg)
				}
			}
			return ext.Execute(filteredArgs)
		}
	}

	return fmt.Errorf("extension '%s' not found", name)
}

// Extension represents a tkt extension
type Extension struct {
	Name string
	Path string
}

// Execute runs the extension with the given arguments
func (e Extension) Execute(args []string) error {
	if verbose.Enabled {
		fmt.Fprintf(os.Stderr, "Executing extension: %s %v\n", e.Path, args)
	}

	cmd := exec.Command(e.Path, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// isExecutable checks if the file is executable
func isExecutable(info os.FileInfo) bool {
	mode := info.Mode()
	return mode.IsRegular() && (mode.Perm()&0111) != 0
}

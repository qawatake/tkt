package extension

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindExtensions(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create some test extensions
	createTestExtension(t, tempDir, "tkt-test1", "#!/bin/bash\necho test1")
	createTestExtension(t, tempDir, "tkt-test2", "#!/bin/bash\necho test2")
	createTestExtension(t, tempDir, "not-tkt-extension", "#!/bin/bash\necho not-tkt")

	// Modify PATH to include our test directory
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tempDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	manager := NewManager()
	extensions, err := manager.FindExtensions()

	assert.NoError(t, err)

	// Should find 2 tkt extensions
	var foundNames []string
	for _, ext := range extensions {
		foundNames = append(foundNames, ext.Name)
	}

	assert.Contains(t, foundNames, "test1")
	assert.Contains(t, foundNames, "test2")
	assert.NotContains(t, foundNames, "not-tkt-extension")
}

func TestExecuteExtension(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Create a test extension that outputs its arguments
	script := `#!/bin/bash
for arg in "$@"; do
    echo "arg: $arg"
done`
	createTestExtension(t, tempDir, "tkt-testargs", script)

	// Modify PATH to include our test directory
	originalPath := os.Getenv("PATH")
	os.Setenv("PATH", tempDir+":"+originalPath)
	defer os.Setenv("PATH", originalPath)

	manager := NewManager()

	// Test finding the extension
	extensions, err := manager.FindExtensions()
	assert.NoError(t, err)

	found := false
	for _, ext := range extensions {
		if ext.Name == "testargs" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find testargs extension")
}

func TestExecuteNonExistentExtension(t *testing.T) {
	manager := NewManager()
	err := manager.Execute("nonexistent", []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "extension 'nonexistent' not found")
}

func createTestExtension(t *testing.T, dir, name, content string) {
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0755)
	assert.NoError(t, err)
}

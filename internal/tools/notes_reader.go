package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tmc/langchaingo/tools"
)

// NotesReader is a tool that can read files from the 'notes' directory.
type NotesReader struct{}

// Statically assert that NotesReader implements the Tool interface.
var _ tools.Tool = NotesReader{}

// Name returns the name of the tool.
func (nr NotesReader) Name() string {
	return "Notes Reader"
}

// Description returns a description of the tool.
func (nr NotesReader) Description() string {
	return `
Use this tool to read the content of a specific note file from the notes directory.
The input should be the exact filename of the note you want to read (e.g., "personal_journal.md").
`
}

// Call reads the content of the specified file from the 'notes' directory.
func (nr NotesReader) Call(_ context.Context, input string) (string, error) {
	// Basic sanitization to prevent directory traversal.
	// We ensure the path is clean and only contains the filename.
	cleanFilename := filepath.Base(input)
	if cleanFilename != input {
		return "", fmt.Errorf("invalid filename provided; please provide only the filename, e.g., 'note1.md'")
	}

	notesDir := "notes"
	filePath := filepath.Join(notesDir, cleanFilename)

	// Check if the file exists.
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		// To help the agent, we can list the available files.
		files, listErr := os.ReadDir(notesDir)
		if listErr != nil {
			return fmt.Sprintf("File '%s' not found.", cleanFilename), nil
		}

		availableFiles := ""
		for _, f := range files {
			if !f.IsDir() {
				availableFiles += "- " + f.Name() + "\n"
			}
		}
		return fmt.Sprintf("File '%s' not found. Available files are:\n%s", cleanFilename, availableFiles), nil
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("error reading file '%s': %w", cleanFilename, err)
	}

	return string(content), nil
}
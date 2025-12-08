package notes

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tmc/langchaingo/tools"
)

type DateFile struct {
	FilePath string
	Time     time.Time
}

const defaultMaxNotes = 5

var (
	datePattern   = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	numberPattern = regexp.MustCompile(`\d+`)
)

// Tool exposes recent notes to the LLM as a callable tool.
type Tool struct {
	notesDir   string
	maxEntries int
}

var _ tools.Tool = (*Tool)(nil)

// NewTool returns a notes tool configured with the notes directory and a sensible default limit.
func NewTool(notesDir string, maxEntries int) *Tool {
	if maxEntries <= 0 {
		maxEntries = defaultMaxNotes
	}
	return &Tool{
		notesDir:   notesDir,
		maxEntries: maxEntries,
	}
}

func (t *Tool) Name() string {
	return "notes"
}

func (t *Tool) Description() string {
	return fmt.Sprintf(
		"Fetch the most recent dated notes from the user's notes directory (default: %d). Optionally pass an integer in the input to choose how many notes to return.",
		t.maxEntries,
	)
}

func (t *Tool) Call(ctx context.Context, input string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if t.notesDir == "" {
		return "", fmt.Errorf("notes directory is not configured")
	}

	amount := t.maxEntries
	if parsed := parseAmount(input); parsed > 0 {
		amount = parsed
	}

	recentNotes, err := GetLastNotes(t.notesDir, amount)
	if err != nil {
		return "", err
	}
	if len(recentNotes) == 0 {
		return "No notes found.", nil
	}

	return PromptFormatNotes(recentNotes), nil
}

func parseAmount(input string) int {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0
	}

	if n, err := strconv.Atoi(trimmed); err == nil && n > 0 {
		return n
	}

	if m := numberPattern.FindString(trimmed); m != "" {
		if n, err := strconv.Atoi(m); err == nil && n > 0 {
			return n
		}
	}
	return 0
}

func PromptFormatNotes(notes []DateFile) string {
	prompt := ""
	for i, note := range notes {
		c, err := os.ReadFile(note.FilePath)
		if err != nil {
			log.Println("Couldn't read note file")
			continue
		}
		prompt += fmt.Sprintf("\nNote %d (%s)\n", i+1, note.Time.Format(time.DateOnly))
		prompt += string(c) + "\n"
	}
	return prompt
}

func GetLastNotes(notesDir string, amount int) ([]DateFile, error) {
	if amount <= 0 {
		amount = defaultMaxNotes
	}

	dirEntries, err := os.ReadDir(notesDir)
	if err != nil {
		log.Printf("Couldn't read note directory: %v ", err)
		return nil, fmt.Errorf("Couldn't read note directory")
	}

	notes := make([]DateFile, 0, amount)

	for _, entry := range dirEntries {
		fileName := entry.Name()
		if !datePattern.MatchString(fileName) {
			continue
		}

		absPath, err := filepath.Abs(filepath.Join(notesDir, fileName))
		if err != nil {
			log.Printf("Couldn't construct abs path: %v", err)
			continue
		}

		nameParts := strings.Split(fileName, ".")
		parsedTime, err := time.Parse(time.DateOnly, nameParts[0])
		if err != nil {
			log.Printf("Skipped this file, couldn't parse date: %v", err)
			continue
		}

		notes = append(notes, DateFile{FilePath: absPath, Time: parsedTime})
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Time.Before(notes[j].Time)
	})

	if len(notes) > amount {
		notes = notes[len(notes)-amount:]
	}

	return notes, nil
}

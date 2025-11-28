package notes

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type DateFile struct {
	FilePath string
	Time     time.Time
}

func PromptFormatNotes(notes []DateFile) string {
	prompt := ""
	for i, note := range notes{
		c, err := os.ReadFile(note.FilePath)
		if err != nil{
			log.Println("Couldn't read note file")
			continue
		}
		prompt += fmt.Sprintf("\nNote %d\n", i+1)
		prompt += string(c) + "\n"
	}
	return prompt
}

func GetLastNotes(notesDir string, amount int) ([]DateFile, error){
	dirEntries, err := os.ReadDir(notesDir)

	if err != nil {
		return nil, fmt.Errorf("Couldn't read note directory")
	}

	notes := []DateFile{}

	for _, entry := range dirEntries {
		r := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
		fileName := entry.Name()
		absPath, err := filepath.Abs(filepath.Join(notesDir, entry.Name()))
		if err != nil{
			log.Printf("Couldn't construct abs path: %v", err)
			continue
		}
		if m := r.FindString(fileName); m != "" {
			f := strings.Split(fileName, ".")
			time, err := time.Parse(time.DateOnly, f[0])
			if err != nil {
				log.Printf("Skipped this file, couldn't parse date: %v", err)
				continue
			}
			notes = append(notes, DateFile{FilePath: absPath, Time: time})
			sort.Slice(notes, func(i, j int) bool {
				return notes[i].Time.Before(notes[j].Time)
			})

			if len(notes) > amount {
				notes = notes[1:]
			}
		}
	}
	return notes, nil
}

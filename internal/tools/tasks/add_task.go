package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	gtasks "google.golang.org/api/tasks/v1"

	"github.com/tmc/langchaingo/tools"
)

// AddTask creates a new task in Google Tasks.
type AddTask struct {
	credFile string
}

var _ tools.Tool = &AddTask{}

func NewAddTask(credFile string) *AddTask {
	return &AddTask{
		credFile: credFile,
	}
}

func (a *AddTask) Name() string {
	return "tasks_add"
}

func (a *AddTask) Description() string {
	return `Add a task to Google Tasks.

Input must be a stringified JSON object like:
{
  "title": "Buy groceries",
  "notes": "Milk, eggs, bread",
  "start_time": "2025-12-10T09:00:00-05:00",
  "due": "2025-12-10",
  "status": "needsAction",
  "task_list_id": "@default"
}

Fields:
- title (string, required): task title.
- notes (string, optional): additional details.
- start_time (string, optional): RFC3339 timestamp or YYYY-MM-DD. Stored in the task and echoed in notes.
- due (string, optional): RFC3339 timestamp or YYYY-MM-DD.
- status (string, optional): needsAction or completed. Defaults to needsAction.
- task_list_id (string, optional): Task list id; omit for @default.`
}

func (a *AddTask) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Task title (required).",
			},
			"notes": map[string]interface{}{
				"type":        "string",
				"description": "Optional task notes.",
			},
			"start_time": map[string]interface{}{
				"type":        "string",
				"description": "RFC3339 timestamp or YYYY-MM-DD.",
			},
			"due": map[string]interface{}{
				"type":        "string",
				"description": "RFC3339 timestamp or YYYY-MM-DD.",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "Task status: needsAction or completed.",
				"enum":        []string{"needsAction", "completed"},
			},
			"task_list_id": map[string]interface{}{
				"type":        "string",
				"description": "Task list id; omit to use the default list (@default).",
			},
		},
		"required": []string{"title"},
	}
}

func (a *AddTask) Call(ctx context.Context, input string) (string, error) {
	ctx = ensureContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	payload, err := parseAddTaskInput(input)
	if err != nil {
		return "", err
	}

	srv, err := newTasksService(ctx, a.credFile)
	if err != nil {
		return "", err
	}

	taskListID := strings.TrimSpace(payload.TaskListID)
	if taskListID == "" {
		taskListID = "@default"
	}

	task := &gtasks.Task{
		Title: strings.TrimSpace(payload.Title),
		Notes: strings.TrimSpace(payload.Notes),
	}

	var startNormalized string
	if payload.StartTime != "" {
		startNormalized, err = normalizeDue(payload.StartTime)
		if err != nil {
			return "", fmt.Errorf("invalid start_time: %w", err)
		}
	}

	if payload.Status != "" {
		task.Status = payload.Status
	}

	if payload.Due != "" {
		normalizedDue, err := normalizeDue(payload.Due)
		if err != nil {
			return "", fmt.Errorf("invalid due: %w", err)
		}
		task.Due = normalizedDue
	} else if startNormalized != "" {
		// Google Tasks has no start time; store start in Due for ordering.
		task.Due = startNormalized
	}

	if startNormalized != "" {
		if task.Notes != "" {
			task.Notes += "\n"
		}
		task.Notes += fmt.Sprintf("Start: %s", startNormalized)
	}

	created, err := srv.Tasks.Insert(taskListID, task).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("unable to create task: %w", err)
	}

	status := strings.TrimSpace(created.Status)
	if status == "" {
		status = "needsAction"
	}

	return fmt.Sprintf("Created task \"%s\" (status: %s, id: %s)", created.Title, status, created.Id), nil
}

type addTaskInput struct {
	Title      string `json:"title"`
	Notes      string `json:"notes"`
	StartTime  string `json:"start_time"`
	Due        string `json:"due"`
	Status     string `json:"status"`
	TaskListID string `json:"task_list_id"`
}

func parseAddTaskInput(raw string) (addTaskInput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return addTaskInput{}, fmt.Errorf("provide task details as a JSON object in the tool input")
	}

	var payload addTaskInput
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return addTaskInput{}, fmt.Errorf("invalid add task payload; expected a JSON object: %w", err)
	}

	if strings.TrimSpace(payload.Title) == "" {
		return addTaskInput{}, fmt.Errorf("title is required to create a task")
	}

	if payload.Status != "" && payload.Status != "needsAction" && payload.Status != "completed" {
		return addTaskInput{}, fmt.Errorf("status must be needsAction or completed when provided")
	}

	return payload, nil
}

func normalizeDue(input string) (string, error) {
	value := strings.TrimSpace(input)
	if value == "" {
		return "", nil
	}

	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t.Format(time.RFC3339), nil
	}

	if t, err := time.Parse(time.DateOnly, value); err == nil {
		return t.Format(time.RFC3339), nil
	}

	return "", fmt.Errorf("could not parse due date %q; use RFC3339 or YYYY-MM-DD", input)
}

package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	gtasks "google.golang.org/api/tasks/v1"

	"github.com/tmc/langchaingo/tools"
)

// ListTasks retrieves tasks from a Google Tasks list (defaults to @default).
type ListTasks struct {
	credFile string
}

var _ tools.Tool = &ListTasks{}

func NewListTasks(credFile string) *ListTasks {
	return &ListTasks{
		credFile: credFile,
	}
}

func (l *ListTasks) Name() string {
	return "tasks"
}

func (l *ListTasks) Description() string {
	return `List tasks from Google Tasks. Defaults to the primary list (@default).

Optional fields:
- task_list_id: specific task list id. Omit to use @default.
- include_completed: set true to include completed/hidden tasks.
- max_results: limit number of tasks returned (1-100, default 25).`
}

func (l *ListTasks) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task_list_id": map[string]interface{}{
				"type":        "string",
				"description": "Task list id; omit to use the default list (@default).",
			},
			"include_completed": map[string]interface{}{
				"type":        "boolean",
				"description": "Include completed tasks when true.",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum tasks to return (1-100, default 25).",
			},
		},
		"required": []string{},
	}
}

func (l *ListTasks) Call(ctx context.Context, input string) (string, error) {
	ctx = ensureContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	payload, err := parseListTasksInput(input)
	if err != nil {
		return "", err
	}

	srv, err := newTasksService(ctx, l.credFile)
	if err != nil {
		return "", err
	}

	taskListID := strings.TrimSpace(payload.TaskListID)
	if taskListID == "" {
		taskListID = "@default"
	}

	maxResults := int64(25)
	switch {
	case payload.MaxResults > 100:
		maxResults = 100
	case payload.MaxResults > 0:
		maxResults = int64(payload.MaxResults)
	}

	includeCompleted := payload.IncludeCompleted

	listCall := srv.Tasks.List(taskListID).
		ShowCompleted(includeCompleted).
		ShowHidden(includeCompleted).
		MaxResults(maxResults).
		Context(ctx)

	tasksResp, err := listCall.Do()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve tasks: %w", err)
	}

	var listTitle string = taskListID
	if tl, err := srv.Tasklists.Get(taskListID).Context(ctx).Do(); err == nil && strings.TrimSpace(tl.Title) != "" {
		listTitle = tl.Title
	}

	if len(tasksResp.Items) == 0 {
		return fmt.Sprintf("No tasks found for \"%s\".", listTitle), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Tasks in \"%s\":\n", listTitle))

	for _, t := range tasksResp.Items {
		title := strings.TrimSpace(t.Title)
		if title == "" {
			title = "(no title)"
		}
		status := strings.TrimSpace(t.Status)
		if status == "" {
			status = "needsAction"
		}
		due := strings.TrimSpace(t.Due)

		b.WriteString("- ")
		b.WriteString(title)
		if due != "" {
			b.WriteString(fmt.Sprintf(" | due: %s", due))
		}
		b.WriteString(fmt.Sprintf(" | status: %s | id: %s\n", status, t.Id))
	}

	return b.String(), nil
}

type listTasksInput struct {
	TaskListID       string `json:"task_list_id"`
	IncludeCompleted bool   `json:"include_completed"`
	MaxResults       int    `json:"max_results"`
}

func parseListTasksInput(raw string) (listTasksInput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return listTasksInput{}, nil
	}

	var payload listTasksInput
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return listTasksInput{}, fmt.Errorf("invalid list tasks payload; expected a JSON object: %w", err)
	}

	if payload.MaxResults < 0 {
		return listTasksInput{}, fmt.Errorf("max_results must be zero or positive")
	}

	return payload, nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func newTasksService(ctx context.Context, credFile string) (*gtasks.Service, error) {
	cred, err := resolveCredential(ctx, credFile)
	if err != nil {
		return nil, err
	}
	return gtasks.NewService(ctx, cred)
}

func resolveCredential(ctx context.Context, credFile string) (option.ClientOption, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	tokenSource := ctx.Value("OauthTokenSource")
	if tokenSource == nil && strings.TrimSpace(credFile) == "" {
		return nil, fmt.Errorf("authentication for google tasks tool is not configured yet")
	}

	if tokenSource != nil {
		ts, ok := tokenSource.(oauth2.TokenSource)
		if !ok || ts == nil {
			return nil, fmt.Errorf("context value OauthTokenSource is not valid")
		}
		return option.WithTokenSource(ts), nil
	}

	return option.WithCredentialsFile(credFile), nil
}

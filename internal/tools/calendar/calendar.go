package calendar

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/tmc/langchaingo/callbacks"
	"github.com/tmc/langchaingo/tools"
)

type Calendar struct {
	credFile         string
	CallbacksHandler callbacks.Handler
}

var _ tools.Tool = &Calendar{}

func New(credFile string) *Calendar {
	return &Calendar{
		credFile:    credFile,
	}
}

func (c *Calendar) Name() string {
	return "calendar"
}


func (c *Calendar) Description() string {
	return `Useful for getting events from the users google calendar.`
}

func (c *Calendar) Call(ctx context.Context, input string) (string, error) {
	var cred option.ClientOption
	if ctx.Value("OauthTokenSource") == nil && c.credFile == "" {
		return "", fmt.Errorf("authentication for calendar tool is not configured yet")
	} else if ctx.Value("OauthTokenSource")!= nil {
		t, o := ctx.Value("OauthTokenSource").(oauth2.TokenSource)
		if !o{
			return "", fmt.Errorf("Context value OauthTokenSource is not valid")
		}
		cred = option.WithTokenSource(t)
	} else if c.credFile != "" {
		cred = option.WithCredentialsFile(c.credFile)
	}

	srv, err := calendar.NewService(context.Background(), cred)
	if err != nil {
		return "", fmt.Errorf("Unable to create Calendar service: %v", err)
	}

	// Define the time window you want to query.
	start := time.Now().Format(time.RFC3339)                       // now
	end := time.Now().Add(3 * 24 * time.Hour).Format(time.RFC3339) // next 24 h

	eventsCall := srv.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(start).
		TimeMax(end).
		OrderBy("startTime")

	// Execute the request.
	events, err := eventsCall.Do()
	if err != nil {
		return "", fmt.Errorf("Unable to retrieve events: %v", err)
	}

	// Print the events.
	if len(events.Items) == 0 {
		return "No upcoming events found.", nil
	}

	var result string
	for _, e := range events.Items {
		start := e.Start.DateTime
		if start == "" {
			// All‑day events use the Date field instead.
			start = e.Start.Date
		}
		result += fmt.Sprintf("%s – %s\n", start, e.Summary)
	}
	return result, nil
}

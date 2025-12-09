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

// Calendar lists upcoming events for the user.
type Calendar struct {
	credFile         string
	CallbacksHandler callbacks.Handler
}

var (
	_ tools.Tool = &Calendar{}
)

func New(credFile string) *Calendar {
	return &Calendar{
		credFile: credFile,
	}
}

func (c *Calendar) Name() string {
	return "calendar"
}

func (c *Calendar) Description() string {
	return `List the user's upcoming Google Calendar events for the next 72 hours, including each event's id for follow-up edits.`
}

func (c *Calendar) Call(ctx context.Context, input string) (string, error) {
	ctx = ensureContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	srv, err := newCalendarService(ctx, c.credFile)
	if err != nil {
		return "", err
	}

	start := time.Now().Format(time.RFC3339)
	end := time.Now().Add(3 * 24 * time.Hour).Format(time.RFC3339)

	eventsCall := srv.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(start).
		TimeMax(end).
		OrderBy("startTime").
		Context(ctx)

	events, err := eventsCall.Do()
	if err != nil {
		return "", fmt.Errorf("unable to retrieve events: %w", err)
	}

	if len(events.Items) == 0 {
		return "No upcoming events found.", nil
	}

	var result string
	for _, e := range events.Items {
		start := e.Start.DateTime
		if start == "" {
			start = e.Start.Date
		}
		result += fmt.Sprintf("%s – %s (id: %s)\n", start, e.Summary, e.Id)
	}
	return result, nil
}

func ensureContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func newCalendarService(ctx context.Context, credFile string) (*calendar.Service, error) {
	cred, err := resolveCredential(ctx, credFile)
	if err != nil {
		return nil, err
	}
	return calendar.NewService(ctx, cred)
}

func resolveCredential(ctx context.Context, credFile string) (option.ClientOption, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	tokenSource := ctx.Value("OauthTokenSource")
	if tokenSource == nil && credFile == "" {
		return nil, fmt.Errorf("authentication for calendar tool is not configured yet")
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

package calendar

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"

	"github.com/tmc/langchaingo/tools"
	"github.com/tmc/langchaingo/callbacks"
)

type Calendar struct{
	auth option.ClientOption
	CallbacksHandler callbacks.Handler
}

var _ tools.Tool = Calendar{}

func New(authType option.ClientOption) tools.Tool{ 
	return Calendar{
		auth: authType,
	} 
}

func (c Calendar) Description() string {
	return `Useful for getting the result of a math expression. 
	The input to this tool should be a valid mathematical expression that could be executed by a starlark evaluator.`
}

func (c Calendar) Name() string {
	return "calendar"
}

func (c Calendar) Call(ctx context.Context, input string) (string, error) {
	// Create a Calendar service client.
	srv, err := calendar.NewService(context.Background(), c.auth)
	if err != nil {
		log.Fatalf("Unable to create Calendar service: %v", err)
	}

	// Define the time window you want to query.
	start := time.Now().Format(time.RFC3339)                     // now
	end := time.Now().Add(3 * 24 * time.Hour).Format(time.RFC3339) // next 24 h

	// Build the request.
	eventsCall := srv.Events.List("primary").
		ShowDeleted(false).
		SingleEvents(true).
		TimeMin(start).
		TimeMax(end).
		OrderBy("startTime")

	// Execute the request.
	events, err := eventsCall.Do()
	if err != nil {
		log.Fatalf("Unable to retrieve events: %v", err)
	}

	// Print the events.
	if len(events.Items) == 0 {
		fmt.Println("No upcoming events found.")
	} else {
		fmt.Println("Upcoming events:")
		for _, e := range events.Items {
			start := e.Start.DateTime
			if start == "" {
				// All‑day events use the Date field instead.
				start = e.Start.Date
			}
			fmt.Printf("%s – %s\n", start, e.Summary)
		}
	}
	return "", nil
}

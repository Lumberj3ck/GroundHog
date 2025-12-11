package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"

	"github.com/tmc/langchaingo/tools"
)

// AddEvent creates new events in the user's Google Calendar.
type AddEvent struct {
	credFile string
}

var _ tools.Tool = &AddEvent{}

func NewAddEvent(credFile string) *AddEvent {
	return &AddEvent{
		credFile: credFile,
	}
}

func (a *AddEvent) Name() string {
	return "calendar_add_event"
}

func (a *AddEvent) Description() string {
	return `Add a new event to the user's Google Calendar.

Input must be a stringified JSON object like:
{
  "summary": "Team sync",
  "start_time": "2025-12-09T10:00:00-05:00",
  "end_time": "2025-12-09T10:30:00-05:00",
  "duration_minutes": 30,
  "description": "Discuss project status",
  "location": "Zoom",
  "time_zone": "America/New_York"
}

Fields:
- summary (string, required): event title.
- start_time (string, required): RFC3339 timestamp or YYYY-MM-DD for all-day events.
- end_time (string, optional): RFC3339 timestamp; omit when using duration_minutes.
- duration_minutes (integer, optional): length in minutes when end_time is omitted.
- description (string, optional)
- location (string, optional)
- time_zone (string, optional): IANA name, e.g., "America/New_York". `
}

// Parameters exposes the structured schema for tool calling.
func (a *AddEvent) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"summary": map[string]interface{}{
				"type":        "string",
				"description": "Event title (required).",
			},
			"start_time": map[string]interface{}{
				"type":        "string",
				"description": "RFC3339 timestamp or YYYY-MM-DD for all-day events (required).",
			},
			"end_time": map[string]interface{}{
				"type":        "string",
				"description": "RFC3339 end time; omit when duration_minutes is used.",
			},
			"duration_minutes": map[string]interface{}{
				"type":        "integer",
				"description": "Length in minutes when end_time is omitted.",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Optional event description.",
			},
			"location": map[string]interface{}{
				"type":        "string",
				"description": "Optional location such as Zoom link or room.",
			},
			"time_zone": map[string]interface{}{
				"type":        "string",
				"description": "IANA time zone, e.g., America/New_York.",
			},
		},
		"required": []string{"summary", "start_time"},
	}
}

func (a *AddEvent) Call(ctx context.Context, input string) (string, error) {
	ctx = ensureContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	payload, err := parseAddEventInput(input)
	if err != nil {
		return "", err
	}

	srv, err := newCalendarService(ctx, a.credFile)
	if err != nil {
		return "", err
	}

	start, end, allDay, tz, err := prepareEventTimes(payload)
	if err != nil {
		return "", err
	}

	event := &calendar.Event{
		Summary:     payload.Summary,
		Description: payload.Description,
		Location:    payload.Location,
	}

	if allDay {
		event.Start = &calendar.EventDateTime{
			Date: start.Format(time.DateOnly),
		}
		event.End = &calendar.EventDateTime{
			Date: end.Format(time.DateOnly),
		}
	} else {
		event.Start = &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: tz,
		}
		event.End = &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: tz,
		}
	}

	insertCall := srv.Events.Insert("primary", event).Context(ctx)
	created, err := insertCall.Do()
	if err != nil {
		return "", fmt.Errorf("unable to create event: %w", err)
	}

	startDisplay := payload.StartTime
	if created.Start != nil {
		if created.Start.DateTime != "" {
			startDisplay = created.Start.DateTime
		} else if created.Start.Date != "" {
			startDisplay = created.Start.Date
		}
	}
	endDisplay := payload.EndTime
	if created.End != nil {
		if created.End.DateTime != "" {
			endDisplay = created.End.DateTime
		} else if created.End.Date != "" {
			endDisplay = created.End.Date
		}
	}

	if created.HtmlLink != "" {
		return fmt.Sprintf("Created calendar event \"%s\" (%s → %s). Link: %s", created.Summary, startDisplay, endDisplay, created.HtmlLink), nil
	}
	return fmt.Sprintf("Created calendar event \"%s\" (%s → %s).", created.Summary, startDisplay, endDisplay), nil
}

type addEventInput struct {
	Summary         string `json:"summary"`
	Description     string `json:"description,omitempty"`
	StartTime       string `json:"start_time"`
	EndTime         string `json:"end_time,omitempty"`
	DurationMinutes int    `json:"duration_minutes,omitempty"`
	TimeZone        string `json:"time_zone,omitempty"`
	Location        string `json:"location,omitempty"`
}

func parseAddEventInput(raw string) (addEventInput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return addEventInput{}, fmt.Errorf("provide event details as a JSON object in the tool input")
	}

	var payload addEventInput
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return addEventInput{}, fmt.Errorf("invalid add event payload; expected a JSON object: %w", err)
	}
	return validateAddEventInput(payload)
}

func prepareEventTimes(in addEventInput) (time.Time, time.Time, bool, string, error) {
	start, startAllDay, err := parseTime(in.StartTime, in.TimeZone)
	if err != nil {
		return time.Time{}, time.Time{}, false, "", fmt.Errorf("invalid start_time: %w", err)
	}

	var end time.Time
	var endAllDay bool
	if in.EndTime != "" {
		end, endAllDay, err = parseTime(in.EndTime, in.TimeZone)
		if err != nil {
			return time.Time{}, time.Time{}, false, "", fmt.Errorf("invalid end_time: %w", err)
		}
		if startAllDay != endAllDay {
			return time.Time{}, time.Time{}, false, "", fmt.Errorf("start_time and end_time must both be date-only or both include time")
		}
	} else if startAllDay {
		end = start.AddDate(0, 0, 1)
		endAllDay = true
	} else if in.DurationMinutes > 0 {
		end = start.Add(time.Duration(in.DurationMinutes) * time.Minute)
	} else {
		end = start.Add(time.Hour)
	}

	if end.Before(start) || end.Equal(start) {
		return time.Time{}, time.Time{}, false, "", fmt.Errorf("end time must be after start time")
	}

	timeZone := strings.TrimSpace(in.TimeZone)
	if startAllDay {
		return start, end, true, "", nil
	}
	return start, end, false, timeZone, nil
}

func parseTime(value, timeZone string) (time.Time, bool, error) {
	if value == "" {
		return time.Time{}, false, fmt.Errorf("time value is empty")
	}

	trimmedTZ := strings.TrimSpace(timeZone)

	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, false, nil
	}

	if t, err := time.Parse(time.DateOnly, value); err == nil {
		return t, true, nil
	}

	loc := time.Local
	if trimmedTZ != "" {
		var err error
		loc, err = time.LoadLocation(trimmedTZ)
		if err != nil {
			return time.Time{}, false, fmt.Errorf("invalid time_zone %q: %w", timeZone, err)
		}
	}

	if t, err := time.ParseInLocation("2006-01-02T15:04:05", value, loc); err == nil {
		return t, false, nil
	}

	if t, err := time.ParseInLocation("2006-01-02T15:04", value, loc); err == nil {
		return t, false, nil
	}

	if t, err := time.ParseInLocation("2006-01-02 15:04", value, loc); err == nil {
		return t, false, nil
	}

	return time.Time{}, false, fmt.Errorf("could not parse time %q; use RFC3339, YYYY-MM-DD for all-day, YYYY-MM-DDTHH:MM[:SS], or YYYY-MM-DD HH:MM", value)
}

func validateAddEventInput(payload addEventInput) (addEventInput, error) {
	if strings.TrimSpace(payload.Summary) == "" {
		return addEventInput{}, fmt.Errorf("summary is required to create an event")
	}
	if strings.TrimSpace(payload.StartTime) == "" {
		return addEventInput{}, fmt.Errorf("start_time is required to create an event")
	}
	return payload, nil
}

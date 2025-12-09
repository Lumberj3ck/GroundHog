package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

// EditEvent updates an existing event in the user's Google Calendar.
type EditEvent struct {
	credFile string
}

var _ = EditEvent{}

func NewEditEvent(credFile string) *EditEvent {
	return &EditEvent{
		credFile: credFile,
	}
}

func (e *EditEvent) Name() string {
	return "calendar_edit_event"
}

func (e *EditEvent) Description() string {
	return `Edit an existing Google Calendar event. Expect a JSON string with: event_id (required), summary (optional), start_time (optional, RFC3339 or YYYY-MM-DD for all-day), end_time (optional, RFC3339), duration_minutes (optional when end_time is omitted), description (optional), location (optional), time_zone (optional IANA, e.g. "America/New_York"). Provide at least one field to update.`
}

func (e *EditEvent) Call(ctx context.Context, input string) (string, error) {
	ctx = ensureContext(ctx)
	if err := ctx.Err(); err != nil {
		return "", err
	}

	payload, err := parseEditEventInput(input)
	if err != nil {
		return "", err
	}

	srv, err := newCalendarService(ctx, e.credFile)
	if err != nil {
		return "", err
	}

	existing, err := srv.Events.Get("primary", payload.EventID).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("unable to fetch event %q: %w", payload.EventID, err)
	}

	updated := &calendar.Event{
		Summary:     existing.Summary,
		Description: existing.Description,
		Location:    existing.Location,
		Start:       existing.Start,
		End:         existing.End,
	}

	if payload.Summary != nil {
		updated.Summary = strings.TrimSpace(*payload.Summary)
	}
	if payload.Description != nil {
		updated.Description = strings.TrimSpace(*payload.Description)
	}
	if payload.Location != nil {
		updated.Location = strings.TrimSpace(*payload.Location)
	}

	timesChanged := payload.StartTime != nil || payload.EndTime != nil || payload.DurationMinutes != nil || payload.TimeZone != nil
	if timesChanged {
		start, end, allDay, tz, err := computeEditedTimes(existing, payload)
		if err != nil {
			return "", err
		}
		if allDay {
			updated.Start = &calendar.EventDateTime{
				Date: start.Format(time.DateOnly),
			}
			updated.End = &calendar.EventDateTime{
				Date: end.Format(time.DateOnly),
			}
		} else {
			updated.Start = &calendar.EventDateTime{
				DateTime: start.Format(time.RFC3339),
				TimeZone: tz,
			}
			updated.End = &calendar.EventDateTime{
				DateTime: end.Format(time.RFC3339),
				TimeZone: tz,
			}
		}
	}

	saved, err := srv.Events.Update("primary", payload.EventID, updated).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("unable to update event: %w", err)
	}

	startDisplay := stringifyEventTime(saved.Start, payload.StartTime)
	endDisplay := stringifyEventTime(saved.End, payload.EndTime)

	if saved.HtmlLink != "" {
		return fmt.Sprintf("Updated calendar event \"%s\" (%s → %s). Link: %s", saved.Summary, startDisplay, endDisplay, saved.HtmlLink), nil
	}
	return fmt.Sprintf("Updated calendar event \"%s\" (%s → %s).", saved.Summary, startDisplay, endDisplay), nil
}

type editEventInput struct {
	EventID         string  `json:"event_id"`
	Summary         *string `json:"summary,omitempty"`
	Description     *string `json:"description,omitempty"`
	StartTime       *string `json:"start_time,omitempty"`
	EndTime         *string `json:"end_time,omitempty"`
	DurationMinutes *int    `json:"duration_minutes,omitempty"`
	TimeZone        *string `json:"time_zone,omitempty"`
	Location        *string `json:"location,omitempty"`
}

func parseEditEventInput(raw string) (editEventInput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return editEventInput{}, fmt.Errorf("provide event details as JSON in the tool input")
	}

	var payload editEventInput
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return editEventInput{}, fmt.Errorf("invalid edit event payload; expected JSON: %w", err)
	}

	payload.EventID = strings.TrimSpace(payload.EventID)
	if payload.EventID == "" {
		return editEventInput{}, fmt.Errorf("event_id is required to edit an event")
	}

	if payload.Summary == nil &&
		payload.Description == nil &&
		payload.StartTime == nil &&
		payload.EndTime == nil &&
		payload.DurationMinutes == nil &&
		payload.TimeZone == nil &&
		payload.Location == nil {
		return editEventInput{}, fmt.Errorf("provide at least one field to update")
	}

	if payload.Summary != nil && strings.TrimSpace(*payload.Summary) == "" {
		return editEventInput{}, fmt.Errorf("summary cannot be empty when provided")
	}
	if payload.DurationMinutes != nil && *payload.DurationMinutes <= 0 {
		return editEventInput{}, fmt.Errorf("duration_minutes must be greater than 0")
	}

	return payload, nil
}

func computeEditedTimes(existing *calendar.Event, in editEventInput) (time.Time, time.Time, bool, string, error) {
	existingStart := eventTimeString(existing.Start)
	existingEnd := eventTimeString(existing.End)

	startInput := pickString(in.StartTime, existingStart)
	endInput := pickString(in.EndTime, existingEnd)
	if startInput == "" {
		return time.Time{}, time.Time{}, false, "", fmt.Errorf("existing event has no start time; please provide start_time")
	}

	tz := pickString(in.TimeZone, existingTimezone(existing))

	start, startAllDay, err := parseTime(startInput, tz)
	if err != nil {
		return time.Time{}, time.Time{}, false, "", fmt.Errorf("invalid start_time: %w", err)
	}

	var end time.Time
	var endAllDay bool
	switch {
	case in.EndTime != nil:
		end, endAllDay, err = parseTime(endInput, tz)
		if err != nil {
			return time.Time{}, time.Time{}, false, "", fmt.Errorf("invalid end_time: %w", err)
		}
	case in.DurationMinutes != nil:
		if startAllDay {
			end = start.AddDate(0, 0, 1)
			endAllDay = true
		} else {
			end = start.Add(time.Duration(*in.DurationMinutes) * time.Minute)
		}
	default:
		end, endAllDay, err = parseTime(endInput, tz)
		if err != nil {
			return time.Time{}, time.Time{}, false, "", fmt.Errorf("invalid end_time: %w", err)
		}
	}

	if startAllDay != endAllDay {
		return time.Time{}, time.Time{}, false, "", fmt.Errorf("start_time and end_time must both be date-only or both include time")
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, false, "", fmt.Errorf("end time must be after start time")
	}

	if startAllDay {
		return start, end, true, "", nil
	}
	return start, end, false, tz, nil
}

func eventTimeString(t *calendar.EventDateTime) string {
	if t == nil {
		return ""
	}
	if strings.TrimSpace(t.DateTime) != "" {
		return t.DateTime
	}
	return strings.TrimSpace(t.Date)
}

func existingTimezone(e *calendar.Event) string {
	if e == nil {
		return ""
	}
	if e.Start != nil && strings.TrimSpace(e.Start.TimeZone) != "" {
		return strings.TrimSpace(e.Start.TimeZone)
	}
	if e.End != nil && strings.TrimSpace(e.End.TimeZone) != "" {
		return strings.TrimSpace(e.End.TimeZone)
	}
	return ""
}

func pickString(value *string, fallback string) string {
	if value != nil {
		return strings.TrimSpace(*value)
	}
	return strings.TrimSpace(fallback)
}

func stringifyEventTime(t *calendar.EventDateTime, provided *string) string {
	if t == nil {
		if provided != nil {
			return strings.TrimSpace(*provided)
		}
		return ""
	}
	if t.DateTime != "" {
		return t.DateTime
	}
	if t.Date != "" {
		return t.Date
	}
	if provided != nil {
		return strings.TrimSpace(*provided)
	}
	return ""
}

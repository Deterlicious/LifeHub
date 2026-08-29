package agenda

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
)

func TestBuildOrdersMixedAgendaAndMarshalsDiscriminatedItems(t *testing.T) {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2026, time.August, 19, 3, 0, 0, 0, time.UTC)
	allDayStart := "2026-08-19"
	allDayEnd := "2026-08-20"
	timedAt := time.Date(2026, time.August, 20, 1, 0, 0, 0, time.UTC)
	billAt := timedAt.Add(time.Hour)
	taskAt := timedAt.Add(2 * time.Hour)
	priorityHigh := domain.PriorityHigh
	priorityLow := domain.PriorityLow

	response := Build(
		"2026-08-20", "2026-08-21", "2026-08-19", now, location, "Asia/Jakarta",
		[]domain.Task{
			{ID: "task-low", Title: "Low", Priority: priorityLow, DueAt: &taskAt, CreatedAt: created},
			{ID: "task-high", Title: "High", Priority: priorityHigh, DueAt: &taskAt, CreatedAt: created},
		},
		[]domain.Event{
			{ID: "timed", Title: "Timed", Timezone: "Asia/Jakarta", StartsAt: &timedAt, CreatedAt: created},
			{ID: "all-day", Title: "All day", AllDay: true, Timezone: "Asia/Jakarta", StartsOn: &allDayStart, EndsOn: &allDayEnd, CreatedAt: created},
		},
		[]domain.Bill{{ID: "bill", Title: "Bill", Amount: 1000, Currency: "IDR", DueAt: billAt, CreatedAt: created}},
		[]domain.Document{{ID: "document", Name: "SIM", Category: "license", ExpiresOn: "2026-08-20", CreatedAt: created}},
	)

	got := make([]string, 0, len(response.Items))
	for _, item := range response.Items {
		got = append(got, item.ID)
		if item.DisplayOn != "2026-08-20" {
			t.Fatalf("item %q display_on = %q", item.ID, item.DisplayOn)
		}
	}
	want := []string{"all-day", "document", "timed", "bill", "task-high", "task-low"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if response.Summary != (Summary{Total: 6, Tasks: 2, Events: 2, Bills: 1, Documents: 1}) {
		t.Fatalf("summary = %#v", response.Summary)
	}

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, exists := decoded.Items[1]["due_at"]; exists {
		t.Fatalf("document leaked task/bill field: %#v", decoded.Items[1])
	}
	if decoded.Items[1]["title"] != "SIM" || decoded.Items[1]["days_until_expiry"] != float64(1) {
		t.Fatalf("document DTO = %#v", decoded.Items[1])
	}
	if _, exists := decoded.Items[4]["completed_at"]; !exists || decoded.Items[4]["completed_at"] != nil {
		t.Fatalf("task DTO must explicitly expose null completion: %#v", decoded.Items[4])
	}
	if _, exists := decoded.Items[0]["priority"]; exists {
		t.Fatalf("event leaked task priority: %#v", decoded.Items[0])
	}
}

func TestSortEventsUsesOverlapDisplayDateAndStableScheduleOrder(t *testing.T) {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	before := time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)
	ends := time.Date(2026, time.August, 20, 4, 0, 0, 0, time.UTC)
	starts := time.Date(2026, time.August, 20, 1, 0, 0, 0, time.UTC)
	allDay := "2026-08-20"
	events := []domain.Event{
		{ID: "timed", StartsAt: &starts},
		{ID: "continuing", StartsAt: &before, EndsAt: &ends},
		{ID: "all-day", AllDay: true, StartsOn: &allDay},
	}
	SortEvents(events, "2026-08-20", location)
	got := []string{events[0].ID, events[1].ID, events[2].ID}
	want := []string{"all-day", "continuing", "timed"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

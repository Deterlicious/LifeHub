package today

import (
	"fmt"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
)

func TestBuildOrdersBucketsAndTieBreaksDeterministically(t *testing.T) {
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	now := start.Add(90 * time.Minute)
	overdue := start.Add(-time.Hour)
	earlierToday := start.Add(time.Hour)
	due := start.Add(2 * time.Hour)
	completed := start.Add(3 * time.Hour)
	created := start.Add(-24 * time.Hour)

	tasks := []domain.Task{
		{ID: "b", Title: "Normal tie", Priority: domain.PriorityNormal, DueAt: &due, CreatedAt: created},
		{ID: "completed", Title: "Completed", Priority: domain.PriorityNormal, CompletedAt: &completed, CreatedAt: created},
		{ID: "anytime", Title: "Anytime", Priority: domain.PriorityHigh, CreatedAt: created},
		{ID: "overdue", Title: "Overdue", Priority: domain.PriorityLow, DueAt: &overdue, CreatedAt: created},
		{ID: "earlier-today", Title: "Earlier today", Priority: domain.PriorityNormal, DueAt: &earlierToday, CreatedAt: created},
		{ID: "a", Title: "High tie", Priority: domain.PriorityHigh, DueAt: &due, CreatedAt: created},
	}

	response := Build("2026-08-19", "Asia/Jakarta", now, start, end, tasks)
	got := make([]string, 0, len(response.Items))
	for _, item := range response.Items {
		got = append(got, item.ID)
	}
	want := []string{"overdue", "earlier-today", "a", "b", "anytime", "completed"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if response.Summary.Open != 5 || response.Summary.Completed != 1 {
		t.Fatalf("summary = %#v", response.Summary)
	}
}

func TestBuildDoesNotSilentlyHideAnytimeTasks(t *testing.T) {
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	tasks := make([]domain.Task, 0, 25)
	for index := range 25 {
		tasks = append(tasks, domain.Task{
			ID: fmt.Sprintf("%02d", index), Title: "Anytime", Priority: domain.PriorityNormal, CreatedAt: start,
		})
	}
	response := Build("2026-08-19", "Asia/Jakarta", start, start, start.Add(24*time.Hour), tasks)
	if len(response.Items) != len(tasks) || response.Summary.Open != len(tasks) {
		t.Fatalf("items = %d, open = %d, want %d", len(response.Items), response.Summary.Open, len(tasks))
	}
}

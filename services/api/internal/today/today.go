package today

import (
	"cmp"
	"slices"
	"time"

	"lifehub/services/api/internal/domain"
)

const (
	BucketOverdue        = "overdue"
	BucketDueToday       = "due_today"
	BucketAnytime        = "anytime"
	BucketCompletedToday = "completed_today"
)

type Item struct {
	Kind        string     `json:"kind"`
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Notes       *string    `json:"notes"`
	Priority    string     `json:"priority"`
	DueAt       *time.Time `json:"due_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Status      string     `json:"status"`
	Bucket      string     `json:"bucket"`
	Urgency     string     `json:"urgency"`
	createdAt   time.Time
}

type Summary struct {
	Open      int `json:"open"`
	Completed int `json:"completed"`
}

type Response struct {
	Date     string  `json:"date"`
	Timezone string  `json:"timezone"`
	Items    []Item  `json:"items"`
	Summary  Summary `json:"summary"`
}

func Build(date, timezone string, now, start, end time.Time, tasks []domain.Task) Response {
	items := make([]Item, 0, len(tasks))
	for _, task := range tasks {
		bucket, ok := classify(task, now, start, end)
		if !ok {
			continue
		}
		status := "open"
		if bucket == BucketCompletedToday {
			status = "completed"
		}
		items = append(items, Item{
			Kind:        "task",
			ID:          task.ID,
			Title:       task.Title,
			Notes:       task.Notes,
			Priority:    task.Priority,
			DueAt:       task.DueAt,
			CompletedAt: task.CompletedAt,
			Status:      status,
			Bucket:      bucket,
			Urgency:     urgency(bucket),
			createdAt:   task.CreatedAt,
		})
	}
	slices.SortStableFunc(items, compareItems)

	response := Response{
		Date:     date,
		Timezone: timezone,
		Items:    items,
	}
	for _, item := range items {
		if item.Status == "completed" {
			response.Summary.Completed++
		} else {
			response.Summary.Open++
		}
	}
	return response
}

func classify(task domain.Task, now, start, end time.Time) (string, bool) {
	if task.CompletedAt != nil {
		if !task.CompletedAt.Before(start) && task.CompletedAt.Before(end) {
			return BucketCompletedToday, true
		}
		return "", false
	}
	if task.DueAt == nil {
		return BucketAnytime, true
	}
	if task.DueAt.Before(now) {
		return BucketOverdue, true
	}
	if task.DueAt.Before(end) {
		return BucketDueToday, true
	}
	return "", false
}

func urgency(bucket string) string {
	switch bucket {
	case BucketOverdue:
		return "overdue"
	case BucketDueToday:
		return "today"
	case BucketAnytime:
		return "anytime"
	default:
		return "completed"
	}
}

func compareItems(left, right Item) int {
	if order := cmp.Compare(bucketRank(left.Bucket), bucketRank(right.Bucket)); order != 0 {
		return order
	}
	switch left.Bucket {
	case BucketOverdue, BucketDueToday:
		if order := compareOptionalTimes(left.DueAt, right.DueAt); order != 0 {
			return order
		}
	case BucketCompletedToday:
		if order := compareOptionalTimes(left.CompletedAt, right.CompletedAt); order != 0 {
			return order
		}
	}
	if order := cmp.Compare(priorityRank(left.Priority), priorityRank(right.Priority)); order != 0 {
		return order
	}
	if order := left.createdAt.Compare(right.createdAt); order != 0 {
		return order
	}
	return cmp.Compare(left.ID, right.ID)
}

func compareOptionalTimes(left, right *time.Time) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return 1
	case right == nil:
		return -1
	default:
		return left.Compare(*right)
	}
}

func bucketRank(bucket string) int {
	switch bucket {
	case BucketOverdue:
		return 0
	case BucketDueToday:
		return 1
	case BucketAnytime:
		return 2
	default:
		return 3
	}
}

func priorityRank(priority string) int {
	switch priority {
	case domain.PriorityHigh:
		return 0
	case domain.PriorityNormal:
		return 1
	default:
		return 2
	}
}

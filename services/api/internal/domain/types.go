package domain

import "time"

const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
)

type Profile struct {
	UserID   string  `json:"user_id"`
	Timezone *string `json:"timezone"`
	Locale   string  `json:"locale"`
	Currency string  `json:"currency"`
}

type Task struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Notes       *string    `json:"notes"`
	Priority    string     `json:"priority"`
	DueAt       *time.Time `json:"due_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateTaskParams struct {
	ID       string
	UserID   string
	Title    string
	Notes    *string
	Priority string
	DueAt    *time.Time
}

package domain

import "time"

const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"

	// MaxBillAmount is the largest integer that remains exactly representable
	// by JavaScript's JSON number model as well as PostgreSQL bigint.
	MaxBillAmount int64 = 9_007_199_254_740_991
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

type UpdateTaskParams struct {
	ID       string
	UserID   string
	Title    *string
	NotesSet bool
	Notes    *string
	Priority *string
	DueAtSet bool
	DueAt    *time.Time
}

type Event struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Notes     *string    `json:"notes"`
	Location  *string    `json:"location"`
	AllDay    bool       `json:"all_day"`
	Timezone  string     `json:"timezone"`
	StartsAt  *time.Time `json:"starts_at"`
	EndsAt    *time.Time `json:"ends_at"`
	StartsOn  *string    `json:"starts_on"`
	EndsOn    *string    `json:"ends_on"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CreateEventParams struct {
	ID       string
	UserID   string
	Title    string
	Notes    *string
	Location *string
	AllDay   bool
	Timezone string
	StartsAt *time.Time
	EndsAt   *time.Time
	StartsOn *time.Time
	EndsOn   *time.Time
}

type UpdateEventParams struct {
	ID          string
	UserID      string
	Title       *string
	NotesSet    bool
	Notes       *string
	LocationSet bool
	Location    *string
	ScheduleSet bool
	AllDay      bool
	Timezone    string
	StartsAt    *time.Time
	EndsAt      *time.Time
	StartsOn    *time.Time
	EndsOn      *time.Time
}

type Bill struct {
	ID        string     `json:"id"`
	Title     string     `json:"title"`
	Notes     *string    `json:"notes"`
	Amount    int64      `json:"amount"`
	Currency  string     `json:"currency"`
	DueAt     time.Time  `json:"due_at"`
	PaidAt    *time.Time `json:"paid_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CreateBillParams struct {
	ID       string
	UserID   string
	Title    string
	Notes    *string
	Amount   int64
	Currency string
	DueAt    time.Time
}

type UpdateBillParams struct {
	ID       string
	UserID   string
	Title    *string
	NotesSet bool
	Notes    *string
	Amount   *int64
	Currency *string
	DueAt    *time.Time
}

type RecurrenceSeries struct {
	ID         string    `json:"id"`
	SourceKind string    `json:"source_kind"`
	Title      string    `json:"title"`
	Frequency  string    `json:"frequency"`
	Interval   int       `json:"interval"`
	AnchorOn   string    `json:"anchor_on"`
	EndsOn     *string   `json:"ends_on"`
	Timezone   string    `json:"timezone"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type CreateRecurringTaskParams struct {
	SeriesID  string
	Task      CreateTaskParams
	Frequency string
	Interval  int
	AnchorOn  time.Time
	EndsOn    *time.Time
	Timezone  string
	TimeLocal string
	FromOn    time.Time
	ThroughOn time.Time
}

type CreateRecurringEventParams struct {
	SeriesID        string
	Event           CreateEventParams
	Frequency       string
	Interval        int
	AnchorOn        time.Time
	EndsOn          *time.Time
	FromOn          time.Time
	ThroughOn       time.Time
	TimeLocal       *string
	DurationSeconds *int64
	AllDaySpan      int
}

type CreateRecurringBillParams struct {
	SeriesID  string
	Bill      CreateBillParams
	Frequency string
	Interval  int
	AnchorOn  time.Time
	EndsOn    *time.Time
	Timezone  string
	TimeLocal string
	FromOn    time.Time
	ThroughOn time.Time
}

type UpdateRecurrenceSeriesParams struct {
	ID        string
	UserID    string
	Frequency string
	Interval  int
	EndsOn    *time.Time
	FromOn    time.Time
	ThroughOn time.Time
}

type Document struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Category        string    `json:"category"`
	Notes           *string   `json:"notes"`
	ExpiresOn       string    `json:"expires_on"`
	Status          string    `json:"status"`
	DaysUntilExpiry int       `json:"days_until_expiry"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type CreateDocumentParams struct {
	ID        string
	UserID    string
	Name      string
	Category  string
	Notes     *string
	ExpiresOn time.Time
}

type UpdateDocumentParams struct {
	ID        string
	UserID    string
	Name      *string
	Category  *string
	NotesSet  bool
	Notes     *string
	ExpiresOn *time.Time
}

type ReminderSchedule struct {
	Kind          string  `json:"kind"`
	MinutesBefore *int    `json:"minutes_before,omitempty"`
	DaysBefore    *int    `json:"days_before,omitempty"`
	TimeLocal     *string `json:"time_local,omitempty"`
}

type Reminder struct {
	ID         string           `json:"id"`
	SourceKind string           `json:"source_kind"`
	SourceID   string           `json:"source_id"`
	Schedule   ReminderSchedule `json:"schedule"`
	Status     string           `json:"status"`
	NextFireAt *time.Time       `json:"next_fire_at"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type CreateReminderParams struct {
	ID            string
	UserID        string
	SourceKind    string
	SourceID      string
	ScheduleKind  string
	MinutesBefore *int
	DaysBefore    *int
	TimeLocal     *string
}

type UpdateReminderParams struct {
	ID            string
	UserID        string
	ScheduleKind  string
	MinutesBefore *int
	DaysBefore    *int
	TimeLocal     *string
}

type Notification struct {
	ID         string     `json:"id"`
	SourceKind string     `json:"source_kind"`
	SourceID   string     `json:"source_id"`
	Title      string     `json:"title"`
	Body       string     `json:"body"`
	CreatedAt  time.Time  `json:"created_at"`
	ReadAt     *time.Time `json:"read_at"`
}

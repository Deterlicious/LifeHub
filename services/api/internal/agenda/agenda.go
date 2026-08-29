package agenda

import (
	"cmp"
	"encoding/json"
	"slices"
	"time"

	"lifehub/services/api/internal/domain"
)

type Item struct {
	Kind            string
	ID              string
	DisplayOn       string
	Title           string
	Notes           *string
	Priority        *string
	DueAt           *time.Time
	CompletedAt     *time.Time
	Location        *string
	AllDay          *bool
	Timezone        *string
	StartsAt        *time.Time
	EndsAt          *time.Time
	StartsOn        *string
	EndsOn          *string
	Amount          *int64
	Currency        *string
	PaidAt          *time.Time
	Category        *string
	ExpiresOn       *string
	DaysUntilExpiry *int
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// MarshalJSON keeps Agenda a true discriminated union: consumers never see
// null fields belonging to another domain, while optional fields native to a
// domain remain explicit in its full DTO.
func (item Item) MarshalJSON() ([]byte, error) {
	base := struct {
		Kind      string    `json:"kind"`
		ID        string    `json:"id"`
		DisplayOn string    `json:"display_on"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
	}{item.Kind, item.ID, item.DisplayOn, item.CreatedAt, item.UpdatedAt}

	switch item.Kind {
	case "task":
		return json.Marshal(struct {
			Kind        string     `json:"kind"`
			ID          string     `json:"id"`
			DisplayOn   string     `json:"display_on"`
			Title       string     `json:"title"`
			Notes       *string    `json:"notes"`
			Priority    string     `json:"priority"`
			DueAt       *time.Time `json:"due_at"`
			CompletedAt *time.Time `json:"completed_at"`
			Status      string     `json:"status"`
			CreatedAt   time.Time  `json:"created_at"`
			UpdatedAt   time.Time  `json:"updated_at"`
		}{base.Kind, base.ID, base.DisplayOn, item.Title, item.Notes, valueOrZero(item.Priority), item.DueAt, item.CompletedAt, item.Status, base.CreatedAt, base.UpdatedAt})
	case "event":
		return json.Marshal(struct {
			Kind      string     `json:"kind"`
			ID        string     `json:"id"`
			DisplayOn string     `json:"display_on"`
			Title     string     `json:"title"`
			Notes     *string    `json:"notes"`
			Location  *string    `json:"location"`
			AllDay    bool       `json:"all_day"`
			Timezone  string     `json:"timezone"`
			StartsAt  *time.Time `json:"starts_at"`
			EndsAt    *time.Time `json:"ends_at"`
			StartsOn  *string    `json:"starts_on"`
			EndsOn    *string    `json:"ends_on"`
			Status    string     `json:"status"`
			CreatedAt time.Time  `json:"created_at"`
			UpdatedAt time.Time  `json:"updated_at"`
		}{base.Kind, base.ID, base.DisplayOn, item.Title, item.Notes, item.Location, valueOrZero(item.AllDay), valueOrZero(item.Timezone), item.StartsAt, item.EndsAt, item.StartsOn, item.EndsOn, item.Status, base.CreatedAt, base.UpdatedAt})
	case "bill":
		return json.Marshal(struct {
			Kind      string     `json:"kind"`
			ID        string     `json:"id"`
			DisplayOn string     `json:"display_on"`
			Title     string     `json:"title"`
			Notes     *string    `json:"notes"`
			Amount    int64      `json:"amount"`
			Currency  string     `json:"currency"`
			DueAt     *time.Time `json:"due_at"`
			PaidAt    *time.Time `json:"paid_at"`
			Status    string     `json:"status"`
			CreatedAt time.Time  `json:"created_at"`
			UpdatedAt time.Time  `json:"updated_at"`
		}{base.Kind, base.ID, base.DisplayOn, item.Title, item.Notes, valueOrZero(item.Amount), valueOrZero(item.Currency), item.DueAt, item.PaidAt, item.Status, base.CreatedAt, base.UpdatedAt})
	default:
		return json.Marshal(struct {
			Kind            string    `json:"kind"`
			ID              string    `json:"id"`
			DisplayOn       string    `json:"display_on"`
			Title           string    `json:"title"`
			Notes           *string   `json:"notes"`
			Category        string    `json:"category"`
			ExpiresOn       string    `json:"expires_on"`
			DaysUntilExpiry int       `json:"days_until_expiry"`
			Status          string    `json:"status"`
			CreatedAt       time.Time `json:"created_at"`
			UpdatedAt       time.Time `json:"updated_at"`
		}{base.Kind, base.ID, base.DisplayOn, item.Title, item.Notes, valueOrZero(item.Category), valueOrZero(item.ExpiresOn), valueOrZero(item.DaysUntilExpiry), item.Status, base.CreatedAt, base.UpdatedAt})
	}
}

func valueOrZero[T any](value *T) (zero T) {
	if value != nil {
		return *value
	}
	return zero
}

type Summary struct {
	Total     int `json:"total"`
	Tasks     int `json:"tasks"`
	Events    int `json:"events"`
	Bills     int `json:"bills"`
	Documents int `json:"documents"`
}

type Response struct {
	From     string  `json:"from"`
	To       string  `json:"to"`
	Timezone string  `json:"timezone"`
	Items    []Item  `json:"items"`
	Summary  Summary `json:"summary"`
}

func Build(from, to, today string, now time.Time, location *time.Location, timezone string, tasks []domain.Task, events []domain.Event, bills []domain.Bill, documents []domain.Document) Response {
	items := make([]Item, 0, len(tasks)+len(events)+len(bills)+len(documents))
	response := Response{From: from, To: to, Timezone: timezone, Items: items}

	for _, task := range tasks {
		if task.DueAt == nil || task.CompletedAt != nil {
			continue
		}
		priority := task.Priority
		dueAt := *task.DueAt
		response.Items = append(response.Items, Item{
			Kind: "task", ID: task.ID, DisplayOn: localDate(dueAt, location), Title: task.Title, Notes: task.Notes,
			Priority: &priority, DueAt: &dueAt, CompletedAt: task.CompletedAt, Status: "open",
			CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		})
		response.Summary.Tasks++
	}
	for _, event := range events {
		item, ok := eventItem(event, from, today, now, location)
		if !ok {
			continue
		}
		response.Items = append(response.Items, item)
		response.Summary.Events++
	}
	for _, bill := range bills {
		if bill.PaidAt != nil {
			continue
		}
		amount := bill.Amount
		currency := bill.Currency
		dueAt := bill.DueAt
		response.Items = append(response.Items, Item{
			Kind: "bill", ID: bill.ID, DisplayOn: localDate(dueAt, location), Title: bill.Title, Notes: bill.Notes,
			Amount: &amount, Currency: &currency, DueAt: &dueAt, PaidAt: nil, Status: "unpaid",
			CreatedAt: bill.CreatedAt, UpdatedAt: bill.UpdatedAt,
		})
		response.Summary.Bills++
	}
	for _, document := range documents {
		days, ok := dateDifference(today, document.ExpiresOn)
		if !ok {
			continue
		}
		status := "valid"
		if days < 0 {
			status = "expired"
		} else if days <= 30 {
			status = "expiring"
		}
		category := document.Category
		expiresOn := document.ExpiresOn
		response.Items = append(response.Items, Item{
			Kind: "document", ID: document.ID, DisplayOn: document.ExpiresOn, Title: document.Name, Notes: document.Notes,
			Category: &category, ExpiresOn: &expiresOn, DaysUntilExpiry: &days, Status: status,
			CreatedAt: document.CreatedAt, UpdatedAt: document.UpdatedAt,
		})
		response.Summary.Documents++
	}

	slices.SortStableFunc(response.Items, compareItems)
	response.Summary.Total = len(response.Items)
	return response
}

func eventItem(event domain.Event, from, today string, now time.Time, location *time.Location) (Item, bool) {
	allDay := event.AllDay
	timezone := event.Timezone
	item := Item{
		Kind: "event", ID: event.ID, Title: event.Title, Notes: event.Notes, Location: event.Location,
		AllDay: &allDay, Timezone: &timezone, StartsAt: event.StartsAt, EndsAt: event.EndsAt,
		StartsOn: event.StartsOn, EndsOn: event.EndsOn, Status: "scheduled",
		CreatedAt: event.CreatedAt, UpdatedAt: event.UpdatedAt,
	}
	if event.AllDay {
		if event.StartsOn == nil {
			return Item{}, false
		}
		item.DisplayOn = maxDate(*event.StartsOn, from)
		last := *event.StartsOn
		if event.EndsOn != nil {
			last = *event.EndsOn
		}
		if last < today {
			item.Status = "past"
		}
		return item, true
	}
	if event.StartsAt == nil {
		return Item{}, false
	}
	item.DisplayOn = maxDate(localDate(*event.StartsAt, location), from)
	if event.EndsAt != nil {
		if !event.StartsAt.After(now) && now.Before(*event.EndsAt) {
			item.Status = "in_progress"
		} else if !now.Before(*event.EndsAt) {
			item.Status = "past"
		}
	} else if event.StartsAt.Before(now) {
		item.Status = "past"
	}
	return item, true
}

func SortEvents(events []domain.Event, from string, location *time.Location) {
	slices.SortStableFunc(events, func(left, right domain.Event) int {
		leftDate := eventDisplayDate(left, from, location)
		rightDate := eventDisplayDate(right, from, location)
		if order := cmp.Compare(leftDate, rightDate); order != 0 {
			return order
		}
		if order := cmp.Compare(eventPresentationRank(left), eventPresentationRank(right)); order != 0 {
			return order
		}
		if left.AllDay {
			if order := compareOptionalStrings(left.StartsOn, right.StartsOn); order != 0 {
				return order
			}
			if order := compareOptionalStrings(left.EndsOn, right.EndsOn); order != 0 {
				return order
			}
		} else {
			if order := compareOptionalTimes(left.StartsAt, right.StartsAt); order != 0 {
				return order
			}
			if order := compareOptionalTimes(left.EndsAt, right.EndsAt); order != 0 {
				return order
			}
		}
		if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
			return order
		}
		return cmp.Compare(left.ID, right.ID)
	})
}

func compareItems(left, right Item) int {
	if order := cmp.Compare(left.DisplayOn, right.DisplayOn); order != 0 {
		return order
	}
	if order := cmp.Compare(presentationRank(left), presentationRank(right)); order != 0 {
		return order
	}
	switch presentationRank(left) {
	case 0:
		if order := compareOptionalStrings(left.StartsOn, right.StartsOn); order != 0 {
			return order
		}
		if order := compareOptionalStrings(left.EndsOn, right.EndsOn); order != 0 {
			return order
		}
	case 1:
		if order := compareOptionalStrings(left.ExpiresOn, right.ExpiresOn); order != 0 {
			return order
		}
	default:
		if order := compareOptionalTimes(effectiveTime(left), effectiveTime(right)); order != 0 {
			return order
		}
	}
	if left.Kind == "task" && right.Kind == "task" {
		if order := cmp.Compare(priorityRank(valueOrZero(left.Priority)), priorityRank(valueOrZero(right.Priority))); order != 0 {
			return order
		}
	}
	if order := cmp.Compare(kindRank(left.Kind), kindRank(right.Kind)); order != 0 {
		return order
	}
	if order := left.CreatedAt.Compare(right.CreatedAt); order != 0 {
		return order
	}
	return cmp.Compare(left.ID, right.ID)
}

func presentationRank(item Item) int {
	if item.Kind == "event" && valueOrZero(item.AllDay) {
		return 0
	}
	if item.Kind == "document" {
		return 1
	}
	return 2
}

func eventPresentationRank(event domain.Event) int {
	if event.AllDay {
		return 0
	}
	return 2
}

func effectiveTime(item Item) *time.Time {
	if item.Kind == "event" {
		return item.StartsAt
	}
	return item.DueAt
}

func eventDisplayDate(event domain.Event, from string, location *time.Location) string {
	if event.AllDay && event.StartsOn != nil {
		return maxDate(*event.StartsOn, from)
	}
	if event.StartsAt != nil {
		return maxDate(localDate(*event.StartsAt, location), from)
	}
	return from
}

func localDate(value time.Time, location *time.Location) string {
	return value.In(location).Format(time.DateOnly)
}

func maxDate(left, right string) string {
	if left > right {
		return left
	}
	return right
}

func dateDifference(startDate, endDate string) (int, bool) {
	start, err := time.Parse(time.DateOnly, startDate)
	if err != nil {
		return 0, false
	}
	end, err := time.Parse(time.DateOnly, endDate)
	if err != nil {
		return 0, false
	}
	return int(end.Sub(start) / (24 * time.Hour)), true
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

func compareOptionalStrings(left, right *string) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return 1
	case right == nil:
		return -1
	default:
		return cmp.Compare(*left, *right)
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

func kindRank(kind string) int {
	switch kind {
	case "event":
		return 0
	case "bill":
		return 1
	case "task":
		return 2
	default:
		return 3
	}
}

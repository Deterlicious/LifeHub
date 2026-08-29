package today

import (
	"cmp"
	"slices"
	"time"

	"lifehub/services/api/internal/domain"
)

const (
	BucketOverdue        = "overdue"
	BucketExpired        = "expired"
	BucketHappeningNow   = "happening_now"
	BucketAllDay         = "all_day"
	BucketDueToday       = "due_today"
	BucketEventToday     = "event_today"
	BucketExpiresToday   = "expires_today"
	BucketExpiringSoon   = "expiring_soon"
	BucketUpcoming       = "upcoming"
	BucketAnytime        = "anytime"
	BucketCompletedToday = "completed_today"
	BucketPaidToday      = "paid_today"
)

type Item struct {
	Kind            string     `json:"kind"`
	ID              string     `json:"id"`
	Title           string     `json:"title,omitempty"`
	Notes           *string    `json:"notes"`
	Priority        *string    `json:"priority,omitempty"`
	DueAt           *time.Time `json:"due_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	Location        *string    `json:"location,omitempty"`
	AllDay          *bool      `json:"all_day,omitempty"`
	Timezone        *string    `json:"timezone,omitempty"`
	StartsAt        *time.Time `json:"starts_at,omitempty"`
	EndsAt          *time.Time `json:"ends_at,omitempty"`
	StartsOn        *string    `json:"starts_on,omitempty"`
	EndsOn          *string    `json:"ends_on,omitempty"`
	Amount          *int64     `json:"amount,omitempty"`
	Currency        *string    `json:"currency,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	Category        *string    `json:"category,omitempty"`
	ExpiresOn       *string    `json:"expires_on,omitempty"`
	DaysUntilExpiry *int       `json:"days_until_expiry,omitempty"`
	Status          string     `json:"status"`
	Bucket          string     `json:"bucket"`
	Urgency         string     `json:"urgency"`
	createdAt       time.Time
	displayOn       string
}

type Summary struct {
	Open      int `json:"open"`
	Completed int `json:"completed"`
	Upcoming  int `json:"upcoming"`
}

type Response struct {
	Date                string  `json:"date"`
	Timezone            string  `json:"timezone"`
	Items               []Item  `json:"items"`
	Upcoming            []Item  `json:"upcoming"`
	UpcomingHorizonDays int     `json:"upcoming_horizon_days"`
	Summary             Summary `json:"summary"`
}

func Build(date, horizonDate, timezone string, now, start, end, horizonEnd time.Time, location *time.Location, tasks []domain.Task, events []domain.Event, bills []domain.Bill, documents []domain.Document) Response {
	items := make([]Item, 0, len(tasks)+len(events)+len(bills)+len(documents))
	upcoming := make([]Item, 0)
	for _, task := range tasks {
		bucket, ok := classify(task, now, start, end, horizonEnd)
		if !ok {
			continue
		}
		status := "open"
		if bucket == BucketCompletedToday {
			status = "completed"
		}
		priority := task.Priority
		item := Item{
			Kind:        "task",
			ID:          task.ID,
			Title:       task.Title,
			Notes:       task.Notes,
			Priority:    &priority,
			DueAt:       task.DueAt,
			CompletedAt: task.CompletedAt,
			Status:      status,
			Bucket:      bucket,
			Urgency:     urgency(bucket),
			createdAt:   task.CreatedAt,
		}
		if bucket == BucketUpcoming {
			item.displayOn = task.DueAt.In(location).Format(time.DateOnly)
			upcoming = append(upcoming, item)
		} else {
			items = append(items, item)
		}
	}
	for _, bill := range bills {
		bucket, status, ok := classifyBill(bill, now, start, end, horizonEnd)
		if !ok {
			continue
		}
		amount := bill.Amount
		currency := bill.Currency
		dueAt := bill.DueAt
		item := Item{
			Kind:      "bill",
			ID:        bill.ID,
			Title:     bill.Title,
			Notes:     bill.Notes,
			Amount:    &amount,
			Currency:  &currency,
			DueAt:     &dueAt,
			PaidAt:    bill.PaidAt,
			Status:    status,
			Bucket:    bucket,
			Urgency:   urgency(bucket),
			createdAt: bill.CreatedAt,
		}
		if bucket == BucketUpcoming {
			item.displayOn = bill.DueAt.In(location).Format(time.DateOnly)
			upcoming = append(upcoming, item)
		} else {
			items = append(items, item)
		}
	}
	for _, event := range events {
		bucket, status, ok := classifyEvent(event, date, horizonDate, now, start, end, horizonEnd)
		if !ok {
			continue
		}
		allDay := event.AllDay
		eventTimezone := event.Timezone
		item := Item{
			Kind:      "event",
			ID:        event.ID,
			Title:     event.Title,
			Notes:     event.Notes,
			Location:  event.Location,
			AllDay:    &allDay,
			Timezone:  &eventTimezone,
			StartsAt:  event.StartsAt,
			EndsAt:    event.EndsAt,
			StartsOn:  event.StartsOn,
			EndsOn:    event.EndsOn,
			Status:    status,
			Bucket:    bucket,
			Urgency:   urgency(bucket),
			createdAt: event.CreatedAt,
		}
		if bucket == BucketUpcoming {
			if event.AllDay && event.StartsOn != nil {
				item.displayOn = *event.StartsOn
			} else if event.StartsAt != nil {
				item.displayOn = event.StartsAt.In(location).Format(time.DateOnly)
			}
			upcoming = append(upcoming, item)
		} else {
			items = append(items, item)
		}
	}
	for _, document := range documents {
		item, isUpcoming, ok := buildDocumentItem(document, date)
		if !ok {
			continue
		}
		if isUpcoming {
			upcoming = append(upcoming, item)
		} else {
			items = append(items, item)
		}
	}
	slices.SortStableFunc(items, compareItems)
	slices.SortStableFunc(upcoming, compareUpcomingItems)

	response := Response{
		Date:                date,
		Timezone:            timezone,
		Items:               items,
		Upcoming:            upcoming,
		UpcomingHorizonDays: 30,
	}
	for _, item := range items {
		if item.Status == "completed" || item.Status == "paid" {
			response.Summary.Completed++
		} else {
			response.Summary.Open++
		}
	}
	response.Summary.Upcoming = len(upcoming)
	return response
}

func buildDocumentItem(document domain.Document, date string) (Item, bool, bool) {
	daysUntilExpiry, ok := dateDifference(date, document.ExpiresOn)
	if !ok || daysUntilExpiry > 30 {
		return Item{}, false, false
	}
	bucket := BucketExpiresToday
	status := "expiring"
	urgencyValue := "today"
	isUpcoming := false
	if daysUntilExpiry < 0 {
		bucket = BucketExpired
		status = "expired"
		urgencyValue = "overdue"
	} else if daysUntilExpiry > 0 {
		bucket = BucketExpiringSoon
		urgencyValue = "upcoming"
		isUpcoming = true
	}
	category := document.Category
	expiresOn := document.ExpiresOn
	days := daysUntilExpiry
	return Item{
		Kind:            "document",
		ID:              document.ID,
		Title:           document.Name,
		Notes:           document.Notes,
		Category:        &category,
		ExpiresOn:       &expiresOn,
		DaysUntilExpiry: &days,
		Status:          status,
		Bucket:          bucket,
		Urgency:         urgencyValue,
		createdAt:       document.CreatedAt,
		displayOn:       document.ExpiresOn,
	}, isUpcoming, true
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
	return int(end.Unix()/86400 - start.Unix()/86400), true
}

func classifyBill(bill domain.Bill, now, start, end, horizonEnd time.Time) (bucket, status string, ok bool) {
	if bill.PaidAt != nil {
		if !bill.PaidAt.Before(start) && bill.PaidAt.Before(end) {
			return BucketPaidToday, "paid", true
		}
		return "", "", false
	}
	if bill.DueAt.Before(now) {
		return BucketOverdue, "unpaid", true
	}
	if bill.DueAt.Before(end) {
		return BucketDueToday, "unpaid", true
	}
	if bill.DueAt.Before(horizonEnd) {
		return BucketUpcoming, "unpaid", true
	}
	return "", "", false
}

func classifyEvent(event domain.Event, date, horizonDate string, now, start, end, horizonEnd time.Time) (bucket, status string, ok bool) {
	if event.AllDay {
		if event.StartsOn == nil {
			return "", "", false
		}
		if *event.StartsOn > date {
			if *event.StartsOn <= horizonDate {
				return BucketUpcoming, "scheduled", true
			}
			return "", "", false
		}
		lastDate := event.StartsOn
		if event.EndsOn != nil {
			lastDate = event.EndsOn
		}
		if *lastDate < date {
			return "", "", false
		}
		return BucketAllDay, "scheduled", true
	}
	if event.StartsAt == nil {
		return "", "", false
	}
	if !event.StartsAt.Before(end) {
		if event.StartsAt.Before(horizonEnd) {
			return BucketUpcoming, "scheduled", true
		}
		return "", "", false
	}
	if event.EndsAt == nil {
		if event.StartsAt.Before(start) || !event.StartsAt.Before(end) {
			return "", "", false
		}
		status = "scheduled"
		if event.StartsAt.Before(now) {
			status = "past"
		}
		return BucketEventToday, status, true
	}
	if !event.StartsAt.Before(end) || !event.EndsAt.After(start) {
		return "", "", false
	}
	if !event.StartsAt.After(now) && now.Before(*event.EndsAt) {
		return BucketHappeningNow, "in_progress", true
	}
	status = "scheduled"
	if !now.Before(*event.EndsAt) {
		status = "past"
	}
	return BucketEventToday, status, true
}

func classify(task domain.Task, now, start, end, horizonEnd time.Time) (string, bool) {
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
	if task.DueAt.Before(horizonEnd) {
		return BucketUpcoming, true
	}
	return "", false
}

func urgency(bucket string) string {
	switch bucket {
	case BucketOverdue:
		return "overdue"
	case BucketExpired:
		return "overdue"
	case BucketHappeningNow:
		return "now"
	case BucketAllDay, BucketEventToday:
		return "today"
	case BucketDueToday:
		return "today"
	case BucketExpiresToday:
		return "today"
	case BucketExpiringSoon:
		return "upcoming"
	case BucketUpcoming:
		return "upcoming"
	case BucketAnytime:
		return "anytime"
	case BucketPaidToday:
		return "completed"
	default:
		return "completed"
	}
}

func compareItems(left, right Item) int {
	leftBucketRank := bucketRank(left.Bucket)
	rightBucketRank := bucketRank(right.Bucket)
	if order := cmp.Compare(leftBucketRank, rightBucketRank); order != 0 {
		return order
	}
	if leftBucketRank == bucketRank(BucketDueToday) {
		if order := compareOptionalTimes(effectiveTime(left), effectiveTime(right)); order != 0 {
			return order
		}
	}
	if leftBucketRank == bucketRank(BucketCompletedToday) {
		if order := compareOptionalTimes(closedTime(left), closedTime(right)); order != 0 {
			return order
		}
	}
	switch left.Bucket {
	case BucketOverdue:
		if order := compareOptionalTimes(left.DueAt, right.DueAt); order != 0 {
			return order
		}
	case BucketHappeningNow:
		if order := compareOptionalTimes(left.StartsAt, right.StartsAt); order != 0 {
			return order
		}
	case BucketAllDay:
		if order := compareOptionalStrings(left.StartsOn, right.StartsOn); order != 0 {
			return order
		}
		if order := compareOptionalStrings(left.EndsOn, right.EndsOn); order != 0 {
			return order
		}
	case BucketExpired, BucketExpiresToday, BucketExpiringSoon:
		if order := compareOptionalStrings(left.ExpiresOn, right.ExpiresOn); order != 0 {
			return order
		}
	}
	if left.Priority != nil && right.Priority != nil {
		if order := cmp.Compare(priorityRank(*left.Priority), priorityRank(*right.Priority)); order != 0 {
			return order
		}
	}
	if order := cmp.Compare(kindRank(left.Kind), kindRank(right.Kind)); order != 0 {
		return order
	}
	if order := left.createdAt.Compare(right.createdAt); order != 0 {
		return order
	}
	return cmp.Compare(left.ID, right.ID)
}

func compareUpcomingItems(left, right Item) int {
	if order := cmp.Compare(left.displayOn, right.displayOn); order != 0 {
		return order
	}
	if order := cmp.Compare(upcomingPresentationRank(left), upcomingPresentationRank(right)); order != 0 {
		return order
	}
	switch upcomingPresentationRank(left) {
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
	if left.Kind == "task" && right.Kind == "task" && left.Priority != nil && right.Priority != nil {
		if order := cmp.Compare(priorityRank(*left.Priority), priorityRank(*right.Priority)); order != 0 {
			return order
		}
	}
	if order := cmp.Compare(kindRank(left.Kind), kindRank(right.Kind)); order != 0 {
		return order
	}
	if order := left.createdAt.Compare(right.createdAt); order != 0 {
		return order
	}
	return cmp.Compare(left.ID, right.ID)
}

func upcomingPresentationRank(item Item) int {
	if item.Kind == "event" && item.AllDay != nil && *item.AllDay {
		return 0
	}
	if item.Kind == "document" {
		return 1
	}
	return 2
}

func effectiveTime(item Item) *time.Time {
	if item.Kind == "event" {
		return item.StartsAt
	}
	return item.DueAt
}

func closedTime(item Item) *time.Time {
	if item.Kind == "bill" {
		return item.PaidAt
	}
	return item.CompletedAt
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

func bucketRank(bucket string) int {
	switch bucket {
	case BucketOverdue:
		return 0
	case BucketExpired:
		return 1
	case BucketHappeningNow:
		return 2
	case BucketAllDay:
		return 3
	case BucketExpiresToday:
		return 4
	case BucketDueToday, BucketEventToday:
		return 5
	case BucketAnytime:
		return 6
	case BucketCompletedToday, BucketPaidToday:
		return 7
	case BucketExpiringSoon:
		return 8
	case BucketUpcoming:
		return 8
	default:
		return 6
	}
}

func kindRank(kind string) int {
	if kind == "event" {
		return 0
	}
	if kind == "bill" {
		return 1
	}
	return 2
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

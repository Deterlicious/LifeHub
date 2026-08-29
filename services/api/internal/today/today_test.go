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

	response := buildForTest("2026-08-19", now, start, end, tasks, nil, nil, nil)
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
	response := buildForTest("2026-08-19", start, start, start.Add(24*time.Hour), tasks, nil, nil, nil)
	if len(response.Items) != len(tasks) || response.Summary.Open != len(tasks) {
		t.Fatalf("items = %d, open = %d, want %d", len(response.Items), response.Summary.Open, len(tasks))
	}
}

func TestBuildMergesTasksEventsAndBillsInTodayOrder(t *testing.T) {
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	now := start.Add(8 * time.Hour)
	overdueAt := now.Add(-time.Hour)
	overdueBillAt := now.Add(-2 * time.Hour)
	dueBillAt := now.Add(time.Hour)
	taskAt := now.Add(3 * time.Hour)
	completedAt := now.Add(-30 * time.Minute)
	paidAt := now.Add(-40 * time.Minute)
	ongoingStart := now.Add(-45 * time.Minute)
	ongoingEnd := now.Add(45 * time.Minute)
	pastStart := now.Add(-3 * time.Hour)
	pastEnd := now.Add(-2 * time.Hour)
	futureStart := now.Add(2 * time.Hour)
	allDayStart := "2026-08-19"
	created := start.Add(-24 * time.Hour)

	tasks := []domain.Task{
		{ID: "completed", Title: "Completed", Priority: domain.PriorityNormal, CompletedAt: &completedAt, CreatedAt: created},
		{ID: "anytime", Title: "Anytime", Priority: domain.PriorityHigh, CreatedAt: created},
		{ID: "task-today", Title: "Task today", Priority: domain.PriorityNormal, DueAt: &taskAt, CreatedAt: created},
		{ID: "overdue", Title: "Overdue", Priority: domain.PriorityLow, DueAt: &overdueAt, CreatedAt: created},
	}
	events := []domain.Event{
		{ID: "future-event", Title: "Future event", Timezone: "Asia/Jakarta", StartsAt: &futureStart, CreatedAt: created},
		{ID: "all-day", Title: "All day", AllDay: true, Timezone: "Asia/Jakarta", StartsOn: &allDayStart, CreatedAt: created},
		{ID: "past-event", Title: "Past event", Timezone: "Asia/Jakarta", StartsAt: &pastStart, EndsAt: &pastEnd, CreatedAt: created},
		{ID: "happening", Title: "Happening", Timezone: "Asia/Jakarta", StartsAt: &ongoingStart, EndsAt: &ongoingEnd, CreatedAt: created},
	}
	bills := []domain.Bill{
		{ID: "overdue-bill", Title: "Overdue bill", Amount: 350000, Currency: "IDR", DueAt: overdueBillAt, CreatedAt: created},
		{ID: "due-bill", Title: "Due bill", Amount: 200000, Currency: "IDR", DueAt: dueBillAt, CreatedAt: created},
		{ID: "paid-bill", Title: "Paid bill", Amount: 100000, Currency: "IDR", DueAt: dueBillAt, PaidAt: &paidAt, CreatedAt: created},
	}
	documents := []domain.Document{
		{ID: "expired-document", Name: "Expired document", Category: "license", ExpiresOn: "2026-08-18", CreatedAt: created},
		{ID: "expires-today", Name: "Expires today", Category: "identity", ExpiresOn: "2026-08-19", CreatedAt: created},
		{ID: "upcoming-document", Name: "Upcoming document", Category: "insurance", ExpiresOn: "2026-08-20", CreatedAt: created},
	}

	response := buildForTest("2026-08-19", now, start, end, tasks, events, bills, documents)
	got := make([]string, 0, len(response.Items))
	for _, item := range response.Items {
		got = append(got, item.ID)
		if item.Kind != "task" && item.Priority != nil {
			t.Fatalf("%s %q invented task priority %q", item.Kind, item.ID, *item.Priority)
		}
	}
	want := []string{"overdue-bill", "overdue", "expired-document", "happening", "all-day", "expires-today", "past-event", "due-bill", "future-event", "task-today", "anytime", "paid-bill", "completed"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	if response.Items[3].Bucket != BucketHappeningNow || response.Items[3].Status != "in_progress" {
		t.Fatalf("happening event = %#v", response.Items[3])
	}
	if response.Items[4].Bucket != BucketAllDay || response.Items[4].StartsOn == nil {
		t.Fatalf("all-day event = %#v", response.Items[4])
	}
	if response.Items[11].Bucket != BucketPaidToday || response.Items[11].Status != "paid" || response.Items[11].Amount == nil {
		t.Fatalf("paid bill = %#v", response.Items[11])
	}
	if response.Summary.Open != 11 || response.Summary.Completed != 2 || response.Summary.Upcoming != 1 || len(response.Upcoming) != 1 {
		t.Fatalf("summary = %#v", response.Summary)
	}
	if response.Upcoming[0].ID != "upcoming-document" || response.Upcoming[0].Bucket != BucketExpiringSoon {
		t.Fatalf("upcoming = %#v", response.Upcoming)
	}
}

func TestBuildUsesInclusiveAllDayEndAndTimedOverlap(t *testing.T) {
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	now := start.Add(8 * time.Hour)
	dayBefore := "2026-08-18"
	today := "2026-08-19"
	dayAfter := "2026-08-20"
	beforeStart := start.Add(-time.Hour)
	afterStart := start.Add(time.Hour)
	atEnd := end
	afterEnd := end.Add(time.Hour)

	events := []domain.Event{
		{ID: "all-day-inclusive", AllDay: true, StartsOn: &dayBefore, EndsOn: &today},
		{ID: "all-day-single", AllDay: true, StartsOn: &today},
		{ID: "all-day-future", AllDay: true, StartsOn: &dayAfter},
		{ID: "range-overlap", StartsAt: &beforeStart, EndsAt: &afterStart},
		{ID: "range-at-end", StartsAt: &atEnd, EndsAt: &afterEnd},
		{ID: "point-at-end", StartsAt: &atEnd},
	}

	response := buildForTest(today, now, start, end, nil, events, nil, nil)
	got := make(map[string]bool, len(response.Items))
	for _, item := range response.Items {
		got[item.ID] = true
	}
	for _, id := range []string{"all-day-inclusive", "all-day-single", "range-overlap"} {
		if !got[id] {
			t.Fatalf("expected %q in Today: %#v", id, response.Items)
		}
	}
	for _, id := range []string{"all-day-future", "range-at-end", "point-at-end"} {
		if got[id] {
			t.Fatalf("unexpected %q in Today: %#v", id, response.Items)
		}
	}
}

func TestBuildPartitionsDocumentExpiryCalendarBoundaries(t *testing.T) {
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	documents := []domain.Document{
		{ID: "expired", Name: "Expired", Category: "license", ExpiresOn: "2026-08-18"},
		{ID: "today", Name: "Today", Category: "identity", ExpiresOn: "2026-08-19"},
		{ID: "tomorrow", Name: "Tomorrow", Category: "work", ExpiresOn: "2026-08-20"},
		{ID: "day-30", Name: "Day 30", Category: "insurance", ExpiresOn: "2026-09-18"},
		{ID: "day-31", Name: "Day 31", Category: "education", ExpiresOn: "2026-09-19"},
	}
	response := buildForTest("2026-08-19", start, start, start.Add(24*time.Hour), nil, nil, nil, documents)
	if len(response.Items) != 2 || response.Items[0].ID != "expired" || response.Items[1].ID != "today" {
		t.Fatalf("primary documents = %#v", response.Items)
	}
	if len(response.Upcoming) != 2 || response.Upcoming[0].ID != "tomorrow" || response.Upcoming[1].ID != "day-30" {
		t.Fatalf("upcoming documents = %#v", response.Upcoming)
	}
	wantDays := []int{-1, 0, 1, 30}
	gotItems := append(append([]Item{}, response.Items...), response.Upcoming...)
	for index, item := range gotItems {
		if item.DaysUntilExpiry == nil || *item.DaysUntilExpiry != wantDays[index] {
			t.Fatalf("item %q days = %v, want %d", item.ID, item.DaysUntilExpiry, wantDays[index])
		}
	}
	if response.UpcomingHorizonDays != 30 || response.Summary.Open != 2 || response.Summary.Completed != 0 || response.Summary.Upcoming != 2 {
		t.Fatalf("response metadata = %#v", response)
	}
}

func TestBuildUpcomingIncludesEveryUnresolvedDomainThroughDay30Only(t *testing.T) {
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	tomorrowEventAt := end.Add(8 * time.Hour)
	tomorrowBillAt := end.Add(9 * time.Hour)
	tomorrowTaskAt := end.Add(10 * time.Hour)
	day30TaskAt := start.AddDate(0, 0, 30).Add(10 * time.Hour)
	day31TaskAt := start.AddDate(0, 0, 31).Add(10 * time.Hour)
	completedYesterday := start.Add(-time.Hour)
	paidYesterday := start.Add(-time.Hour)
	tomorrow := "2026-08-20"
	day30 := "2026-09-18"
	day31 := "2026-09-19"

	response := buildForTest(
		"2026-08-19", start.Add(8*time.Hour), start, end,
		[]domain.Task{
			{ID: "task", Title: "Task", Priority: domain.PriorityHigh, DueAt: &tomorrowTaskAt},
			{ID: "task-day-30", Title: "Day 30", Priority: domain.PriorityNormal, DueAt: &day30TaskAt},
			{ID: "task-day-31", Title: "Day 31", Priority: domain.PriorityNormal, DueAt: &day31TaskAt},
			{ID: "completed", Title: "Completed", Priority: domain.PriorityNormal, DueAt: &tomorrowTaskAt, CompletedAt: &completedYesterday},
		},
		[]domain.Event{
			{ID: "all-day", Title: "All day", AllDay: true, Timezone: "Asia/Jakarta", StartsOn: &tomorrow},
			{ID: "event", Title: "Event", Timezone: "Asia/Jakarta", StartsAt: &tomorrowEventAt},
			{ID: "all-day-31", Title: "All day 31", AllDay: true, Timezone: "Asia/Jakarta", StartsOn: &day31},
		},
		[]domain.Bill{
			{ID: "bill", Title: "Bill", Amount: 1000, Currency: "IDR", DueAt: tomorrowBillAt},
			{ID: "paid", Title: "Paid", Amount: 1000, Currency: "IDR", DueAt: tomorrowBillAt, PaidAt: &paidYesterday},
		},
		[]domain.Document{
			{ID: "document", Name: "Document", Category: "license", ExpiresOn: tomorrow},
			{ID: "document-day-30", Name: "Document 30", Category: "license", ExpiresOn: day30},
			{ID: "document-day-31", Name: "Document 31", Category: "license", ExpiresOn: day31},
		},
	)

	got := make([]string, 0, len(response.Upcoming))
	for _, item := range response.Upcoming {
		got = append(got, item.ID)
		if item.Urgency != "upcoming" {
			t.Fatalf("item %q urgency = %q", item.ID, item.Urgency)
		}
		if item.Kind != "document" && item.Bucket != BucketUpcoming {
			t.Fatalf("item %q bucket = %q", item.ID, item.Bucket)
		}
	}
	want := []string{"all-day", "document", "event", "bill", "task", "document-day-30", "task-day-30"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("upcoming = %v, want %v", got, want)
	}
	if response.Summary.Open != 0 || response.Summary.Completed != 0 || response.Summary.Upcoming != len(want) {
		t.Fatalf("summary = %#v", response.Summary)
	}
}

func buildForTest(date string, now, start, end time.Time, tasks []domain.Task, events []domain.Event, bills []domain.Bill, documents []domain.Document) Response {
	location, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		panic(err)
	}
	localDate, err := time.Parse(time.DateOnly, date)
	if err != nil {
		panic(err)
	}
	return Build(
		date,
		localDate.AddDate(0, 0, 30).Format(time.DateOnly),
		"Asia/Jakarta",
		now,
		start,
		end,
		start.In(location).AddDate(0, 0, 31).UTC(),
		location,
		tasks,
		events,
		bills,
		documents,
	)
}

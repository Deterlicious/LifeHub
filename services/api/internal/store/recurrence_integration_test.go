package store

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/migrate"
)

func TestRecurrencePostgresCalendarOwnershipAndStop(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := migrate.Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	storage, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)

	userID := mustUUID(t)
	otherUserID := mustUUID(t)
	for _, id := range []string{userID, otherUserID} {
		if _, err := storage.GetProfile(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := storage.UpdateProfileTimezone(ctx, userID, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.UpdateProfileTimezone(ctx, otherUserID, "Asia/Jakarta"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	monthlySeriesID := mustUUID(t)
	anchor := integrationDate(t, "2027-01-31")
	anchorDue := time.Date(2027, time.January, 31, 14, 15, 0, 0, time.UTC)
	if _, err := storage.CreateRecurringTask(ctx, domain.CreateRecurringTaskParams{
		SeriesID: monthlySeriesID,
		Task: domain.CreateTaskParams{
			ID: mustUUID(t), UserID: userID, Title: "Tutup buku", Priority: domain.PriorityHigh, DueAt: &anchorDue,
		},
		Frequency: "monthly", Interval: 1, AnchorOn: anchor, Timezone: "America/New_York",
		TimeLocal: "09:15:00", FromOn: anchor, ThroughOn: integrationDate(t, "2027-05-31"),
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.MaterializeRecurrences(ctx, userID, anchor, integrationDate(t, "2027-05-31")); err != nil {
		t.Fatal(err)
	}
	assertOccurrenceDates(t, ctx, storage, "tasks", monthlySeriesID, []string{
		"2027-01-31", "2027-02-28", "2027-03-31", "2027-04-30", "2027-05-31",
	})
	var editedTaskID, deletedTaskID string
	if err := storage.pool.QueryRow(ctx, `SELECT id::text FROM lifehub.tasks WHERE series_id=$1::uuid AND occurrence_on='2027-02-28'::date`, monthlySeriesID).Scan(&editedTaskID); err != nil {
		t.Fatal(err)
	}
	if err := storage.pool.QueryRow(ctx, `SELECT id::text FROM lifehub.tasks WHERE series_id=$1::uuid AND occurrence_on='2027-03-31'::date`, monthlySeriesID).Scan(&deletedTaskID); err != nil {
		t.Fatal(err)
	}
	editedTitle := "Tutup buku khusus"
	if _, err := storage.UpdateTask(ctx, domain.UpdateTaskParams{ID: editedTaskID, UserID: userID, Title: &editedTitle}); err != nil {
		t.Fatal(err)
	}
	if err := storage.DeleteTask(ctx, userID, deletedTaskID); err != nil {
		t.Fatal(err)
	}
	var monthlyCount int
	if err := storage.pool.QueryRow(ctx, `SELECT count(*) FROM lifehub.tasks WHERE series_id=$1::uuid`, monthlySeriesID).Scan(&monthlyCount); err != nil {
		t.Fatal(err)
	}
	if monthlyCount != 5 {
		t.Fatalf("monthly occurrence count = %d, want 5 after idempotent materialization", monthlyCount)
	}
	if err := storage.MaterializeRecurrences(ctx, userID, anchor, integrationDate(t, "2027-05-31")); err != nil {
		t.Fatal(err)
	}
	var preservedTitle string
	var isException, excluded bool
	if err := storage.pool.QueryRow(ctx, `SELECT title, is_exception FROM lifehub.tasks WHERE id=$1::uuid`, editedTaskID).Scan(&preservedTitle, &isException); err != nil {
		t.Fatal(err)
	}
	if preservedTitle != editedTitle || !isException {
		t.Fatalf("edited occurrence title=%q exception=%v", preservedTitle, isException)
	}
	if err := storage.pool.QueryRow(ctx, `SELECT excluded_at IS NOT NULL FROM lifehub.tasks WHERE id=$1::uuid`, deletedTaskID).Scan(&excluded); err != nil {
		t.Fatal(err)
	}
	if !excluded {
		t.Fatal("deleted recurring occurrence was not retained as an exclusion")
	}
	if _, err := storage.GetTask(ctx, userID, deletedTaskID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("excluded recurring task get error=%v, want ErrNotFound", err)
	}
	updatedSeries, err := storage.UpdateRecurrenceSeries(ctx, domain.UpdateRecurrenceSeriesParams{
		ID: monthlySeriesID, UserID: userID, Frequency: "weekly", Interval: 1,
		FromOn: anchor, ThroughOn: integrationDate(t, "2027-02-28"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedSeries.Frequency != "weekly" || updatedSeries.Interval != 1 {
		t.Fatalf("updated series=%#v", updatedSeries)
	}
	assertOccurrenceDates(t, ctx, storage, "tasks", monthlySeriesID, []string{
		"2027-01-31", "2027-02-07", "2027-02-14", "2027-02-21", "2027-02-28",
	})
	if err := storage.pool.QueryRow(ctx, `SELECT title FROM lifehub.tasks WHERE id=$1::uuid`, editedTaskID).Scan(&preservedTitle); err != nil {
		t.Fatal(err)
	}
	if preservedTitle != editedTitle {
		t.Fatalf("series edit overwrote occurrence exception: %q", preservedTitle)
	}

	series, err := storage.ListRecurrenceSeries(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(series) != 1 || series[0].Title != "Tutup buku" || series[0].SourceKind != "task" {
		t.Fatalf("series = %#v", series)
	}
	if other, err := storage.ListRecurrenceSeries(ctx, otherUserID); err != nil || len(other) != 0 {
		t.Fatalf("other owner series = %#v, error=%v", other, err)
	}
	if _, err := storage.GetRecurrenceSeries(ctx, otherUserID, monthlySeriesID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get error = %v, want ErrNotFound", err)
	}
	if err := storage.StopRecurrenceSeries(ctx, otherUserID, monthlySeriesID, anchor); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner stop error = %v, want ErrNotFound", err)
	}

	sweepSeriesID := mustUUID(t)
	sweepAnchor := integrationDate(t, "2026-08-27")
	sweepDue := time.Date(2026, time.August, 27, 13, 0, 0, 0, time.UTC)
	if _, err := storage.CreateRecurringTask(ctx, domain.CreateRecurringTaskParams{
		SeriesID: sweepSeriesID,
		Task: domain.CreateTaskParams{
			ID: mustUUID(t), UserID: userID, Title: "Rutinitas", Priority: domain.PriorityNormal, DueAt: &sweepDue,
		},
		Frequency: "daily", Interval: 1, AnchorOn: sweepAnchor, Timezone: "America/New_York",
		TimeLocal: "09:00:00", FromOn: sweepAnchor, ThroughOn: sweepAnchor,
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.MaterializeAllRecurrences(ctx, time.Date(2026, time.August, 27, 16, 0, 0, 0, time.UTC), 90); err != nil {
		t.Fatal(err)
	}
	var sweepCount int
	if err := storage.pool.QueryRow(ctx, `SELECT count(*) FROM lifehub.tasks WHERE series_id=$1::uuid`, sweepSeriesID).Scan(&sweepCount); err != nil {
		t.Fatal(err)
	}
	if sweepCount != 91 {
		t.Fatalf("rolling sweep count = %d, want 91", sweepCount)
	}

	dstSeriesID := mustUUID(t)
	dstStart := time.Date(2026, time.March, 7, 7, 30, 0, 0, time.UTC)
	duration := int64(3600)
	localTime := "02:30:00"
	if _, err := storage.CreateRecurringEvent(ctx, domain.CreateRecurringEventParams{
		SeriesID: dstSeriesID,
		Event: domain.CreateEventParams{
			ID: mustUUID(t), UserID: userID, Title: "Latihan pagi", Timezone: "America/New_York", StartsAt: &dstStart,
		},
		Frequency: "daily", Interval: 1, AnchorOn: integrationDate(t, "2026-03-07"),
		FromOn: integrationDate(t, "2026-03-07"), ThroughOn: integrationDate(t, "2026-03-09"),
		TimeLocal: &localTime, DurationSeconds: &duration,
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := storage.pool.Query(ctx, `
		SELECT occurrence_on::text, starts_at, ends_at
		FROM lifehub.events WHERE series_id=$1::uuid ORDER BY occurrence_on
	`, dstSeriesID)
	if err != nil {
		t.Fatal(err)
	}
	var gotStarts []string
	for rows.Next() {
		var occurrence string
		var startsAt, endsAt time.Time
		if err := rows.Scan(&occurrence, &startsAt, &endsAt); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		if endsAt.Sub(startsAt) != time.Hour {
			rows.Close()
			t.Fatalf("%s duration = %s, want 1h", occurrence, endsAt.Sub(startsAt))
		}
		gotStarts = append(gotStarts, occurrence+"="+startsAt.UTC().Format(time.RFC3339))
	}
	rows.Close()
	wantStarts := []string{
		"2026-03-07=2026-03-07T07:30:00Z",
		"2026-03-08=2026-03-08T07:30:00Z",
		"2026-03-09=2026-03-09T06:30:00Z",
	}
	if !reflect.DeepEqual(gotStarts, wantStarts) {
		t.Fatalf("DST starts = %v, want %v", gotStarts, wantStarts)
	}

	allDaySeriesID := mustUUID(t)
	allDayStart := integrationDate(t, "2026-09-10")
	allDayEnd := integrationDate(t, "2026-09-12")
	if _, err := storage.CreateRecurringEvent(ctx, domain.CreateRecurringEventParams{
		SeriesID: allDaySeriesID,
		Event: domain.CreateEventParams{
			ID: mustUUID(t), UserID: userID, Title: "Retret", AllDay: true, Timezone: "America/New_York",
			StartsOn: &allDayStart, EndsOn: &allDayEnd,
		},
		Frequency: "weekly", Interval: 1, AnchorOn: allDayStart,
		FromOn: allDayStart, ThroughOn: integrationDate(t, "2026-09-17"), AllDaySpan: 2,
	}); err != nil {
		t.Fatal(err)
	}
	var secondEnd string
	if err := storage.pool.QueryRow(ctx, `
		SELECT ends_on::text FROM lifehub.events
		WHERE series_id=$1::uuid AND occurrence_on='2026-09-17'::date
	`, allDaySeriesID).Scan(&secondEnd); err != nil {
		t.Fatal(err)
	}
	if secondEnd != "2026-09-19" {
		t.Fatalf("second all-day end = %q, want 2026-09-19", secondEnd)
	}

	billSeriesID := mustUUID(t)
	billAnchor := integrationDate(t, "2030-01-10")
	billDue := time.Date(2030, time.January, 10, 15, 0, 0, 0, time.UTC)
	anchorBill, err := storage.CreateRecurringBill(ctx, domain.CreateRecurringBillParams{
		SeriesID: billSeriesID,
		Bill: domain.CreateBillParams{
			ID: mustUUID(t), UserID: userID, Title: "Iuran", Amount: 250_000, Currency: "IDR", DueAt: billDue,
		},
		Frequency: "daily", Interval: 1, AnchorOn: billAnchor, Timezone: "America/New_York",
		TimeLocal: "10:00:00", FromOn: billAnchor, ThroughOn: integrationDate(t, "2030-01-12"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := storage.MarkBillPaid(ctx, userID, anchorBill.ID); err != nil {
		t.Fatal(err)
	}
	var futureBillID string
	if err := storage.pool.QueryRow(ctx, `
		SELECT id::text FROM lifehub.bills
		WHERE series_id=$1::uuid AND occurrence_on='2030-01-11'::date
	`, billSeriesID).Scan(&futureBillID); err != nil {
		t.Fatal(err)
	}
	minutes := 60
	reminderID := mustUUID(t)
	if _, err := storage.CreateReminder(ctx, domain.CreateReminderParams{
		ID: reminderID, UserID: userID, SourceKind: "bill", SourceID: futureBillID,
		ScheduleKind: "before_moment", MinutesBefore: &minutes,
	}); err != nil {
		t.Fatal(err)
	}
	if err := storage.StopRecurrenceSeries(ctx, userID, billSeriesID, billAnchor); err != nil {
		t.Fatal(err)
	}
	var preserved, future, reminderDefinitions int
	if err := storage.pool.QueryRow(ctx, `SELECT count(*) FROM lifehub.bills WHERE series_id=$1::uuid AND paid_at IS NOT NULL`, billSeriesID).Scan(&preserved); err != nil {
		t.Fatal(err)
	}
	if err := storage.pool.QueryRow(ctx, `SELECT count(*) FROM lifehub.bills WHERE series_id=$1::uuid AND paid_at IS NULL`, billSeriesID).Scan(&future); err != nil {
		t.Fatal(err)
	}
	if err := storage.pool.QueryRow(ctx, `SELECT count(*) FROM lifehub.reminder_definitions WHERE id=$1::uuid`, reminderID).Scan(&reminderDefinitions); err != nil {
		t.Fatal(err)
	}
	if preserved != 1 || future != 0 || reminderDefinitions != 0 {
		t.Fatalf("stop result preserved=%d future=%d reminder_definitions=%d, want 1/0/0", preserved, future, reminderDefinitions)
	}
	stopped, err := storage.GetRecurrenceSeries(ctx, userID, billSeriesID)
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Active {
		t.Fatal("stopped series is still active")
	}
}

func integrationDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func assertOccurrenceDates(t *testing.T, ctx context.Context, storage *Store, table, seriesID string, want []string) {
	t.Helper()
	query := "SELECT occurrence_on::text FROM lifehub." + table + " WHERE series_id=$1::uuid AND excluded_at IS NULL ORDER BY occurrence_on"
	rows, err := storage.pool.Query(ctx, query, seriesID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := make([]string, 0, len(want))
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			t.Fatal(err)
		}
		got = append(got, value)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s recurrence dates = %v, want %v", table, got, want)
	}
}

package store

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/migrate"
	"lifehub/services/api/internal/timeutil"
)

func TestStorePostgresOwnershipAndIdempotentCompletion(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := migrate.Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	storage, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)

	userA := mustUUID(t)
	userB := mustUUID(t)
	profile, err := storage.GetProfile(ctx, userA)
	if err != nil {
		t.Fatal(err)
	}
	if profile.Timezone != nil {
		t.Fatalf("new profile timezone = %q, want unconfirmed", *profile.Timezone)
	}
	if _, err := storage.GetProfile(ctx, userB); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userA, userB})
	})

	taskID, _ := NewUUID()
	task, err := storage.CreateTask(ctx, domain.CreateTaskParams{
		ID: taskID, UserID: userA, Title: "Submit laporan", Priority: domain.PriorityHigh,
	})
	if err != nil {
		t.Fatal(err)
	}
	if task.CompletedAt != nil {
		t.Fatal("new task is completed")
	}

	if _, err := storage.CompleteTask(ctx, userB, taskID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner completion error = %v, want not found", err)
	}

	first, err := storage.CompleteTask(ctx, userA, taskID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := storage.CompleteTask(ctx, userA, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if first.CompletedAt == nil || second.CompletedAt == nil || !first.CompletedAt.Equal(*second.CompletedAt) {
		t.Fatalf("completion was not idempotent: first=%v second=%v", first.CompletedAt, second.CompletedAt)
	}
}

func TestStorePostgresTodayScope(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	_, _ = storage.GetProfile(ctx, userID)
	_, _ = storage.GetProfile(ctx, otherUserID)
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	dueToday := start.Add(4 * time.Hour)
	future := end.Add(time.Hour)
	for _, params := range []domain.CreateTaskParams{
		{ID: mustUUID(t), UserID: userID, Title: "Anytime", Priority: domain.PriorityNormal},
		{ID: mustUUID(t), UserID: userID, Title: "Today", Priority: domain.PriorityNormal, DueAt: &dueToday},
		{ID: mustUUID(t), UserID: userID, Title: "Future", Priority: domain.PriorityNormal, DueAt: &future},
		{ID: mustUUID(t), UserID: otherUserID, Title: "Other owner", Priority: domain.PriorityNormal, DueAt: &dueToday},
	} {
		if _, err := storage.CreateTask(ctx, params); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := storage.ListTodayTasks(ctx, userID, start, end, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].Title != "Today" || tasks[1].Title != "Anytime" {
		t.Fatalf("unexpected Today tasks: %#v", tasks)
	}
}

func TestStorePostgresTodayDoesNotTruncateAnytimeTasks(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = $1::uuid", userID)
	})

	const taskCount = 25
	for range taskCount {
		if _, err := storage.CreateTask(ctx, domain.CreateTaskParams{
			ID: mustUUID(t), UserID: userID, Title: "Anytime", Priority: domain.PriorityNormal,
		}); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	tasks, err := storage.ListTodayTasks(ctx, userID, start, end, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != taskCount {
		t.Fatalf("Today returned %d anytime tasks, want %d", len(tasks), taskCount)
	}
}

func TestStorePostgresTodayEventsUseOverlapAllDayAndOwnership(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	date := "2026-08-19"
	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	pointAtStart := start
	beforeStart := start.Add(-time.Hour)
	afterStart := start.Add(time.Hour)
	atEnd := end
	afterEnd := end.Add(time.Hour)
	dayBefore := time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)
	today := dayBefore.AddDate(0, 0, 1)

	events := []domain.CreateEventParams{
		{ID: mustUUID(t), UserID: userID, Title: "Point at start", Timezone: "Asia/Jakarta", StartsAt: &pointAtStart},
		{ID: mustUUID(t), UserID: userID, Title: "Range overlap", Timezone: "Asia/Jakarta", StartsAt: &beforeStart, EndsAt: &afterStart},
		{ID: mustUUID(t), UserID: userID, Title: "Range ends at start", Timezone: "Asia/Jakarta", StartsAt: &beforeStart, EndsAt: &pointAtStart},
		{ID: mustUUID(t), UserID: userID, Title: "Range starts at end", Timezone: "Asia/Jakarta", StartsAt: &atEnd, EndsAt: &afterEnd},
		{ID: mustUUID(t), UserID: userID, Title: "All-day inclusive", Timezone: "Asia/Jakarta", AllDay: true, StartsOn: &dayBefore, EndsOn: &today},
		{ID: mustUUID(t), UserID: userID, Title: "All-day before", Timezone: "Asia/Jakarta", AllDay: true, StartsOn: &dayBefore},
		{ID: mustUUID(t), UserID: otherUserID, Title: "Other owner all-day", Timezone: "Asia/Jakarta", AllDay: true, StartsOn: &today},
	}
	for _, params := range events {
		if _, err := storage.CreateEvent(ctx, params); err != nil {
			t.Fatalf("create %q: %v", params.Title, err)
		}
	}

	gotEvents, err := storage.ListTodayEvents(ctx, userID, date, date, start, end)
	if err != nil {
		t.Fatal(err)
	}
	gotTitles := make(map[string]bool, len(gotEvents))
	for _, event := range gotEvents {
		gotTitles[event.Title] = true
	}
	for _, title := range []string{"Point at start", "Range overlap", "All-day inclusive"} {
		if !gotTitles[title] {
			t.Fatalf("missing %q from Today events: %#v", title, gotEvents)
		}
	}
	for _, title := range []string{"Range ends at start", "Range starts at end", "All-day before", "Other owner all-day"} {
		if gotTitles[title] {
			t.Fatalf("unexpected %q in Today events: %#v", title, gotEvents)
		}
	}
	if len(gotEvents) != 3 {
		t.Fatalf("Today returned %d events, want 3: %#v", len(gotEvents), gotEvents)
	}

	invalidID := mustUUID(t)
	if _, err := storage.CreateEvent(ctx, domain.CreateEventParams{
		ID: invalidID, UserID: userID, Title: "Invalid union", Timezone: "Asia/Jakarta",
		AllDay: true, StartsAt: &pointAtStart, StartsOn: &today,
	}); err == nil {
		t.Fatal("database accepted mixed timed/all-day columns")
	}
	equalEndID := mustUUID(t)
	if _, err := storage.CreateEvent(ctx, domain.CreateEventParams{
		ID: equalEndID, UserID: userID, Title: "Invalid zero duration", Timezone: "Asia/Jakarta",
		StartsAt: &pointAtStart, EndsAt: &pointAtStart,
	}); err == nil {
		t.Fatal("database accepted a timed event whose end equals its start")
	}
}

func TestStorePostgresTodayDoesNotTruncateEvents(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = $1::uuid", userID)
	})

	const eventCount = 25
	startsAt := time.Date(2026, time.August, 19, 2, 0, 0, 0, time.UTC)
	for range eventCount {
		if _, err := storage.CreateEvent(ctx, domain.CreateEventParams{
			ID: mustUUID(t), UserID: userID, Title: "Point event", Timezone: "Asia/Jakarta", StartsAt: &startsAt,
		}); err != nil {
			t.Fatal(err)
		}
	}

	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	events, err := storage.ListTodayEvents(ctx, userID, "2026-08-19", "2026-08-19", start, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != eventCount {
		t.Fatalf("Today returned %d events, want %d", len(events), eventCount)
	}
}

func TestStorePostgresBillOwnershipIdempotentPaymentAndConstraints(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	dueAt := time.Date(2026, time.August, 19, 7, 0, 0, 0, time.UTC)
	billID := mustUUID(t)
	bill, err := storage.CreateBill(ctx, domain.CreateBillParams{
		ID: billID, UserID: userID, Title: "Internet", Amount: domain.MaxBillAmount, Currency: "IDR", DueAt: dueAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if bill.Amount != domain.MaxBillAmount || bill.PaidAt != nil {
		t.Fatalf("unexpected created bill: %#v", bill)
	}
	if _, err := storage.MarkBillPaid(ctx, otherUserID, billID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner payment error = %v, want not found", err)
	}
	first, err := storage.MarkBillPaid(ctx, userID, billID)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	second, err := storage.MarkBillPaid(ctx, userID, billID)
	if err != nil {
		t.Fatal(err)
	}
	if first.PaidAt == nil || second.PaidAt == nil || !first.PaidAt.Equal(*second.PaidAt) || !first.UpdatedAt.Equal(second.UpdatedAt) {
		t.Fatalf("payment was not idempotent: first=%#v second=%#v", first, second)
	}

	invalidBills := []domain.CreateBillParams{
		{ID: mustUUID(t), UserID: userID, Title: "Zero", Amount: 0, Currency: "IDR", DueAt: dueAt},
		{ID: mustUUID(t), UserID: userID, Title: "Unsafe", Amount: domain.MaxBillAmount + 1, Currency: "IDR", DueAt: dueAt},
		{ID: mustUUID(t), UserID: userID, Title: "Currency", Amount: 1, Currency: "idr", DueAt: dueAt},
	}
	for _, params := range invalidBills {
		if _, err := storage.CreateBill(ctx, params); err == nil {
			t.Fatalf("database accepted invalid bill %#v", params)
		}
	}
}

func TestStorePostgresTodayBillsScopeBoundariesOwnershipAndNoTruncation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	start := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	dueToday := start.Add(time.Hour)
	future := end
	const unpaidCount = 25
	for range unpaidCount {
		if _, err := storage.CreateBill(ctx, domain.CreateBillParams{
			ID: mustUUID(t), UserID: userID, Title: "Unpaid", Amount: 1000, Currency: "IDR", DueAt: dueToday,
		}); err != nil {
			t.Fatal(err)
		}
	}

	paidAtStartID := mustUUID(t)
	paidAtEndID := mustUUID(t)
	paidBeforeID := mustUUID(t)
	futureID := mustUUID(t)
	otherOwnerID := mustUUID(t)
	for _, params := range []domain.CreateBillParams{
		{ID: paidAtStartID, UserID: userID, Title: "Paid at start", Amount: 1, Currency: "IDR", DueAt: future},
		{ID: paidAtEndID, UserID: userID, Title: "Paid at end", Amount: 1, Currency: "IDR", DueAt: future},
		{ID: paidBeforeID, UserID: userID, Title: "Paid before", Amount: 1, Currency: "IDR", DueAt: dueToday},
		{ID: futureID, UserID: userID, Title: "Future unpaid", Amount: 1, Currency: "IDR", DueAt: future},
		{ID: otherOwnerID, UserID: otherUserID, Title: "Other owner", Amount: 1, Currency: "IDR", DueAt: dueToday},
	} {
		if _, err := storage.CreateBill(ctx, params); err != nil {
			t.Fatal(err)
		}
	}
	for billID, paidAt := range map[string]time.Time{
		paidAtStartID: start,
		paidAtEndID:   end,
		paidBeforeID:  start.Add(-time.Microsecond),
	} {
		if _, err := storage.pool.Exec(ctx, `
			UPDATE lifehub.bills SET paid_at = $2, updated_at = $2 WHERE id = $1::uuid
		`, billID, paidAt); err != nil {
			t.Fatal(err)
		}
	}

	bills, err := storage.ListTodayBills(ctx, userID, start, end, end)
	if err != nil {
		t.Fatal(err)
	}
	if len(bills) != unpaidCount+1 {
		t.Fatalf("Today returned %d bills, want %d: %#v", len(bills), unpaidCount+1, bills)
	}
	gotIDs := make(map[string]bool, len(bills))
	for _, bill := range bills {
		gotIDs[bill.ID] = true
	}
	if !gotIDs[paidAtStartID] {
		t.Fatal("bill paid exactly at local-day start was excluded")
	}
	for _, excludedID := range []string{paidAtEndID, paidBeforeID, futureID, otherOwnerID} {
		if gotIDs[excludedID] {
			t.Fatalf("excluded bill %s appeared in Today", excludedID)
		}
	}
}

func TestStorePostgresDocumentCRUDOwnershipAndNotesClear(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	documentID := mustUUID(t)
	notes := "Perpanjang segera"
	expiresOn := time.Date(2020, time.January, 2, 0, 0, 0, 0, time.UTC)
	document, err := storage.CreateDocument(ctx, domain.CreateDocumentParams{
		ID: documentID, UserID: userID, Name: "SIM", Category: "license", Notes: &notes, ExpiresOn: expiresOn,
	})
	if err != nil {
		t.Fatal(err)
	}
	if document.ExpiresOn != "2020-01-02" || document.Notes == nil || *document.Notes != notes {
		t.Fatalf("created document = %#v", document)
	}
	owned, err := storage.ListDocuments(ctx, userID)
	if err != nil || len(owned) != 1 || owned[0].ID != documentID {
		t.Fatalf("owned documents = %#v, err=%v", owned, err)
	}
	other, err := storage.ListDocuments(ctx, otherUserID)
	if err != nil || len(other) != 0 {
		t.Fatalf("other-owner list = %#v, err=%v", other, err)
	}
	if _, err := storage.GetDocument(ctx, otherUserID, documentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get error = %v", err)
	}
	name := "SIM A"
	if _, err := storage.UpdateDocument(ctx, domain.UpdateDocumentParams{
		ID: documentID, UserID: otherUserID, Name: &name,
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner update error = %v", err)
	}
	cleared, err := storage.UpdateDocument(ctx, domain.UpdateDocumentParams{
		ID: documentID, UserID: userID, NotesSet: true, Notes: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Notes != nil {
		t.Fatalf("notes were not cleared: %#v", cleared)
	}
	category := "identity"
	newExpiry := time.Date(2027, time.November, 6, 0, 0, 0, 0, time.UTC)
	updated, err := storage.UpdateDocument(ctx, domain.UpdateDocumentParams{
		ID: documentID, UserID: userID, Name: &name, Category: &category, ExpiresOn: &newExpiry,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != name || updated.Category != category || updated.ExpiresOn != "2027-11-06" || updated.Notes != nil {
		t.Fatalf("updated document = %#v", updated)
	}
	if err := storage.DeleteDocument(ctx, otherUserID, documentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner delete error = %v", err)
	}
	if err := storage.DeleteDocument(ctx, userID, documentID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetDocument(ctx, userID, documentID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted document get error = %v", err)
	}

	tooLongNotes := strings.Repeat("a", 5001)
	invalidDocuments := []domain.CreateDocumentParams{
		{ID: mustUUID(t), UserID: userID, Name: " ", Category: "license", ExpiresOn: expiresOn},
		{ID: mustUUID(t), UserID: userID, Name: "Invalid category", Category: "passport", ExpiresOn: expiresOn},
		{ID: mustUUID(t), UserID: userID, Name: "Long notes", Category: "other", Notes: &tooLongNotes, ExpiresOn: expiresOn},
	}
	for _, params := range invalidDocuments {
		if _, err := storage.CreateDocument(ctx, params); err == nil {
			t.Fatalf("database accepted invalid document %#v", params)
		}
	}
}

func TestStorePostgresDocumentsDateBoundaryOwnershipAndNoTruncation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	expiredDate := time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)
	todayDate := expiredDate.AddDate(0, 0, 1)
	day30 := todayDate.AddDate(0, 0, 30)
	day31 := todayDate.AddDate(0, 0, 31)
	const expiredCount = 25
	for range expiredCount {
		if _, err := storage.CreateDocument(ctx, domain.CreateDocumentParams{
			ID: mustUUID(t), UserID: userID, Name: "Expired", Category: "other", ExpiresOn: expiredDate,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, params := range []domain.CreateDocumentParams{
		{ID: mustUUID(t), UserID: userID, Name: "Today", Category: "identity", ExpiresOn: todayDate},
		{ID: mustUUID(t), UserID: userID, Name: "Day 30", Category: "insurance", ExpiresOn: day30},
		{ID: mustUUID(t), UserID: userID, Name: "Day 31", Category: "education", ExpiresOn: day31},
		{ID: mustUUID(t), UserID: otherUserID, Name: "Other owner", Category: "work", ExpiresOn: expiredDate},
	} {
		if _, err := storage.CreateDocument(ctx, params); err != nil {
			t.Fatal(err)
		}
	}

	allDocuments, err := storage.ListDocuments(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(allDocuments) != expiredCount+3 || allDocuments[0].ExpiresOn != "2026-08-18" || allDocuments[len(allDocuments)-1].ExpiresOn != "2026-09-19" {
		t.Fatalf("ordered full list = %#v", allDocuments)
	}
	todayDocuments, err := storage.ListTodayDocuments(ctx, userID, day30.Format(time.DateOnly))
	if err != nil {
		t.Fatal(err)
	}
	if len(todayDocuments) != expiredCount+2 {
		t.Fatalf("Today returned %d documents, want %d: %#v", len(todayDocuments), expiredCount+2, todayDocuments)
	}
	for _, document := range todayDocuments {
		if document.Name == "Day 31" || document.Name == "Other owner" {
			t.Fatalf("out-of-scope document appeared: %#v", document)
		}
	}
}

func TestStorePostgresTaskEventBillCorrectionsOwnershipAndIdempotency(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	notes := "private"
	dueAt := time.Date(2026, time.August, 20, 2, 0, 0, 0, time.UTC)
	taskID := mustUUID(t)
	if _, err := storage.CreateTask(ctx, domain.CreateTaskParams{
		ID: taskID, UserID: userID, Title: "Task", Notes: &notes, Priority: domain.PriorityNormal, DueAt: &dueAt,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetTask(ctx, otherUserID, taskID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get task = %v", err)
	}
	completed, err := storage.CompleteTask(ctx, userID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	newTitle := "Corrected task"
	if _, err := storage.UpdateTask(ctx, domain.UpdateTaskParams{ID: taskID, UserID: otherUserID, Title: &newTitle}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner update task = %v", err)
	}
	updatedTask, err := storage.UpdateTask(ctx, domain.UpdateTaskParams{
		ID: taskID, UserID: userID, Title: &newTitle, NotesSet: true, DueAtSet: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedTask.Title != newTitle || updatedTask.Notes != nil || updatedTask.DueAt != nil || updatedTask.CompletedAt == nil || !updatedTask.CompletedAt.Equal(*completed.CompletedAt) {
		t.Fatalf("updated completed task = %#v", updatedTask)
	}
	firstUncomplete, err := storage.UncompleteTask(ctx, userID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	secondUncomplete, err := storage.UncompleteTask(ctx, userID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if firstUncomplete.CompletedAt != nil || secondUncomplete.CompletedAt != nil || !firstUncomplete.UpdatedAt.Equal(secondUncomplete.UpdatedAt) {
		t.Fatalf("uncomplete was not idempotent: first=%#v second=%#v", firstUncomplete, secondUncomplete)
	}
	if _, err := storage.UncompleteTask(ctx, otherUserID, taskID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner uncomplete = %v", err)
	}

	eventID := mustUUID(t)
	startsAt := time.Date(2026, time.August, 20, 3, 0, 0, 0, time.UTC)
	endsAt := startsAt.Add(time.Hour)
	if _, err := storage.CreateEvent(ctx, domain.CreateEventParams{
		ID: eventID, UserID: userID, Title: "Event", Notes: &notes, Timezone: "Asia/Jakarta", StartsAt: &startsAt, EndsAt: &endsAt,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetEvent(ctx, otherUserID, eventID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get event = %v", err)
	}
	eventTitle := "Corrected event"
	if _, err := storage.UpdateEvent(ctx, domain.UpdateEventParams{ID: eventID, UserID: otherUserID, Title: &eventTitle}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner update event = %v", err)
	}
	metadataEvent, err := storage.UpdateEvent(ctx, domain.UpdateEventParams{ID: eventID, UserID: userID, Title: &eventTitle})
	if err != nil {
		t.Fatal(err)
	}
	if metadataEvent.Title != eventTitle || metadataEvent.StartsAt == nil || !metadataEvent.StartsAt.Equal(startsAt) || metadataEvent.EndsAt == nil || !metadataEvent.EndsAt.Equal(endsAt) {
		t.Fatalf("metadata update replaced schedule: %#v", metadataEvent)
	}
	startsOn := time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC)
	allDayEvent, err := storage.UpdateEvent(ctx, domain.UpdateEventParams{
		ID: eventID, UserID: userID, NotesSet: true, ScheduleSet: true, AllDay: true,
		Timezone: "Asia/Jakarta", StartsOn: &startsOn,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !allDayEvent.AllDay || allDayEvent.StartsOn == nil || allDayEvent.StartsAt != nil || allDayEvent.EndsAt != nil || allDayEvent.Notes != nil {
		t.Fatalf("timed to all-day = %#v", allDayEvent)
	}
	timedAgain, err := storage.UpdateEvent(ctx, domain.UpdateEventParams{
		ID: eventID, UserID: userID, ScheduleSet: true, Timezone: "Asia/Jakarta", StartsAt: &startsAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	if timedAgain.AllDay || timedAgain.StartsAt == nil || timedAgain.StartsOn != nil || timedAgain.EndsOn != nil {
		t.Fatalf("all-day to timed = %#v", timedAgain)
	}

	billID := mustUUID(t)
	bill, err := storage.CreateBill(ctx, domain.CreateBillParams{
		ID: billID, UserID: userID, Title: "Bill", Notes: &notes, Amount: 1000, Currency: "IDR", DueAt: dueAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	paid, err := storage.MarkBillPaid(ctx, userID, bill.ID)
	if err != nil {
		t.Fatal(err)
	}
	amount := int64(2000)
	if _, err := storage.GetBill(ctx, otherUserID, billID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get bill = %v", err)
	}
	if _, err := storage.UpdateBill(ctx, domain.UpdateBillParams{ID: billID, UserID: otherUserID, Amount: &amount}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner update bill = %v", err)
	}
	updatedBill, err := storage.UpdateBill(ctx, domain.UpdateBillParams{
		ID: billID, UserID: userID, NotesSet: true, Amount: &amount,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedBill.PaidAt == nil || !updatedBill.PaidAt.Equal(*paid.PaidAt) || updatedBill.Amount != amount || updatedBill.Notes != nil {
		t.Fatalf("updated paid bill = %#v", updatedBill)
	}
	firstUnpaid, err := storage.MarkBillUnpaid(ctx, userID, billID)
	if err != nil {
		t.Fatal(err)
	}
	secondUnpaid, err := storage.MarkBillUnpaid(ctx, userID, billID)
	if err != nil {
		t.Fatal(err)
	}
	if firstUnpaid.PaidAt != nil || secondUnpaid.PaidAt != nil || !firstUnpaid.UpdatedAt.Equal(secondUnpaid.UpdatedAt) {
		t.Fatalf("mark-unpaid was not idempotent: first=%#v second=%#v", firstUnpaid, secondUnpaid)
	}
	if _, err := storage.MarkBillUnpaid(ctx, otherUserID, billID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner mark-unpaid = %v", err)
	}

	paidAt := time.Date(2026, time.August, 19, 4, 0, 0, 123, time.UTC)
	historyIDs := []string{
		"00000000-0000-4000-8000-000000007101",
		"00000000-0000-4000-8000-000000007102",
		"00000000-0000-4000-8000-000000007103",
		"00000000-0000-4000-8000-000000007104",
	}
	for _, id := range historyIDs {
		if _, err := storage.CreateBill(ctx, domain.CreateBillParams{ID: id, UserID: userID, Title: "History", Amount: 1, Currency: "IDR", DueAt: dueAt}); err != nil {
			t.Fatal(err)
		}
		if _, err := storage.pool.Exec(ctx, "UPDATE lifehub.bills SET paid_at = $2 WHERE id = $1::uuid", id, paidAt); err != nil {
			t.Fatal(err)
		}
	}
	firstPage, err := storage.ListBills(ctx, userID, "paid", 2, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(firstPage) != 2 || firstPage[0].ID != historyIDs[0] || firstPage[1].ID != historyIDs[1] {
		t.Fatalf("first paid page = %#v", firstPage)
	}
	afterID := firstPage[1].ID
	secondPage, err := storage.ListBills(ctx, userID, "paid", 2, firstPage[1].PaidAt, &afterID)
	if err != nil {
		t.Fatal(err)
	}
	if len(secondPage) != 2 || secondPage[0].ID != historyIDs[2] || secondPage[1].ID != historyIDs[3] {
		t.Fatalf("second paid page = %#v", secondPage)
	}

	if err := storage.DeleteTask(ctx, otherUserID, taskID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner delete task = %v", err)
	}
	if err := storage.DeleteEvent(ctx, otherUserID, eventID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner delete event = %v", err)
	}
	if err := storage.DeleteBill(ctx, otherUserID, billID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner delete bill = %v", err)
	}
	if err := storage.DeleteTask(ctx, userID, taskID); err != nil {
		t.Fatal(err)
	}
	if err := storage.DeleteEvent(ctx, userID, eventID); err != nil {
		t.Fatal(err)
	}
	if err := storage.DeleteBill(ctx, userID, billID); err != nil {
		t.Fatal(err)
	}
}

func TestStorePostgresAgendaAndUpcomingBoundariesOwnershipAndNoTruncation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err := storage.GetProfile(ctx, userID); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
	})

	todayStart := time.Date(2026, time.August, 18, 17, 0, 0, 0, time.UTC)
	rangeStart := todayStart.AddDate(0, 0, 1)
	rangeEnd := todayStart.AddDate(0, 0, 31)
	dayOneAt := rangeStart.Add(8 * time.Hour)
	day30At := rangeEnd.Add(-14 * time.Hour)
	day31At := rangeEnd.Add(8 * time.Hour)
	const bulkTasks = 25
	for range bulkTasks {
		if _, err := storage.CreateTask(ctx, domain.CreateTaskParams{
			ID: mustUUID(t), UserID: userID, Title: "Bulk", Priority: domain.PriorityNormal, DueAt: &dayOneAt,
		}); err != nil {
			t.Fatal(err)
		}
	}
	for _, params := range []domain.CreateTaskParams{
		{ID: mustUUID(t), UserID: userID, Title: "Day 30", Priority: domain.PriorityHigh, DueAt: &day30At},
		{ID: mustUUID(t), UserID: userID, Title: "Day 31", Priority: domain.PriorityHigh, DueAt: &day31At},
		{ID: mustUUID(t), UserID: otherUserID, Title: "Other", Priority: domain.PriorityHigh, DueAt: &dayOneAt},
	} {
		if _, err := storage.CreateTask(ctx, params); err != nil {
			t.Fatal(err)
		}
	}

	continuingStart := rangeStart.Add(-time.Hour)
	continuingEnd := rangeStart.Add(time.Hour)
	pointAtEnd := rangeEnd
	day30Date := time.Date(2026, time.September, 18, 0, 0, 0, 0, time.UTC)
	for _, params := range []domain.CreateEventParams{
		{ID: mustUUID(t), UserID: userID, Title: "Continuing", Timezone: "Asia/Jakarta", StartsAt: &continuingStart, EndsAt: &continuingEnd},
		{ID: mustUUID(t), UserID: userID, Title: "Day 30 all-day", Timezone: "Asia/Jakarta", AllDay: true, StartsOn: &day30Date},
		{ID: mustUUID(t), UserID: userID, Title: "At range end", Timezone: "Asia/Jakarta", StartsAt: &pointAtEnd},
		{ID: mustUUID(t), UserID: otherUserID, Title: "Other", Timezone: "Asia/Jakarta", StartsAt: &dayOneAt},
	} {
		if _, err := storage.CreateEvent(ctx, params); err != nil {
			t.Fatal(err)
		}
	}

	for _, params := range []domain.CreateBillParams{
		{ID: mustUUID(t), UserID: userID, Title: "Day 1", Amount: 1, Currency: "IDR", DueAt: dayOneAt},
		{ID: mustUUID(t), UserID: userID, Title: "Day 30", Amount: 1, Currency: "IDR", DueAt: day30At},
		{ID: mustUUID(t), UserID: userID, Title: "Day 31", Amount: 1, Currency: "IDR", DueAt: day31At},
		{ID: mustUUID(t), UserID: otherUserID, Title: "Other", Amount: 1, Currency: "IDR", DueAt: dayOneAt},
	} {
		if _, err := storage.CreateBill(ctx, params); err != nil {
			t.Fatal(err)
		}
	}
	for _, params := range []domain.CreateDocumentParams{
		{ID: mustUUID(t), UserID: userID, Name: "Day 1", Category: "other", ExpiresOn: time.Date(2026, time.August, 20, 0, 0, 0, 0, time.UTC)},
		{ID: mustUUID(t), UserID: userID, Name: "Day 30", Category: "other", ExpiresOn: day30Date},
		{ID: mustUUID(t), UserID: userID, Name: "Day 31", Category: "other", ExpiresOn: day30Date.AddDate(0, 0, 1)},
		{ID: mustUUID(t), UserID: otherUserID, Name: "Other", Category: "other", ExpiresOn: day30Date},
	} {
		if _, err := storage.CreateDocument(ctx, params); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := storage.ListAgendaTasks(ctx, userID, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	events, err := storage.ListEvents(ctx, userID, "2026-08-20", "2026-09-18", rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	bills, err := storage.ListAgendaBills(ctx, userID, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	documents, err := storage.ListAgendaDocuments(ctx, userID, "2026-08-20", "2026-09-18")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != bulkTasks+1 || len(events) != 2 || len(bills) != 2 || len(documents) != 2 {
		t.Fatalf("agenda counts tasks=%d events=%d bills=%d documents=%d", len(tasks), len(events), len(bills), len(documents))
	}
	for _, event := range events {
		if event.Title == "At range end" || event.Title == "Other" {
			t.Fatalf("out-of-range event appeared: %#v", event)
		}
	}

	todayTasks, err := storage.ListTodayTasks(ctx, userID, todayStart, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	todayEvents, err := storage.ListTodayEvents(ctx, userID, "2026-08-19", "2026-09-18", todayStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	todayBills, err := storage.ListTodayBills(ctx, userID, todayStart, rangeStart, rangeEnd)
	if err != nil {
		t.Fatal(err)
	}
	if len(todayTasks) != bulkTasks+1 || len(todayEvents) != 2 || len(todayBills) != 2 {
		t.Fatalf("Today+Upcoming counts tasks=%d events=%d bills=%d", len(todayTasks), len(todayEvents), len(todayBills))
	}
}

func TestStorePostgresLocalMidnightGapRangesDoNotLeakAdjacentDates(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := migrate.Up(ctx, databaseURL); err != nil {
		t.Fatal(err)
	}
	storage, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(storage.Close)

	tests := []struct {
		name      string
		timezone  string
		date      string
		wantStart time.Time
		wantEnd   time.Time
		skipped   string
	}{
		{
			name:      "Cairo midnight gap",
			timezone:  "Africa/Cairo",
			date:      "2026-04-24",
			wantStart: time.Date(2026, time.April, 23, 22, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.April, 24, 21, 0, 0, 0, time.UTC),
		},
		{
			name:      "Santiago midnight gap",
			timezone:  "America/Santiago",
			date:      "2026-09-06",
			wantStart: time.Date(2026, time.September, 6, 4, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2026, time.September, 7, 3, 0, 0, 0, time.UTC),
		},
		{
			name:      "Apia day before skipped date",
			timezone:  "Pacific/Apia",
			date:      "2011-12-29",
			wantStart: time.Date(2011, time.December, 29, 10, 0, 0, 0, time.UTC),
			wantEnd:   time.Date(2011, time.December, 30, 10, 0, 0, 0, time.UTC),
			skipped:   "2011-12-30",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			location, err := timeutil.LoadLocation(test.timezone)
			if err != nil {
				t.Fatal(err)
			}
			date, err := time.Parse(time.DateOnly, test.date)
			if err != nil {
				t.Fatal(err)
			}
			start, err := timeutil.LocalDateStart(date, location)
			if err != nil {
				t.Fatal(err)
			}
			end, err := timeutil.LocalDateEnd(date, location)
			if err != nil {
				t.Fatal(err)
			}
			if !start.Equal(test.wantStart) || !end.Equal(test.wantEnd) {
				t.Fatalf("range = [%s,%s), want [%s,%s)", start, end, test.wantStart, test.wantEnd)
			}
			if test.skipped != "" {
				skipped, err := time.Parse(time.DateOnly, test.skipped)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := timeutil.LocalDateStart(skipped, location); !errors.Is(err, timeutil.ErrSkippedLocalDate) {
					t.Fatalf("skipped date error = %v", err)
				}
			}

			userID := mustUUID(t)
			otherUserID := mustUUID(t)
			if _, err := storage.GetProfile(ctx, userID); err != nil {
				t.Fatal(err)
			}
			if _, err := storage.GetProfile(ctx, otherUserID); err != nil {
				t.Fatal(err)
			}
			if _, err := storage.UpdateProfileTimezone(ctx, userID, test.timezone); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cleanupCancel()
				_, _ = storage.pool.Exec(cleanupContext, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{userID, otherUserID})
			})

			prior := start.Add(-time.Second)
			insideEnd := end.Add(-time.Second)
			for _, params := range []domain.CreateTaskParams{
				{ID: mustUUID(t), UserID: userID, Title: "Prior", Priority: domain.PriorityNormal, DueAt: &prior},
				{ID: mustUUID(t), UserID: userID, Title: "At start", Priority: domain.PriorityNormal, DueAt: &start},
				{ID: mustUUID(t), UserID: userID, Title: "Before end", Priority: domain.PriorityNormal, DueAt: &insideEnd},
				{ID: mustUUID(t), UserID: userID, Title: "At end", Priority: domain.PriorityNormal, DueAt: &end},
				{ID: mustUUID(t), UserID: otherUserID, Title: "Other owner", Priority: domain.PriorityNormal, DueAt: &start},
			} {
				if _, err := storage.CreateTask(ctx, params); err != nil {
					t.Fatal(err)
				}
			}
			tasks, err := storage.ListAgendaTasks(ctx, userID, start, end)
			if err != nil {
				t.Fatal(err)
			}
			assertIntegrationTitles(t, "tasks", taskTitles(tasks), []string{"At start", "Before end"})

			for _, params := range []domain.CreateBillParams{
				{ID: mustUUID(t), UserID: userID, Title: "Prior", Amount: 1, Currency: "IDR", DueAt: prior},
				{ID: mustUUID(t), UserID: userID, Title: "At start", Amount: 1, Currency: "IDR", DueAt: start},
				{ID: mustUUID(t), UserID: userID, Title: "Before end", Amount: 1, Currency: "IDR", DueAt: insideEnd},
				{ID: mustUUID(t), UserID: userID, Title: "At end", Amount: 1, Currency: "IDR", DueAt: end},
				{ID: mustUUID(t), UserID: otherUserID, Title: "Other owner", Amount: 1, Currency: "IDR", DueAt: start},
			} {
				if _, err := storage.CreateBill(ctx, params); err != nil {
					t.Fatal(err)
				}
			}
			bills, err := storage.ListAgendaBills(ctx, userID, start, end)
			if err != nil {
				t.Fatal(err)
			}
			assertIntegrationTitles(t, "bills", billTitles(bills), []string{"At start", "Before end"})

			endsAtStart := start
			overlapsEnd := start.Add(time.Second)
			for _, params := range []domain.CreateEventParams{
				{ID: mustUUID(t), UserID: userID, Title: "Prior point", Timezone: test.timezone, StartsAt: &prior},
				{ID: mustUUID(t), UserID: userID, Title: "At start point", Timezone: test.timezone, StartsAt: &start},
				{ID: mustUUID(t), UserID: userID, Title: "Before end point", Timezone: test.timezone, StartsAt: &insideEnd},
				{ID: mustUUID(t), UserID: userID, Title: "At end point", Timezone: test.timezone, StartsAt: &end},
				{ID: mustUUID(t), UserID: userID, Title: "Ends at start", Timezone: test.timezone, StartsAt: &prior, EndsAt: &endsAtStart},
				{ID: mustUUID(t), UserID: userID, Title: "Overlaps start", Timezone: test.timezone, StartsAt: &prior, EndsAt: &overlapsEnd},
				{ID: mustUUID(t), UserID: otherUserID, Title: "Other owner", Timezone: test.timezone, StartsAt: &start},
			} {
				if _, err := storage.CreateEvent(ctx, params); err != nil {
					t.Fatal(err)
				}
			}
			events, err := storage.ListEvents(ctx, userID, test.date, test.date, start, end)
			if err != nil {
				t.Fatal(err)
			}
			assertIntegrationTitles(t, "events", eventTitles(events), []string{"At start point", "Before end point", "Overlaps start"})
		})
	}
}

func taskTitles(items []domain.Task) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles
}

func billTitles(items []domain.Bill) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles
}

func eventTitles(items []domain.Event) []string {
	titles := make([]string, 0, len(items))
	for _, item := range items {
		titles = append(titles, item.Title)
	}
	return titles
}

func assertIntegrationTitles(t *testing.T, kind string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s titles = %v, want %v", kind, got, want)
	}
	wantSet := make(map[string]struct{}, len(want))
	for _, title := range want {
		wantSet[title] = struct{}{}
	}
	for _, title := range got {
		if _, ok := wantSet[title]; !ok {
			t.Fatalf("%s titles = %v, want %v", kind, got, want)
		}
	}
}

func mustUUID(t *testing.T) string {
	t.Helper()
	value, err := NewUUID()
	if err != nil {
		t.Fatal(err)
	}
	return value
}

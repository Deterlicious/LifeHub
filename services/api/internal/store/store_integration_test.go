package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/migrate"
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

	tasks, err := storage.ListTodayTasks(ctx, userID, start, end)
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
	tasks, err := storage.ListTodayTasks(ctx, userID, start, start.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != taskCount {
		t.Fatalf("Today returned %d anytime tasks, want %d", len(tasks), taskCount)
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

package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/migrate"
	"lifehub/services/api/internal/reminders"
	"lifehub/services/api/internal/riverinfra"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

type reminderPGFixture struct {
	ctx    context.Context
	store  *Store
	user   string
	other  string
	jobMu  sync.Mutex
	jobIDs []int64
	cancel context.CancelFunc
	dbURL  string
}

func newReminderPGFixture(t *testing.T) *reminderPGFixture {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := migrate.Up(ctx, databaseURL); err != nil {
		cancel()
		t.Fatal(err)
	}
	if err := riverinfra.Migrate(ctx, databaseURL); err != nil {
		cancel()
		t.Fatal(err)
	}
	storage, err := Open(ctx, databaseURL)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	fixture := &reminderPGFixture{ctx: ctx, store: storage, user: mustUUID(t), other: mustUUID(t), cancel: cancel, dbURL: databaseURL}
	if _, err := storage.UpdateProfileTimezone(ctx, fixture.user, "Asia/Jakarta"); err != nil {
		t.Fatal(err)
	}
	if _, err := storage.UpdateProfileTimezone(ctx, fixture.other, "Asia/Jakarta"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		rows, err := storage.pool.Query(cleanupCtx, `
			SELECT river_job_id FROM lifehub.reminder_schedules
			WHERE user_id = ANY($1::uuid[]) AND river_job_id IS NOT NULL
		`, []string{fixture.user, fixture.other})
		if err == nil {
			for rows.Next() {
				var id int64
				if rows.Scan(&id) == nil {
					fixture.trackJob(id)
				}
			}
			rows.Close()
		}
		_, _ = storage.pool.Exec(cleanupCtx, "DELETE FROM lifehub.profiles WHERE user_id = ANY($1::uuid[])", []string{fixture.user, fixture.other})
		fixture.jobMu.Lock()
		jobIDs := append([]int64(nil), fixture.jobIDs...)
		fixture.jobMu.Unlock()
		if len(jobIDs) > 0 {
			_, _ = storage.pool.Exec(cleanupCtx, "DELETE FROM river.river_job WHERE id = ANY($1::bigint[])", jobIDs)
		}
		storage.Close()
		cancel()
	})
	return fixture
}

func (fixture *reminderPGFixture) trackJob(jobID int64) {
	fixture.jobMu.Lock()
	defer fixture.jobMu.Unlock()
	fixture.jobIDs = append(fixture.jobIDs, jobID)
}

func (fixture *reminderPGFixture) latestSchedule(t *testing.T, reminderID string) (string, int64, int64, time.Time, string) {
	t.Helper()
	var scheduleID, state string
	var generation, jobID int64
	var fireAt time.Time
	if err := fixture.store.pool.QueryRow(fixture.ctx, `
		SELECT id::text, generation, river_job_id, fire_at, state
		FROM lifehub.reminder_schedules
		WHERE reminder_id = $1::uuid
		ORDER BY generation DESC LIMIT 1
	`, reminderID).Scan(&scheduleID, &generation, &jobID, &fireAt, &state); err != nil {
		t.Fatal(err)
	}
	fixture.trackJob(jobID)
	return scheduleID, generation, jobID, fireAt, state
}

func (fixture *reminderPGFixture) createMomentReminder(t *testing.T, sourceKind, sourceID string, minutes int) domain.Reminder {
	t.Helper()
	item, err := fixture.store.CreateReminder(fixture.ctx, domain.CreateReminderParams{
		ID: mustUUID(t), UserID: fixture.user, SourceKind: sourceKind, SourceID: sourceID,
		ScheduleKind: "before_moment", MinutesBefore: &minutes,
	})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func (fixture *reminderPGFixture) createDateReminder(t *testing.T, sourceKind, sourceID string, days int, local string) domain.Reminder {
	t.Helper()
	item, err := fixture.store.CreateReminder(fixture.ctx, domain.CreateReminderParams{
		ID: mustUUID(t), UserID: fixture.user, SourceKind: sourceKind, SourceID: sourceID,
		ScheduleKind: "before_date", DaysBefore: &days, TimeLocal: &local,
	})
	if err != nil {
		t.Fatal(err)
	}
	return item
}

func TestReminderPostgresAllSourceAdaptersOwnershipAndInvalidation(t *testing.T) {
	fixture := newReminderPGFixture(t)
	now := time.Now().UTC()
	taskDue, eventStart, billDue := now.Add(48*time.Hour), now.Add(50*time.Hour), now.Add(52*time.Hour)
	taskID, eventID, allDayID, billID, documentID := mustUUID(t), mustUUID(t), mustUUID(t), mustUUID(t), mustUUID(t)
	if _, err := fixture.store.CreateTask(fixture.ctx, domain.CreateTaskParams{ID: taskID, UserID: fixture.user, Title: "Tugas penting", Priority: domain.PriorityHigh, DueAt: &taskDue}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.CreateEvent(fixture.ctx, domain.CreateEventParams{ID: eventID, UserID: fixture.user, Title: "Jadwal", Timezone: "Asia/Jakarta", StartsAt: &eventStart}); err != nil {
		t.Fatal(err)
	}
	date := now.In(time.FixedZone("WIB", 7*60*60)).AddDate(0, 0, 10)
	dateOnly := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	if _, err := fixture.store.CreateEvent(fixture.ctx, domain.CreateEventParams{ID: allDayID, UserID: fixture.user, Title: "Seharian", Timezone: "Asia/Jakarta", AllDay: true, StartsOn: &dateOnly}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.CreateBill(fixture.ctx, domain.CreateBillParams{ID: billID, UserID: fixture.user, Title: "Tagihan", Amount: 1000, Currency: "IDR", DueAt: billDue}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.CreateDocument(fixture.ctx, domain.CreateDocumentParams{ID: documentID, UserID: fixture.user, Name: "Paspor", Category: "identity", ExpiresOn: dateOnly}); err != nil {
		t.Fatal(err)
	}

	taskReminder := fixture.createMomentReminder(t, "task", taskID, 30)
	eventReminder := fixture.createMomentReminder(t, "event", eventID, 15)
	allDayReminder := fixture.createDateReminder(t, "event", allDayID, 1, "09:00")
	billReminder := fixture.createMomentReminder(t, "bill", billID, 60)
	documentReminder := fixture.createDateReminder(t, "document", documentID, 2, "08:00")
	for _, item := range []domain.Reminder{taskReminder, eventReminder, allDayReminder, billReminder, documentReminder} {
		if item.Status != "scheduled" || item.NextFireAt == nil {
			t.Fatalf("reminder not scheduled: %#v", item)
		}
		_, _, jobID, _, _ := fixture.latestSchedule(t, item.ID)
		var encoded []byte
		if err := fixture.store.pool.QueryRow(fixture.ctx, "SELECT args FROM river.river_job WHERE id = $1", jobID).Scan(&encoded); err != nil {
			t.Fatal(err)
		}
		var args map[string]any
		if err := json.Unmarshal(encoded, &args); err != nil {
			t.Fatal(err)
		}
		if len(args) != 2 || args["schedule_id"] == nil || args["generation"] == nil {
			t.Fatalf("private data leaked into River args: %s", encoded)
		}
	}

	minutes := 1
	if _, err := fixture.store.CreateReminder(fixture.ctx, domain.CreateReminderParams{
		ID: mustUUID(t), UserID: fixture.user, SourceKind: "document", SourceID: documentID,
		ScheduleKind: "before_moment", MinutesBefore: &minutes,
	}); !errors.Is(err, ErrReminderScheduleMismatch) {
		t.Fatalf("document moment schedule error = %v", err)
	}
	if _, err := fixture.store.GetReminder(fixture.ctx, fixture.other, taskReminder.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner get error = %v", err)
	}
	if _, err := fixture.store.ListReminders(fixture.ctx, fixture.other, "task", taskID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner list error = %v", err)
	}
	if err := fixture.store.DeleteReminder(fixture.ctx, fixture.other, taskReminder.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner delete error = %v", err)
	}

	_, oldTaskGeneration, oldTaskJob, _, _ := fixture.latestSchedule(t, taskReminder.ID)
	newTaskDue := taskDue.Add(24 * time.Hour)
	if _, err := fixture.store.UpdateTask(fixture.ctx, domain.UpdateTaskParams{ID: taskID, UserID: fixture.user, DueAtSet: true, DueAt: &newTaskDue}); err != nil {
		t.Fatal(err)
	}
	_, newTaskGeneration, _, _, state := fixture.latestSchedule(t, taskReminder.ID)
	if newTaskGeneration != oldTaskGeneration+1 || state != "scheduled" {
		t.Fatalf("task generation=%d state=%s", newTaskGeneration, state)
	}
	assertRiverJobState(t, fixture, oldTaskJob, "cancelled")

	firstComplete, err := fixture.store.CompleteTask(fixture.ctx, fixture.user, taskID)
	if err != nil {
		t.Fatal(err)
	}
	secondComplete, err := fixture.store.CompleteTask(fixture.ctx, fixture.user, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if firstComplete.CompletedAt == nil || secondComplete.CompletedAt == nil || !firstComplete.CompletedAt.Equal(*secondComplete.CompletedAt) || !firstComplete.UpdatedAt.Equal(secondComplete.UpdatedAt) {
		t.Fatal("repeat completion changed idempotent timestamps")
	}
	var taskGenerations int
	if err := fixture.store.pool.QueryRow(fixture.ctx, "SELECT count(*) FROM lifehub.reminder_schedules WHERE reminder_id=$1::uuid", taskReminder.ID).Scan(&taskGenerations); err != nil {
		t.Fatal(err)
	}
	if taskGenerations != 2 {
		t.Fatalf("completion generated schedules: %d", taskGenerations)
	}
	firstUncomplete, err := fixture.store.UncompleteTask(fixture.ctx, fixture.user, taskID)
	if err != nil {
		t.Fatal(err)
	}
	secondUncomplete, err := fixture.store.UncompleteTask(fixture.ctx, fixture.user, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if !firstUncomplete.UpdatedAt.Equal(secondUncomplete.UpdatedAt) {
		t.Fatal("repeat uncomplete changed updated_at")
	}
	if err := fixture.store.pool.QueryRow(fixture.ctx, "SELECT count(*) FROM lifehub.reminder_schedules WHERE reminder_id=$1::uuid", taskReminder.ID).Scan(&taskGenerations); err != nil {
		t.Fatal(err)
	}
	if taskGenerations != 3 {
		t.Fatalf("repeat uncomplete generated schedules: %d", taskGenerations)
	}

	if _, err := fixture.store.UpdateEvent(fixture.ctx, domain.UpdateEventParams{ID: eventID, UserID: fixture.user, Title: stringPointer("Jadwal baru")}); err != nil {
		t.Fatal(err)
	}
	if _, generation, _, _, _ := fixture.latestSchedule(t, eventReminder.ID); generation != 1 {
		t.Fatalf("metadata-only event generation=%d", generation)
	}
	newEventStart := eventStart.Add(24 * time.Hour)
	if _, err := fixture.store.UpdateEvent(fixture.ctx, domain.UpdateEventParams{
		ID: eventID, UserID: fixture.user, ScheduleSet: true, Timezone: "Asia/Jakarta", StartsAt: &newEventStart,
	}); err != nil {
		t.Fatal(err)
	}
	if _, generation, _, _, _ := fixture.latestSchedule(t, eventReminder.ID); generation != 2 {
		t.Fatalf("event generation=%d", generation)
	}
	if _, err := fixture.store.UpdateDocument(fixture.ctx, domain.UpdateDocumentParams{ID: documentID, UserID: fixture.user, Name: stringPointer("Paspor baru")}); err != nil {
		t.Fatal(err)
	}
	if _, generation, _, _, _ := fixture.latestSchedule(t, documentReminder.ID); generation != 1 {
		t.Fatalf("metadata-only document generation=%d", generation)
	}
	newExpiry := dateOnly.AddDate(0, 0, 1)
	if _, err := fixture.store.UpdateDocument(fixture.ctx, domain.UpdateDocumentParams{ID: documentID, UserID: fixture.user, ExpiresOn: &newExpiry}); err != nil {
		t.Fatal(err)
	}
	if _, generation, _, _, _ := fixture.latestSchedule(t, documentReminder.ID); generation != 2 {
		t.Fatalf("document generation=%d", generation)
	}
	firstPaid, err := fixture.store.MarkBillPaid(fixture.ctx, fixture.user, billID)
	if err != nil {
		t.Fatal(err)
	}
	secondPaid, err := fixture.store.MarkBillPaid(fixture.ctx, fixture.user, billID)
	if err != nil {
		t.Fatal(err)
	}
	if !firstPaid.UpdatedAt.Equal(secondPaid.UpdatedAt) {
		t.Fatal("repeat mark-paid changed updated_at")
	}
	if _, err := fixture.store.MarkBillUnpaid(fixture.ctx, fixture.user, billID); err != nil {
		t.Fatal(err)
	}
	_, billGeneration, _, _, _ := fixture.latestSchedule(t, billReminder.ID)
	if _, err := fixture.store.MarkBillUnpaid(fixture.ctx, fixture.user, billID); err != nil {
		t.Fatal(err)
	}
	if _, repeatedGeneration, _, _, _ := fixture.latestSchedule(t, billReminder.ID); repeatedGeneration != billGeneration {
		t.Fatalf("repeat mark-unpaid generation=%d want=%d", repeatedGeneration, billGeneration)
	}
}

func TestDeleteUserDataCascadesPrivateRowsAndCancelsReminderJobs(t *testing.T) {
	fixture := newReminderPGFixture(t)
	due := time.Now().UTC().Add(48 * time.Hour)
	userTaskID := mustUUID(t)
	otherTaskID := mustUUID(t)
	if _, err := fixture.store.CreateTask(fixture.ctx, domain.CreateTaskParams{
		ID: userTaskID, UserID: fixture.user, Title: "Data pribadi", Priority: domain.PriorityNormal, DueAt: &due,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.CreateTask(fixture.ctx, domain.CreateTaskParams{
		ID: otherTaskID, UserID: fixture.other, Title: "Tetap ada", Priority: domain.PriorityNormal, DueAt: &due,
	}); err != nil {
		t.Fatal(err)
	}
	reminder := fixture.createMomentReminder(t, "task", userTaskID, 30)
	_, _, jobID, _, _ := fixture.latestSchedule(t, reminder.ID)

	if err := fixture.store.DeleteUserData(fixture.ctx, fixture.user); err != nil {
		t.Fatal(err)
	}
	assertRiverJobState(t, fixture, jobID, "cancelled")

	var profiles, tasks, definitions, schedules, notifications, series int
	if err := fixture.store.pool.QueryRow(fixture.ctx, `
		SELECT
			(SELECT count(*) FROM lifehub.profiles WHERE user_id = $1::uuid),
			(SELECT count(*) FROM lifehub.tasks WHERE user_id = $1::uuid),
			(SELECT count(*) FROM lifehub.reminder_definitions WHERE user_id = $1::uuid),
			(SELECT count(*) FROM lifehub.reminder_schedules WHERE user_id = $1::uuid),
			(SELECT count(*) FROM lifehub.notifications WHERE user_id = $1::uuid),
			(SELECT count(*) FROM lifehub.recurrence_series WHERE user_id = $1::uuid)
	`, fixture.user).Scan(&profiles, &tasks, &definitions, &schedules, &notifications, &series); err != nil {
		t.Fatal(err)
	}
	if profiles+tasks+definitions+schedules+notifications+series != 0 {
		t.Fatalf("deleted user rows remain: profiles=%d tasks=%d definitions=%d schedules=%d notifications=%d series=%d", profiles, tasks, definitions, schedules, notifications, series)
	}
	var otherTasks int
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT count(*) FROM lifehub.tasks WHERE user_id = $1::uuid`, fixture.other).Scan(&otherTasks); err != nil {
		t.Fatal(err)
	}
	if otherTasks != 1 {
		t.Fatalf("other user tasks = %d, want 1", otherTasks)
	}
}

func assertRiverJobState(t *testing.T, fixture *reminderPGFixture, jobID int64, want string) {
	t.Helper()
	var state string
	if err := fixture.store.pool.QueryRow(fixture.ctx, "SELECT state FROM river.river_job WHERE id=$1", jobID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if state != want {
		t.Fatalf("River job %d state=%s want=%s", jobID, state, want)
	}
}

func stringPointer(value string) *string { return &value }

type forcedRetryProcessor struct{}

func (forcedRetryProcessor) ProcessReminder(context.Context, string, int64) error {
	return errors.New("forced first-worker failure")
}

type fastReminderRetryPolicy struct {
	delay time.Duration
}

func (policy fastReminderRetryPolicy) NextRetry(*rivertype.JobRow) time.Time {
	return time.Now().UTC().Add(policy.delay)
}

func newReminderWorkerClient(
	t *testing.T,
	fixture *reminderPGFixture,
	processor reminders.Processor,
) *river.Client[pgx.Tx] {
	t.Helper()
	client, err := river.NewClient(riverpgxv5.New(fixture.store.pool), &river.Config{
		Schema: riverinfra.Schema,
		Queues: map[string]river.QueueConfig{
			reminders.QueueName: {MaxWorkers: 1},
		},
		Workers:           reminders.NewWorkers(processor),
		MaxAttempts:       reminders.MaxAttempts,
		JobTimeout:        reminders.WorkerTimeout,
		SoftStopTimeout:   time.Second,
		RetryPolicy:       fastReminderRetryPolicy{delay: 2 * time.Second},
		PollOnly:          true,
		FetchCooldown:     5 * time.Millisecond,
		FetchPollInterval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func waitForReminderJobEvent(t *testing.T, events <-chan *river.Event, kind river.EventKind, jobID int64) *river.Event {
	t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case event := <-events:
			if event != nil && event.Kind == kind && event.Job != nil && event.Job.ID == jobID {
				return event
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for River event %s for job %d", kind, jobID)
		}
	}
}

func stopReminderWorkerClient(t *testing.T, client *river.Client[pgx.Tx]) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-client.Stopped():
	case <-ctx.Done():
		t.Fatal("timed out waiting for River client to stop")
	}
}

func TestReminderPostgresExactlyOnceAcrossRiverRetryAndWorkerRestart(t *testing.T) {
	fixture := newReminderPGFixture(t)
	due := time.Now().UTC().Add(24 * time.Hour)
	taskID := mustUUID(t)
	if _, err := fixture.store.CreateTask(fixture.ctx, domain.CreateTaskParams{ID: taskID, UserID: fixture.user, Title: "Kirim laporan", Priority: domain.PriorityNormal, DueAt: &due}); err != nil {
		t.Fatal(err)
	}
	item := fixture.createMomentReminder(t, "task", taskID, 0)
	scheduleID, generation, jobID, _, _ := fixture.latestSchedule(t, item.ID)
	past := time.Now().UTC().Add(-time.Minute)
	if _, err := fixture.store.pool.Exec(fixture.ctx, "UPDATE lifehub.tasks SET due_at=$2 WHERE id=$1::uuid", taskID, past); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, "UPDATE lifehub.reminder_schedules SET fire_at=$2 WHERE id=$1::uuid", scheduleID, past); err != nil {
		t.Fatal(err)
	}
	updatedTitle := "Laporan terbaru"
	if _, err := fixture.store.UpdateTask(fixture.ctx, domain.UpdateTaskParams{ID: taskID, UserID: fixture.user, Title: &updatedTitle}); err != nil {
		t.Fatal(err)
	}
	_, unchangedGeneration, _, unchangedFire, unchangedState := fixture.latestSchedule(t, item.ID)
	fireDelta := unchangedFire.Sub(past)
	if unchangedGeneration != generation || fireDelta < -time.Microsecond || fireDelta > time.Microsecond || unchangedState != "scheduled" {
		t.Fatalf("metadata edit rotated due schedule: generation=%d fire=%s state=%s", unchangedGeneration, unchangedFire, unchangedState)
	}
	if _, err := fixture.store.pool.Exec(fixture.ctx, `
		UPDATE river.river_job
		SET state = 'available', scheduled_at = now() - interval '1 second', finalized_at = NULL
		WHERE id = $1
	`, jobID); err != nil {
		t.Fatal(err)
	}

	firstClient := newReminderWorkerClient(t, fixture, forcedRetryProcessor{})
	failedEvents, cancelFailed := firstClient.Subscribe(river.EventKindJobFailed)
	defer cancelFailed()
	if err := firstClient.Start(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	failed := waitForReminderJobEvent(t, failedEvents, river.EventKindJobFailed, jobID)
	if failed.Job.Attempt != 1 {
		t.Fatalf("first River attempt=%d", failed.Job.Attempt)
	}
	stopReminderWorkerClient(t, firstClient)
	var firstAttempt int
	var firstState rivertype.JobState
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT attempt, state FROM river.river_job WHERE id = $1`, jobID).Scan(&firstAttempt, &firstState); err != nil {
		t.Fatal(err)
	}
	if firstAttempt != 1 || (firstState != rivertype.JobStateRetryable && firstState != rivertype.JobStateAvailable) {
		t.Fatalf("persisted first River attempt=%d state=%s", firstAttempt, firstState)
	}

	secondClient := newReminderWorkerClient(t, fixture, fixture.store)
	completedEvents, cancelCompleted := secondClient.Subscribe(river.EventKindJobCompleted)
	defer cancelCompleted()
	if err := secondClient.Start(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	completed := waitForReminderJobEvent(t, completedEvents, river.EventKindJobCompleted, jobID)
	if completed.Job.Attempt != 2 {
		t.Fatalf("restarted River attempt=%d", completed.Job.Attempt)
	}
	stopReminderWorkerClient(t, secondClient)
	var completedAttempt int
	var completedState rivertype.JobState
	if err := fixture.store.pool.QueryRow(fixture.ctx, `SELECT attempt, state FROM river.river_job WHERE id = $1`, jobID).Scan(&completedAttempt, &completedState); err != nil {
		t.Fatal(err)
	}
	if completedAttempt != 2 || completedState != rivertype.JobStateCompleted {
		t.Fatalf("persisted restarted River attempt=%d state=%s", completedAttempt, completedState)
	}

	var count int
	if err := fixture.store.pool.QueryRow(fixture.ctx, "SELECT count(*) FROM lifehub.notifications WHERE schedule_id=$1::uuid AND generation=$2", scheduleID, generation).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("notifications=%d want=1", count)
	}
	items, unread, err := fixture.store.ListNotifications(fixture.ctx, fixture.user, 10, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || unread != 1 || items[0].Body != updatedTitle {
		t.Fatalf("notifications=%#v unread=%d", items, unread)
	}
	if _, _, err := fixture.store.MarkNotificationRead(fixture.ctx, fixture.other, items[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-owner mark read error=%v", err)
	}
	first, unread, err := fixture.store.MarkNotificationRead(fixture.ctx, fixture.user, items[0].ID)
	if err != nil || unread != 0 || first.ReadAt == nil {
		t.Fatalf("first read=%#v unread=%d err=%v", first, unread, err)
	}
	second, unread, err := fixture.store.MarkNotificationRead(fixture.ctx, fixture.user, items[0].ID)
	if err != nil || unread != 0 || second.ReadAt == nil || !second.ReadAt.Equal(*first.ReadAt) {
		t.Fatalf("repeat read=%#v unread=%d err=%v", second, unread, err)
	}
	if marked, err := fixture.store.MarkAllNotificationsRead(fixture.ctx, fixture.user); err != nil || marked != 0 {
		t.Fatalf("mark all=%d err=%v", marked, err)
	}
}

func TestReminderPostgresTimezoneChangeAndReadPathTerminalReconciliation(t *testing.T) {
	fixture := newReminderPGFixture(t)
	localNow := time.Now().In(time.FixedZone("WIB", 7*60*60))
	expires := localNow.AddDate(0, 0, 20)
	expiresOn := time.Date(expires.Year(), expires.Month(), expires.Day(), 0, 0, 0, 0, time.UTC)
	documentID := mustUUID(t)
	if _, err := fixture.store.CreateDocument(fixture.ctx, domain.CreateDocumentParams{ID: documentID, UserID: fixture.user, Name: "Visa", Category: "license", ExpiresOn: expiresOn}); err != nil {
		t.Fatal(err)
	}
	item := fixture.createDateReminder(t, "document", documentID, 1, "09:00")
	_, generation1, job1, fire1, _ := fixture.latestSchedule(t, item.ID)
	if _, err := fixture.store.UpdateProfileTimezone(fixture.ctx, fixture.user, "Asia/Tokyo"); err != nil {
		t.Fatal(err)
	}
	_, generation2, job2, fire2, state := fixture.latestSchedule(t, item.ID)
	if generation2 != generation1+1 || state != "scheduled" || fire1.Sub(fire2) != 2*time.Hour {
		t.Fatalf("timezone reschedule gen=%d fire1=%s fire2=%s state=%s", generation2, fire1, fire2, state)
	}
	assertRiverJobState(t, fixture, job1, "cancelled")
	if _, err := fixture.store.pool.Exec(fixture.ctx, `
		UPDATE river.river_job
		SET state='discarded', finalized_at=now()
		WHERE id=$1
	`, job2); err != nil {
		t.Fatal(err)
	}
	got, err := fixture.store.GetReminder(fixture.ctx, fixture.user, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "inactive" || got.NextFireAt != nil {
		t.Fatalf("discarded reminder remains scheduled: %#v", got)
	}
}

func TestReminderPostgresCreateSerializesWithSourceDelete(t *testing.T) {
	fixture := newReminderPGFixture(t)
	due := time.Now().UTC().Add(24 * time.Hour)
	taskID := mustUUID(t)
	if _, err := fixture.store.CreateTask(fixture.ctx, domain.CreateTaskParams{ID: taskID, UserID: fixture.user, Title: "Race", Priority: domain.PriorityNormal, DueAt: &due}); err != nil {
		t.Fatal(err)
	}
	tx, err := fixture.store.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(fixture.ctx, "DELETE FROM lifehub.tasks WHERE id=$1::uuid AND user_id=$2::uuid", taskID, fixture.user); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		minutes := 0
		_, createErr := fixture.store.CreateReminder(fixture.ctx, domain.CreateReminderParams{
			ID: mustUUID(t), UserID: fixture.user, SourceKind: "task", SourceID: taskID,
			ScheduleKind: "before_moment", MinutesBefore: &minutes,
		})
		result <- createErr
	}()
	select {
	case err := <-result:
		t.Fatalf("create returned before delete transaction resolved: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit(fixture.ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("create after delete error=%v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent create did not unblock")
	}
	var definitions int
	if err := fixture.store.pool.QueryRow(fixture.ctx, "SELECT count(*) FROM lifehub.reminder_definitions WHERE source_kind='task' AND source_id=$1::uuid", taskID).Scan(&definitions); err != nil {
		t.Fatal(err)
	}
	if definitions != 0 {
		t.Fatalf("orphan definitions=%d", definitions)
	}
}

var _ reminders.Processor = (*Store)(nil)

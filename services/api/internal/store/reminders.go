package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/reminders"
	"lifehub/services/api/internal/timeutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/riverqueue/river"
)

var (
	ErrReminderSourceUnscheduled = errors.New("reminder source cannot be scheduled")
	ErrReminderScheduleMismatch  = errors.New("reminder schedule does not match source")
	ErrReminderInvalidLocalTime  = errors.New("reminder local time is invalid")
)

type reminderDefinition struct {
	ID            string
	UserID        string
	SourceKind    string
	SourceID      string
	ScheduleKind  string
	MinutesBefore *int
	DaysBefore    *int
	TimeLocal     *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type reminderSource struct {
	Kind     string
	Title    string
	Timezone string
	Moment   *time.Time
	Date     *time.Time
	Active   bool
}

func (s *Store) CreateReminder(ctx context.Context, params domain.CreateReminderParams) (domain.Reminder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("begin create reminder: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	source, err := s.loadReminderSourceForScheduleTx(ctx, tx, params.UserID, params.SourceKind, params.SourceID)
	if err != nil {
		return domain.Reminder{}, err
	}
	definition := reminderDefinition{
		ID: params.ID, UserID: params.UserID, SourceKind: params.SourceKind, SourceID: params.SourceID,
		ScheduleKind: params.ScheduleKind, MinutesBefore: params.MinutesBefore, DaysBefore: params.DaysBefore, TimeLocal: params.TimeLocal,
	}
	fireAt, err := calculateReminderFireAt(definition, source)
	if err != nil {
		return domain.Reminder{}, err
	}
	if err := ensureFutureFireAt(ctx, tx, fireAt); err != nil {
		return domain.Reminder{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lifehub.reminder_definitions (
			id, user_id, source_kind, source_id, schedule_kind, minutes_before, days_before, time_local
		)
		VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6, $7, $8::time)
	`, params.ID, params.UserID, params.SourceKind, params.SourceID, params.ScheduleKind, params.MinutesBefore, params.DaysBefore, params.TimeLocal); err != nil {
		return domain.Reminder{}, fmt.Errorf("insert reminder definition: %w", err)
	}
	if err := s.insertReminderScheduleTx(ctx, tx, definition, 1, fireAt); err != nil {
		return domain.Reminder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Reminder{}, fmt.Errorf("commit create reminder: %w", err)
	}
	return s.GetReminder(ctx, params.UserID, params.ID)
}

func (s *Store) GetReminder(ctx context.Context, userID, reminderID string) (domain.Reminder, error) {
	if err := s.reconcileTerminalReminderSchedulesForUser(ctx, userID); err != nil {
		return domain.Reminder{}, err
	}
	reminder, err := scanReminder(s.pool.QueryRow(ctx, reminderSelectSQL+`
		WHERE definition.id = $1::uuid AND definition.user_id = $2::uuid
	`, reminderID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Reminder{}, ErrNotFound
	}
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("get reminder: %w", err)
	}
	return reminder, nil
}

func (s *Store) ListReminders(ctx context.Context, userID, sourceKind, sourceID string) ([]domain.Reminder, error) {
	if _, err := s.loadReminderSource(ctx, userID, sourceKind, sourceID); err != nil {
		return nil, err
	}
	if err := s.reconcileTerminalReminderSchedulesForUser(ctx, userID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, reminderSelectSQL+`
		WHERE definition.user_id = $1::uuid
		  AND definition.source_kind = $2
		  AND definition.source_id = $3::uuid
		ORDER BY definition.created_at, definition.id
	`, userID, sourceKind, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Reminder, 0)
	for rows.Next() {
		item, err := scanReminder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan reminder: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate reminders: %w", err)
	}
	return items, nil
}

func (s *Store) UpdateReminder(ctx context.Context, params domain.UpdateReminderParams) (domain.Reminder, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Reminder{}, fmt.Errorf("begin update reminder: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	definition, err := loadReminderDefinitionTx(ctx, tx, params.UserID, params.ID, false)
	if err != nil {
		return domain.Reminder{}, err
	}
	source, err := s.loadReminderSourceForScheduleTx(ctx, tx, params.UserID, definition.SourceKind, definition.SourceID)
	if err != nil {
		return domain.Reminder{}, err
	}
	definition, err = loadReminderDefinitionTx(ctx, tx, params.UserID, params.ID, true)
	if err != nil {
		return domain.Reminder{}, err
	}
	definition.ScheduleKind = params.ScheduleKind
	definition.MinutesBefore = params.MinutesBefore
	definition.DaysBefore = params.DaysBefore
	definition.TimeLocal = params.TimeLocal
	fireAt, err := calculateReminderFireAt(definition, source)
	if err != nil {
		return domain.Reminder{}, err
	}
	if err := ensureFutureFireAt(ctx, tx, fireAt); err != nil {
		return domain.Reminder{}, err
	}
	if err := s.invalidateActiveReminderScheduleTx(ctx, tx, definition.ID); err != nil {
		return domain.Reminder{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lifehub.reminder_definitions
		SET schedule_kind = $3,
		    minutes_before = $4,
		    days_before = $5,
		    time_local = $6::time,
		    updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, params.ID, params.UserID, params.ScheduleKind, params.MinutesBefore, params.DaysBefore, params.TimeLocal); err != nil {
		return domain.Reminder{}, fmt.Errorf("update reminder definition: %w", err)
	}
	generation, err := nextReminderGenerationTx(ctx, tx, definition.ID)
	if err != nil {
		return domain.Reminder{}, err
	}
	if err := s.insertReminderScheduleTx(ctx, tx, definition, generation, fireAt); err != nil {
		return domain.Reminder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Reminder{}, fmt.Errorf("commit update reminder: %w", err)
	}
	return s.GetReminder(ctx, params.UserID, params.ID)
}

func (s *Store) DeleteReminder(ctx context.Context, userID, reminderID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete reminder: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := loadReminderDefinitionTx(ctx, tx, userID, reminderID, true); err != nil {
		return err
	}
	if err := s.invalidateActiveReminderScheduleTx(ctx, tx, reminderID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM lifehub.reminder_definitions WHERE id = $1::uuid AND user_id = $2::uuid`, reminderID, userID); err != nil {
		return fmt.Errorf("delete reminder definition: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete reminder: %w", err)
	}
	return nil
}

func (s *Store) ProcessReminder(ctx context.Context, scheduleID string, generation int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin reminder delivery: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var reminderID string
	if err := tx.QueryRow(ctx, `
		SELECT reminder_id::text
		FROM lifehub.reminder_schedules
		WHERE id = $1::uuid AND generation = $2
	`, scheduleID, generation).Scan(&reminderID); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("find reminder schedule: %w", err)
	}
	definition, err := loadReminderDefinitionByIDTx(ctx, tx, reminderID, false)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	source, err := s.loadReminderSourceForScheduleTx(ctx, tx, definition.UserID, definition.SourceKind, definition.SourceID)
	if errors.Is(err, ErrNotFound) {
		// A concurrent source delete commits before this share lock is acquired.
		definition, definitionErr := loadReminderDefinitionByIDTx(ctx, tx, reminderID, true)
		if errors.Is(definitionErr, ErrNotFound) {
			return nil
		}
		if definitionErr != nil {
			return definitionErr
		}
		_ = definition
		if err := invalidateRunningScheduleTx(ctx, tx, scheduleID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if err != nil {
		return err
	}
	definition, err = loadReminderDefinitionByIDTx(ctx, tx, reminderID, true)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var (
		state        string
		scheduledFor time.Time
	)
	if err := tx.QueryRow(ctx, `
		SELECT state, fire_at
		FROM lifehub.reminder_schedules
		WHERE id = $1::uuid AND reminder_id = $2::uuid AND generation = $3
		FOR UPDATE
	`, scheduleID, reminderID, generation).Scan(&state, &scheduledFor); errors.Is(err, pgx.ErrNoRows) {
		return nil
	} else if err != nil {
		return fmt.Errorf("lock reminder schedule: %w", err)
	}
	if state != "scheduled" {
		return nil
	}
	fireAt, err := calculateReminderFireAt(definition, source)
	if err != nil || !fireAt.Equal(scheduledFor) {
		if err := invalidateRunningScheduleTx(ctx, tx, scheduleID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	var due bool
	if err := tx.QueryRow(ctx, `SELECT $1::timestamptz <= now()`, scheduledFor).Scan(&due); err != nil {
		return fmt.Errorf("check reminder fire time: %w", err)
	}
	if !due {
		return fmt.Errorf("reminder job ran before its immutable fire time")
	}

	notificationID, err := NewUUID()
	if err != nil {
		return err
	}
	title := reminderNotificationTitle(definition.SourceKind)
	if _, err := tx.Exec(ctx, `
		INSERT INTO lifehub.notifications (
			id, schedule_id, generation, user_id, source_kind, source_id, title, body
		)
		VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, $6::uuid, $7, $8)
		ON CONFLICT (schedule_id, generation) DO NOTHING
	`, notificationID, scheduleID, generation, definition.UserID, definition.SourceKind, definition.SourceID, title, source.Title); err != nil {
		return fmt.Errorf("insert reminder notification: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lifehub.reminder_schedules
		SET state = 'delivered', delivered_at = now()
		WHERE id = $1::uuid AND state = 'scheduled'
	`, scheduleID); err != nil {
		return fmt.Errorf("mark reminder delivered: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit reminder delivery: %w", err)
	}
	return nil
}

func (s *Store) ListNotifications(ctx context.Context, userID string, limit int, afterAt *time.Time, afterID *string) ([]domain.Notification, int, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, source_kind, source_id::text, title, body, created_at, read_at
		FROM lifehub.notifications
		WHERE user_id = $1::uuid
		  AND (
		    $2::timestamptz IS NULL
		    OR created_at < $2
		    OR (created_at = $2 AND id < $3::uuid)
		  )
		ORDER BY created_at DESC, id DESC
		LIMIT $4
	`, userID, afterAt, afterID, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list notifications: %w", err)
	}
	defer rows.Close()
	items := make([]domain.Notification, 0)
	for rows.Next() {
		item, err := scanNotification(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan notification: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate notifications: %w", err)
	}
	unread, err := s.NotificationUnreadCount(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	return items, unread, nil
}

func (s *Store) NotificationUnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	if err := s.pool.QueryRow(ctx, `
		SELECT count(*)::integer
		FROM lifehub.notifications
		WHERE user_id = $1::uuid AND read_at IS NULL
	`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func (s *Store) MarkNotificationRead(ctx context.Context, userID, notificationID string) (domain.Notification, int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Notification{}, 0, fmt.Errorf("begin mark notification read: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	item, err := scanNotification(tx.QueryRow(ctx, `
		UPDATE lifehub.notifications
		SET read_at = COALESCE(read_at, now())
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING id::text, source_kind, source_id::text, title, body, created_at, read_at
	`, notificationID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Notification{}, 0, ErrNotFound
	}
	if err != nil {
		return domain.Notification{}, 0, fmt.Errorf("mark notification read: %w", err)
	}
	var unread int
	if err := tx.QueryRow(ctx, `SELECT count(*)::integer FROM lifehub.notifications WHERE user_id = $1::uuid AND read_at IS NULL`, userID).Scan(&unread); err != nil {
		return domain.Notification{}, 0, fmt.Errorf("count unread notifications: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Notification{}, 0, fmt.Errorf("commit mark notification read: %w", err)
	}
	return item, unread, nil
}

func (s *Store) MarkAllNotificationsRead(ctx context.Context, userID string) (int, error) {
	result, err := s.pool.Exec(ctx, `
		UPDATE lifehub.notifications
		SET read_at = now()
		WHERE user_id = $1::uuid AND read_at IS NULL
	`, userID)
	if err != nil {
		return 0, fmt.Errorf("mark all notifications read: %w", err)
	}
	return int(result.RowsAffected()), nil
}

func (s *Store) InvalidateReminderJob(ctx context.Context, riverJobID int64) error {
	if _, err := s.pool.Exec(ctx, `
		UPDATE lifehub.reminder_schedules
		SET state = 'invalidated', invalidated_at = now()
		WHERE river_job_id = $1 AND state = 'scheduled'
	`, riverJobID); err != nil {
		return fmt.Errorf("invalidate exhausted reminder job: %w", err)
	}
	return nil
}

// ReconcileTerminalReminderSchedules closes the narrow crash window where a
// River job reaches discarded state while the terminal error callback cannot
// update LifeHub (for example, during a database restart).
func (s *Store) ReconcileTerminalReminderSchedules(ctx context.Context) error {
	if _, err := s.pool.Exec(ctx, `
		UPDATE lifehub.reminder_schedules AS schedule
		SET state = 'invalidated', invalidated_at = now()
		FROM river.river_job AS job
		WHERE schedule.river_job_id = job.id
		  AND schedule.state = 'scheduled'
		  AND job.state = 'discarded'
	`); err != nil {
		return fmt.Errorf("reconcile terminal reminder jobs: %w", err)
	}
	return nil
}

func (s *Store) reconcileTerminalReminderSchedulesForUser(ctx context.Context, userID string) error {
	if _, err := s.pool.Exec(ctx, `
		UPDATE lifehub.reminder_schedules AS schedule
		SET state = 'invalidated', invalidated_at = now()
		FROM river.river_job AS job
		WHERE schedule.river_job_id = job.id
		  AND schedule.user_id = $1::uuid
		  AND schedule.state = 'scheduled'
		  AND job.state = 'discarded'
	`, userID); err != nil {
		return fmt.Errorf("reconcile owned terminal reminder jobs: %w", err)
	}
	return nil
}

const reminderSelectSQL = `
	SELECT definition.id::text,
	       definition.source_kind,
	       definition.source_id::text,
	       definition.schedule_kind,
	       definition.minutes_before,
	       definition.days_before,
	       to_char(definition.time_local, 'HH24:MI'),
	       latest.state,
	       CASE WHEN latest.state = 'scheduled' THEN latest.fire_at END,
	       definition.created_at,
	       definition.updated_at
	FROM lifehub.reminder_definitions AS definition
	LEFT JOIN LATERAL (
		SELECT state, fire_at
		FROM lifehub.reminder_schedules
		WHERE reminder_id = definition.id
		ORDER BY generation DESC
		LIMIT 1
	) AS latest ON true
`

func scanReminder(row scanner) (domain.Reminder, error) {
	var (
		item         domain.Reminder
		minutes      pgtype.Int4
		days         pgtype.Int4
		timeLocal    pgtype.Text
		latestState  pgtype.Text
		nextFireAt   pgtype.Timestamptz
		scheduleKind string
	)
	if err := row.Scan(
		&item.ID, &item.SourceKind, &item.SourceID, &scheduleKind,
		&minutes, &days, &timeLocal, &latestState, &nextFireAt,
		&item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return domain.Reminder{}, err
	}
	item.Schedule.Kind = scheduleKind
	if minutes.Valid {
		value := int(minutes.Int32)
		item.Schedule.MinutesBefore = &value
	}
	if days.Valid {
		value := int(days.Int32)
		item.Schedule.DaysBefore = &value
	}
	if timeLocal.Valid {
		item.Schedule.TimeLocal = &timeLocal.String
	}
	item.Status = "inactive"
	if latestState.Valid {
		switch latestState.String {
		case "scheduled":
			item.Status = "scheduled"
		case "delivered":
			item.Status = "delivered"
		}
	}
	if nextFireAt.Valid {
		value := nextFireAt.Time
		item.NextFireAt = &value
	}
	return item, nil
}

func scanNotification(row scanner) (domain.Notification, error) {
	var (
		item   domain.Notification
		readAt pgtype.Timestamptz
	)
	if err := row.Scan(&item.ID, &item.SourceKind, &item.SourceID, &item.Title, &item.Body, &item.CreatedAt, &readAt); err != nil {
		return domain.Notification{}, err
	}
	if readAt.Valid {
		value := readAt.Time
		item.ReadAt = &value
	}
	return item, nil
}

func loadReminderDefinitionTx(ctx context.Context, tx pgx.Tx, userID, reminderID string, lock bool) (reminderDefinition, error) {
	query := `
		SELECT id::text, user_id::text, source_kind, source_id::text, schedule_kind,
		       minutes_before, days_before, to_char(time_local, 'HH24:MI'), created_at, updated_at
		FROM lifehub.reminder_definitions
		WHERE id = $1::uuid AND user_id = $2::uuid
	`
	if lock {
		query += " FOR UPDATE"
	}
	return scanReminderDefinition(tx.QueryRow(ctx, query, reminderID, userID))
}

func loadReminderDefinitionByIDTx(ctx context.Context, tx pgx.Tx, reminderID string, lock bool) (reminderDefinition, error) {
	query := `
		SELECT id::text, user_id::text, source_kind, source_id::text, schedule_kind,
		       minutes_before, days_before, to_char(time_local, 'HH24:MI'), created_at, updated_at
		FROM lifehub.reminder_definitions
		WHERE id = $1::uuid
	`
	if lock {
		query += " FOR UPDATE"
	}
	return scanReminderDefinition(tx.QueryRow(ctx, query, reminderID))
}

func scanReminderDefinition(row scanner) (reminderDefinition, error) {
	var (
		definition reminderDefinition
		minutes    pgtype.Int4
		days       pgtype.Int4
		timeLocal  pgtype.Text
	)
	err := row.Scan(
		&definition.ID, &definition.UserID, &definition.SourceKind, &definition.SourceID,
		&definition.ScheduleKind, &minutes, &days, &timeLocal, &definition.CreatedAt, &definition.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return reminderDefinition{}, ErrNotFound
	}
	if err != nil {
		return reminderDefinition{}, fmt.Errorf("load reminder definition: %w", err)
	}
	if minutes.Valid {
		value := int(minutes.Int32)
		definition.MinutesBefore = &value
	}
	if days.Valid {
		value := int(days.Int32)
		definition.DaysBefore = &value
	}
	if timeLocal.Valid {
		definition.TimeLocal = &timeLocal.String
	}
	return definition, nil
}

func (s *Store) loadReminderSource(ctx context.Context, userID, sourceKind, sourceID string) (reminderSource, error) {
	return s.loadReminderSourceRow(ctx, s.pool, userID, sourceKind, sourceID, false)
}

func (s *Store) loadReminderSourceTx(ctx context.Context, tx pgx.Tx, userID, sourceKind, sourceID string) (reminderSource, error) {
	return s.loadReminderSourceRow(ctx, tx, userID, sourceKind, sourceID, false)

}

func (s *Store) loadReminderSourceForScheduleTx(ctx context.Context, tx pgx.Tx, userID, sourceKind, sourceID string) (reminderSource, error) {
	return s.loadReminderSourceRow(ctx, tx, userID, sourceKind, sourceID, true)
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (s *Store) loadReminderSourceRow(ctx context.Context, db queryRower, userID, sourceKind, sourceID string, lock bool) (reminderSource, error) {
	var source reminderSource
	source.Kind = sourceKind
	switch sourceKind {
	case "task":
		var moment pgtype.Timestamptz
		query := `
			SELECT task.title, task.due_at, task.completed_at IS NULL, COALESCE(profile.timezone, '')
			FROM lifehub.tasks AS task
			JOIN lifehub.profiles AS profile ON profile.user_id = task.user_id
			WHERE task.id = $1::uuid AND task.user_id = $2::uuid AND task.excluded_at IS NULL
		`
		if lock {
			query += " FOR SHARE OF task, profile"
		}
		err := db.QueryRow(ctx, query, sourceID, userID).Scan(&source.Title, &moment, &source.Active, &source.Timezone)
		if err := mapReminderSourceError(err); err != nil {
			return reminderSource{}, err
		}
		if moment.Valid {
			value := moment.Time
			source.Moment = &value
		}
	case "event":
		var (
			allDay bool
			moment pgtype.Timestamptz
			date   pgtype.Date
		)
		query := `
			SELECT event.title, event.all_day, event.starts_at, event.starts_on, COALESCE(profile.timezone, '')
			FROM lifehub.events AS event
			JOIN lifehub.profiles AS profile ON profile.user_id = event.user_id
			WHERE event.id = $1::uuid AND event.user_id = $2::uuid AND event.excluded_at IS NULL
		`
		if lock {
			query += " FOR SHARE OF event, profile"
		}
		err := db.QueryRow(ctx, query, sourceID, userID).Scan(&source.Title, &allDay, &moment, &date, &source.Timezone)
		if err := mapReminderSourceError(err); err != nil {
			return reminderSource{}, err
		}
		source.Active = true
		if allDay && date.Valid {
			value := date.Time
			source.Date = &value
		} else if moment.Valid {
			value := moment.Time
			source.Moment = &value
		}
	case "bill":
		var moment time.Time
		query := `
			SELECT bill.title, bill.due_at, bill.paid_at IS NULL, COALESCE(profile.timezone, '')
			FROM lifehub.bills AS bill
			JOIN lifehub.profiles AS profile ON profile.user_id = bill.user_id
			WHERE bill.id = $1::uuid AND bill.user_id = $2::uuid AND bill.excluded_at IS NULL
		`
		if lock {
			query += " FOR SHARE OF bill, profile"
		}
		err := db.QueryRow(ctx, query, sourceID, userID).Scan(&source.Title, &moment, &source.Active, &source.Timezone)
		if err := mapReminderSourceError(err); err != nil {
			return reminderSource{}, err
		}
		source.Moment = &moment
	case "document":
		var date time.Time
		query := `
			SELECT document.name, document.expires_on, COALESCE(profile.timezone, '')
			FROM lifehub.documents AS document
			JOIN lifehub.profiles AS profile ON profile.user_id = document.user_id
			WHERE document.id = $1::uuid AND document.user_id = $2::uuid
		`
		if lock {
			query += " FOR SHARE OF document, profile"
		}
		err := db.QueryRow(ctx, query, sourceID, userID).Scan(&source.Title, &date, &source.Timezone)
		if err := mapReminderSourceError(err); err != nil {
			return reminderSource{}, err
		}
		source.Active = true
		source.Date = &date
	default:
		return reminderSource{}, ErrNotFound
	}
	return source, nil
}

func mapReminderSourceError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load reminder source: %w", err)
	}
	return nil
}

func calculateReminderFireAt(definition reminderDefinition, source reminderSource) (time.Time, error) {
	if !source.Active {
		return time.Time{}, ErrReminderSourceUnscheduled
	}
	if source.Moment != nil {
		if definition.ScheduleKind != "before_moment" || definition.MinutesBefore == nil {
			return time.Time{}, ErrReminderScheduleMismatch
		}
		return source.Moment.Add(-time.Duration(*definition.MinutesBefore) * time.Minute).UTC(), nil
	}
	if source.Date != nil {
		if definition.ScheduleKind != "before_date" || definition.DaysBefore == nil || definition.TimeLocal == nil || source.Timezone == "" {
			return time.Time{}, ErrReminderScheduleMismatch
		}
		location, err := timeutil.LoadLocation(source.Timezone)
		if err != nil {
			return time.Time{}, ErrReminderInvalidLocalTime
		}
		date := source.Date.AddDate(0, 0, -*definition.DaysBefore).Format(time.DateOnly)
		fireAt, err := timeutil.ParseLocalWallTime(date+"T"+*definition.TimeLocal, location)
		if err != nil {
			return time.Time{}, ErrReminderInvalidLocalTime
		}
		return fireAt, nil
	}
	return time.Time{}, ErrReminderSourceUnscheduled
}

func ensureFutureFireAt(ctx context.Context, tx pgx.Tx, fireAt time.Time) error {
	future, err := isFutureFireAt(ctx, tx, fireAt)
	if err != nil {
		return err
	}
	if !future {
		return ErrReminderSourceUnscheduled
	}
	return nil
}

func isFutureFireAt(ctx context.Context, tx pgx.Tx, fireAt time.Time) (bool, error) {
	var future bool
	if err := tx.QueryRow(ctx, `SELECT $1::timestamptz > now()`, fireAt).Scan(&future); err != nil {
		return false, fmt.Errorf("validate reminder fire time: %w", err)
	}
	return future, nil
}

// refreshSourceRemindersTx makes a source mutation and every reminder schedule
// generation one atomic PostgreSQL operation. The immutable old generation is
// invalidated before a replacement is enqueued. Definitions that no longer
// match the source shape, are resolved, or would fire in the past remain
// visible as inactive and can be reconfigured by the user.
func (s *Store) refreshSourceRemindersTx(ctx context.Context, tx pgx.Tx, userID, sourceKind, sourceID string, deleteDefinitions bool) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, user_id::text, source_kind, source_id::text, schedule_kind,
		       minutes_before, days_before, to_char(time_local, 'HH24:MI'), created_at, updated_at
		FROM lifehub.reminder_definitions
		WHERE user_id = $1::uuid AND source_kind = $2 AND source_id = $3::uuid
		ORDER BY id
		FOR UPDATE
	`, userID, sourceKind, sourceID)
	if err != nil {
		return fmt.Errorf("lock source reminder definitions: %w", err)
	}
	definitions := make([]reminderDefinition, 0)
	for rows.Next() {
		definition, scanErr := scanReminderDefinition(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate source reminder definitions: %w", err)
	}
	rows.Close()
	if len(definitions) == 0 {
		return nil
	}

	for _, definition := range definitions {
		if err := s.invalidateActiveReminderScheduleTx(ctx, tx, definition.ID); err != nil {
			return err
		}
	}
	if deleteDefinitions {
		if _, err := tx.Exec(ctx, `
			DELETE FROM lifehub.reminder_definitions
			WHERE user_id = $1::uuid AND source_kind = $2 AND source_id = $3::uuid
		`, userID, sourceKind, sourceID); err != nil {
			return fmt.Errorf("delete source reminder definitions: %w", err)
		}
		return nil
	}

	source, err := s.loadReminderSourceTx(ctx, tx, userID, sourceKind, sourceID)
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		fireAt, err := calculateReminderFireAt(definition, source)
		if errors.Is(err, ErrReminderSourceUnscheduled) || errors.Is(err, ErrReminderScheduleMismatch) || errors.Is(err, ErrReminderInvalidLocalTime) {
			continue
		}
		if err != nil {
			return err
		}
		future, err := isFutureFireAt(ctx, tx, fireAt)
		if err != nil {
			return err
		}
		if !future {
			continue
		}
		generation, err := nextReminderGenerationTx(ctx, tx, definition.ID)
		if err != nil {
			return err
		}
		if err := s.insertReminderScheduleTx(ctx, tx, definition, generation, fireAt); err != nil {
			return err
		}
	}
	return nil
}

// refreshUserDateRemindersTx is called while the owned profile row is locked
// by a timezone change. This guarantees before_date schedules are never left
// pointing at an instant derived from the previous IANA timezone.
func (s *Store) refreshUserDateRemindersTx(ctx context.Context, tx pgx.Tx, userID string) error {
	rows, err := tx.Query(ctx, `
		SELECT id::text, user_id::text, source_kind, source_id::text, schedule_kind,
		       minutes_before, days_before, to_char(time_local, 'HH24:MI'), created_at, updated_at
		FROM lifehub.reminder_definitions
		WHERE user_id = $1::uuid AND schedule_kind = 'before_date'
		ORDER BY id
		FOR UPDATE
	`, userID)
	if err != nil {
		return fmt.Errorf("lock profile date reminder definitions: %w", err)
	}
	definitions := make([]reminderDefinition, 0)
	for rows.Next() {
		definition, scanErr := scanReminderDefinition(rows)
		if scanErr != nil {
			rows.Close()
			return scanErr
		}
		definitions = append(definitions, definition)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate profile date reminder definitions: %w", err)
	}
	rows.Close()

	for _, definition := range definitions {
		if err := s.invalidateActiveReminderScheduleTx(ctx, tx, definition.ID); err != nil {
			return err
		}
		source, err := s.loadReminderSourceTx(ctx, tx, userID, definition.SourceKind, definition.SourceID)
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		fireAt, err := calculateReminderFireAt(definition, source)
		if errors.Is(err, ErrReminderSourceUnscheduled) || errors.Is(err, ErrReminderScheduleMismatch) || errors.Is(err, ErrReminderInvalidLocalTime) {
			continue
		}
		if err != nil {
			return err
		}
		future, err := isFutureFireAt(ctx, tx, fireAt)
		if err != nil {
			return err
		}
		if !future {
			continue
		}
		generation, err := nextReminderGenerationTx(ctx, tx, definition.ID)
		if err != nil {
			return err
		}
		if err := s.insertReminderScheduleTx(ctx, tx, definition, generation, fireAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) insertReminderScheduleTx(ctx context.Context, tx pgx.Tx, definition reminderDefinition, generation int64, fireAt time.Time) error {
	scheduleID, err := NewUUID()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lifehub.reminder_schedules (id, reminder_id, user_id, generation, fire_at)
		VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5)
	`, scheduleID, definition.ID, definition.UserID, generation, fireAt); err != nil {
		return fmt.Errorf("insert reminder schedule: %w", err)
	}
	result, err := s.riverClient.InsertTx(ctx, tx, reminders.FireArgs{ScheduleID: scheduleID, Generation: generation}, &river.InsertOpts{
		Queue: reminders.QueueName, ScheduledAt: fireAt, MaxAttempts: reminders.MaxAttempts,
	})
	if err != nil {
		return fmt.Errorf("insert reminder River job: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lifehub.reminder_schedules
		SET river_job_id = $2
		WHERE id = $1::uuid
	`, scheduleID, result.Job.ID); err != nil {
		return fmt.Errorf("link reminder River job: %w", err)
	}
	return nil
}

func (s *Store) invalidateActiveReminderScheduleTx(ctx context.Context, tx pgx.Tx, reminderID string) error {
	var (
		scheduleID string
		jobID      pgtype.Int8
	)
	err := tx.QueryRow(ctx, `
		SELECT id::text, river_job_id
		FROM lifehub.reminder_schedules
		WHERE reminder_id = $1::uuid AND state = 'scheduled'
		FOR UPDATE
	`, reminderID).Scan(&scheduleID, &jobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lock active reminder schedule: %w", err)
	}
	if jobID.Valid {
		if _, err := s.riverClient.JobCancelTx(ctx, tx, jobID.Int64); err != nil && !errors.Is(err, river.ErrNotFound) {
			return fmt.Errorf("cancel reminder River job: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lifehub.reminder_schedules
		SET state = 'invalidated', invalidated_at = now()
		WHERE id = $1::uuid AND state = 'scheduled'
	`, scheduleID); err != nil {
		return fmt.Errorf("invalidate reminder schedule: %w", err)
	}
	return nil
}

func invalidateRunningScheduleTx(ctx context.Context, tx pgx.Tx, scheduleID string) error {
	if _, err := tx.Exec(ctx, `
		UPDATE lifehub.reminder_schedules
		SET state = 'invalidated', invalidated_at = now()
		WHERE id = $1::uuid AND state = 'scheduled'
	`, scheduleID); err != nil {
		return fmt.Errorf("invalidate stale reminder schedule: %w", err)
	}
	return nil
}

func nextReminderGenerationTx(ctx context.Context, tx pgx.Tx, reminderID string) (int64, error) {
	var generation int64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(max(generation), 0) + 1
		FROM lifehub.reminder_schedules
		WHERE reminder_id = $1::uuid
	`, reminderID).Scan(&generation); err != nil {
		return 0, fmt.Errorf("next reminder generation: %w", err)
	}
	return generation, nil
}

func reminderNotificationTitle(sourceKind string) string {
	switch sourceKind {
	case "task":
		return "Pengingat tugas"
	case "event":
		return "Pengingat jadwal"
	case "bill":
		return "Pengingat tagihan"
	default:
		return "Pengingat dokumen"
	}
}

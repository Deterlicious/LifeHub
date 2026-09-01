package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"lifehub/services/api/db/migrations"
	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/riverinfra"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool        *pgxpool.Pool
	riverClient *river.Client[pgx.Tx]
}

type PoolSettings struct {
	MaxConns int32
	MinConns int32
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	return OpenWithPoolSettings(ctx, databaseURL, PoolSettings{MaxConns: 12, MinConns: 1})
}

func OpenWithPoolSettings(ctx context.Context, databaseURL string, settings PoolSettings) (*Store, error) {
	if settings.MaxConns < 1 || settings.MinConns < 0 || settings.MinConns > settings.MaxConns {
		return nil, fmt.Errorf("invalid database pool settings")
	}
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration")
	}
	config.MaxConns = settings.MaxConns
	config.MinConns = settings.MinConns
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute
	config.HealthCheckPeriod = time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	riverClient, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
		Schema:              riverinfra.Schema,
		SkipUnknownJobCheck: true,
	})
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("create River insert client: %w", err)
	}
	return &Store{pool: pool, riverClient: riverClient}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Ready(ctx context.Context) error {
	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	var applicationVersion int
	var applicationApplied bool
	if err := s.pool.QueryRow(ctx, `
		SELECT version_id, is_applied
		FROM lifehub.goose_db_version
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&applicationVersion, &applicationApplied); err != nil {
		return fmt.Errorf("read application schema version: %w", err)
	}
	if !applicationApplied || applicationVersion != migrations.LatestVersion {
		return fmt.Errorf("application schema version is %d, require %d", applicationVersion, migrations.LatestVersion)
	}

	var riverMaximum, riverCount int
	if err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(version), 0), COUNT(DISTINCT version)
		FROM river.river_migration
		WHERE line = 'main' AND version BETWEEN 1 AND $1
	`, riverinfra.TargetVersion).Scan(&riverMaximum, &riverCount); err != nil {
		return fmt.Errorf("read River schema version: %w", err)
	}
	if riverMaximum != riverinfra.TargetVersion || riverCount != riverinfra.TargetVersion {
		return fmt.Errorf("River schema is incomplete at version %d", riverMaximum)
	}
	return nil
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) GetProfile(ctx context.Context, userID string) (domain.Profile, error) {
	if _, err := s.pool.Exec(ctx, `
		INSERT INTO lifehub.profiles (user_id)
		VALUES ($1::uuid)
		ON CONFLICT (user_id) DO NOTHING
	`, userID); err != nil {
		return domain.Profile{}, fmt.Errorf("ensure profile: %w", err)
	}

	profile, err := scanProfile(s.pool.QueryRow(ctx, `
		SELECT user_id::text, timezone, locale, currency
		FROM lifehub.profiles
		WHERE user_id = $1::uuid
	`, userID))
	if err != nil {
		return domain.Profile{}, fmt.Errorf("get profile: %w", err)
	}
	return profile, nil
}

func (s *Store) UpdateProfileTimezone(ctx context.Context, userID, timezone string) (domain.Profile, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("begin update profile: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	profile, err := scanProfile(tx.QueryRow(ctx, `
		INSERT INTO lifehub.profiles (user_id, timezone)
		VALUES ($1::uuid, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET timezone = EXCLUDED.timezone, updated_at = now()
		RETURNING user_id::text, timezone, locale, currency
	`, userID, timezone))
	if err != nil {
		return domain.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	if err := s.refreshUserDateRemindersTx(ctx, tx, userID); err != nil {
		return domain.Profile{}, fmt.Errorf("refresh timezone reminders: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Profile{}, fmt.Errorf("commit update profile: %w", err)
	}
	return profile, nil
}

func (s *Store) DeleteUserData(ctx context.Context, userID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete user data: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	rows, err := tx.Query(ctx, `
		SELECT river_job_id
		FROM lifehub.reminder_schedules
		WHERE user_id = $1::uuid
		  AND state = 'scheduled'
		  AND river_job_id IS NOT NULL
		FOR UPDATE
	`, userID)
	if err != nil {
		return fmt.Errorf("lock user reminder jobs: %w", err)
	}
	jobIDs := make([]int64, 0)
	for rows.Next() {
		var jobID int64
		if err := rows.Scan(&jobID); err != nil {
			rows.Close()
			return fmt.Errorf("scan user reminder job: %w", err)
		}
		jobIDs = append(jobIDs, jobID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("read user reminder jobs: %w", err)
	}
	rows.Close()

	for _, jobID := range jobIDs {
		if _, err := s.riverClient.JobCancelTx(ctx, tx, jobID); err != nil && !errors.Is(err, river.ErrNotFound) {
			return fmt.Errorf("cancel user reminder job: %w", err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM lifehub.profiles WHERE user_id = $1::uuid`, userID); err != nil {
		return fmt.Errorf("delete user profile data: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete user data: %w", err)
	}
	return nil
}

func (s *Store) CreateTask(ctx context.Context, params domain.CreateTaskParams) (domain.Task, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO lifehub.tasks (id, user_id, title, notes, priority, due_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
		RETURNING id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
	`, params.ID, params.UserID, params.Title, params.Notes, params.Priority, params.DueAt)
	task, err := scanTask(row)
	if err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}
	return task, nil
}

func (s *Store) CompleteTask(ctx context.Context, userID, taskID string) (domain.Task, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin complete task: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var alreadyCompleted bool
	if err := tx.QueryRow(ctx, `
		SELECT completed_at IS NOT NULL
		FROM lifehub.tasks
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		FOR UPDATE
	`, taskID, userID).Scan(&alreadyCompleted); errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	} else if err != nil {
		return domain.Task{}, fmt.Errorf("lock task for completion: %w", err)
	}
	task, err := scanTask(tx.QueryRow(ctx, `
		UPDATE lifehub.tasks
		SET completed_at = COALESCE(completed_at, now()),
		    updated_at = CASE WHEN completed_at IS NULL THEN now() ELSE updated_at END
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		RETURNING id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
	`, taskID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("complete task: %w", err)
	}
	if !alreadyCompleted {
		if err := s.refreshSourceRemindersTx(ctx, tx, userID, "task", taskID, false); err != nil {
			return domain.Task{}, fmt.Errorf("refresh completed task reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit complete task: %w", err)
	}
	return task, nil
}

func (s *Store) CreateEvent(ctx context.Context, params domain.CreateEventParams) (domain.Event, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO lifehub.events (
			id, user_id, title, notes, location, all_day, timezone,
			starts_at, ends_at, starts_on, ends_on
		)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id::text, title, notes, location, all_day, timezone,
			starts_at, ends_at, starts_on, ends_on, created_at, updated_at
	`,
		params.ID,
		params.UserID,
		params.Title,
		params.Notes,
		params.Location,
		params.AllDay,
		params.Timezone,
		params.StartsAt,
		params.EndsAt,
		params.StartsOn,
		params.EndsOn,
	)
	event, err := scanEvent(row)
	if err != nil {
		return domain.Event{}, fmt.Errorf("create event: %w", err)
	}
	return event, nil
}

// ListTodayEvents returns every owned event that intersects Today plus future
// events that start inside the bounded Upcoming horizon. Go separates the two
// feeds and performs final cross-domain ordering.
func (s *Store) ListTodayEvents(ctx context.Context, userID, date, horizonDate string, start, horizonEnd time.Time) ([]domain.Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, notes, location, all_day, timezone,
			starts_at, ends_at, starts_on, ends_on, created_at, updated_at
		FROM lifehub.events
		WHERE user_id = $1::uuid
		  AND excluded_at IS NULL
		  AND (
			(
				all_day
				AND starts_on <= $3::date
				AND COALESCE(ends_on, starts_on) >= $2::date
			)
			OR
			(
				NOT all_day
				AND (
					(ends_at IS NULL AND starts_at >= $4 AND starts_at < $5)
					OR
					(ends_at IS NOT NULL AND starts_at < $5 AND ends_at > $4)
				)
			)
		  )
		ORDER BY created_at, id
	`, userID, date, horizonDate, start, horizonEnd)
	if err != nil {
		return nil, fmt.Errorf("list today events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan today event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate today events: %w", err)
	}
	return events, nil
}

func (s *Store) CreateBill(ctx context.Context, params domain.CreateBillParams) (domain.Bill, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO lifehub.bills (id, user_id, title, notes, amount, currency, due_at)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
		RETURNING id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
	`, params.ID, params.UserID, params.Title, params.Notes, params.Amount, params.Currency, params.DueAt)
	bill, err := scanBill(row)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("create bill: %w", err)
	}
	return bill, nil
}

func (s *Store) MarkBillPaid(ctx context.Context, userID, billID string) (domain.Bill, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin mark bill paid: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var alreadyPaid bool
	if err := tx.QueryRow(ctx, `
		SELECT paid_at IS NOT NULL
		FROM lifehub.bills
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		FOR UPDATE
	`, billID, userID).Scan(&alreadyPaid); errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, ErrNotFound
	} else if err != nil {
		return domain.Bill{}, fmt.Errorf("lock bill for payment: %w", err)
	}
	bill, err := scanBill(tx.QueryRow(ctx, `
		UPDATE lifehub.bills
		SET paid_at = COALESCE(paid_at, now()),
		    updated_at = CASE WHEN paid_at IS NULL THEN now() ELSE updated_at END
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		RETURNING id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
	`, billID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("mark bill paid: %w", err)
	}
	if !alreadyPaid {
		if err := s.refreshSourceRemindersTx(ctx, tx, userID, "bill", billID, false); err != nil {
			return domain.Bill{}, fmt.Errorf("refresh paid bill reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit mark bill paid: %w", err)
	}
	return bill, nil
}

// ListTodayBills returns every owned unpaid bill due before the Upcoming
// horizon plus bills paid during Today. Go separates current and future rows.
func (s *Store) ListTodayBills(ctx context.Context, userID string, start, end, horizonEnd time.Time) ([]domain.Bill, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
		FROM lifehub.bills
		WHERE user_id = $1::uuid
		  AND excluded_at IS NULL
		  AND (
			(paid_at IS NULL AND due_at < $4)
			OR (paid_at >= $2 AND paid_at < $3)
		  )
		ORDER BY
			CASE WHEN paid_at IS NOT NULL THEN 1 ELSE 0 END,
			CASE WHEN paid_at IS NULL THEN due_at END,
			CASE WHEN paid_at IS NOT NULL THEN paid_at END,
			created_at,
			id
	`, userID, start, end, horizonEnd)
	if err != nil {
		return nil, fmt.Errorf("list today bills: %w", err)
	}
	defer rows.Close()

	bills := make([]domain.Bill, 0)
	for rows.Next() {
		bill, err := scanBill(rows)
		if err != nil {
			return nil, fmt.Errorf("scan today bill: %w", err)
		}
		bills = append(bills, bill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate today bills: %w", err)
	}
	return bills, nil
}

func (s *Store) CreateDocument(ctx context.Context, params domain.CreateDocumentParams) (domain.Document, error) {
	row := s.pool.QueryRow(ctx, `
		INSERT INTO lifehub.documents (id, user_id, name, category, notes, expires_on)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)
		RETURNING id::text, name, category, notes, expires_on, created_at, updated_at
	`, params.ID, params.UserID, params.Name, params.Category, params.Notes, params.ExpiresOn)
	document, err := scanDocument(row)
	if err != nil {
		return domain.Document{}, fmt.Errorf("create document: %w", err)
	}
	return document, nil
}

func (s *Store) ListDocuments(ctx context.Context, userID string) ([]domain.Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, category, notes, expires_on, created_at, updated_at
		FROM lifehub.documents
		WHERE user_id = $1::uuid
		ORDER BY expires_on, created_at, id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	documents := make([]domain.Document, 0)
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}
	return documents, nil
}

func (s *Store) GetDocument(ctx context.Context, userID, documentID string) (domain.Document, error) {
	document, err := scanDocument(s.pool.QueryRow(ctx, `
		SELECT id::text, name, category, notes, expires_on, created_at, updated_at
		FROM lifehub.documents
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, documentID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Document{}, ErrNotFound
	}
	if err != nil {
		return domain.Document{}, fmt.Errorf("get document: %w", err)
	}
	return document, nil
}

func (s *Store) UpdateDocument(ctx context.Context, params domain.UpdateDocumentParams) (domain.Document, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Document{}, fmt.Errorf("begin update document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	document, err := scanDocument(tx.QueryRow(ctx, `
		UPDATE lifehub.documents
		SET name = CASE WHEN $3::boolean THEN $4::text ELSE name END,
		    category = CASE WHEN $5::boolean THEN $6::text ELSE category END,
		    notes = CASE WHEN $7::boolean THEN $8::text ELSE notes END,
		    expires_on = CASE WHEN $9::boolean THEN $10::date ELSE expires_on END,
		    updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING id::text, name, category, notes, expires_on, created_at, updated_at
	`,
		params.ID,
		params.UserID,
		params.Name != nil,
		params.Name,
		params.Category != nil,
		params.Category,
		params.NotesSet,
		params.Notes,
		params.ExpiresOn != nil,
		params.ExpiresOn,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Document{}, ErrNotFound
	}
	if err != nil {
		return domain.Document{}, fmt.Errorf("update document: %w", err)
	}
	if params.ExpiresOn != nil {
		if err := s.refreshSourceRemindersTx(ctx, tx, params.UserID, "document", params.ID, false); err != nil {
			return domain.Document{}, fmt.Errorf("refresh document reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Document{}, fmt.Errorf("commit update document: %w", err)
	}
	return document, nil
}

func (s *Store) DeleteDocument(ctx context.Context, userID, documentID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete document: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Exec(ctx, `
		DELETE FROM lifehub.documents
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, documentID, userID)
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	if err := s.refreshSourceRemindersTx(ctx, tx, userID, "document", documentID, true); err != nil {
		return fmt.Errorf("delete document reminders: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete document: %w", err)
	}
	return nil
}

// ListTodayDocuments returns every expired owned document plus documents
// expiring through the inclusive horizon. No trusted Today items are capped.
func (s *Store) ListTodayDocuments(ctx context.Context, userID, horizonDate string) ([]domain.Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, category, notes, expires_on, created_at, updated_at
		FROM lifehub.documents
		WHERE user_id = $1::uuid
		  AND expires_on <= $2::date
		ORDER BY expires_on, created_at, id
	`, userID, horizonDate)
	if err != nil {
		return nil, fmt.Errorf("list today documents: %w", err)
	}
	defer rows.Close()

	documents := make([]domain.Document, 0)
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan today document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate today documents: %w", err)
	}
	return documents, nil
}

// ListTodayTasks returns every matching task so a trusted daily view never
// silently hides work. Completed tasks from other days and future tasks are
// absent; Go owns the final domain ordering.
func (s *Store) ListTodayTasks(ctx context.Context, userID string, start, end, horizonEnd time.Time) ([]domain.Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
		FROM lifehub.tasks
		WHERE user_id = $1::uuid
		  AND excluded_at IS NULL
		  AND (
			(completed_at IS NULL AND (due_at IS NULL OR due_at < $4))
			OR (completed_at >= $2 AND completed_at < $3)
		  )
		ORDER BY
			CASE
				WHEN completed_at >= $2 AND completed_at < $3 THEN 3
				WHEN due_at IS NULL THEN 2
				WHEN due_at < $2 THEN 0
				ELSE 1
			END,
			CASE WHEN completed_at IS NULL THEN due_at END ASC NULLS LAST,
			CASE WHEN completed_at IS NOT NULL THEN completed_at END ASC NULLS LAST,
			CASE priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END,
			created_at,
			id
	`, userID, start, end, horizonEnd)
	if err != nil {
		return nil, fmt.Errorf("list today tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan today task: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate today tasks: %w", err)
	}
	return tasks, nil
}

func NewUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := make([]byte, 32)
	hex.Encode(encoded, value[:])
	return string(encoded[0:8]) + "-" +
		string(encoded[8:12]) + "-" +
		string(encoded[12:16]) + "-" +
		string(encoded[16:20]) + "-" +
		string(encoded[20:32]), nil
}

func ValidUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	decoded, err := hex.DecodeString(
		value[0:8] + value[9:13] + value[14:18] + value[19:23] + value[24:36],
	)
	return err == nil && len(decoded) == 16
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProfile(row scanner) (domain.Profile, error) {
	var (
		profile  domain.Profile
		timezone pgtype.Text
	)
	if err := row.Scan(&profile.UserID, &timezone, &profile.Locale, &profile.Currency); err != nil {
		return domain.Profile{}, err
	}
	if timezone.Valid {
		profile.Timezone = &timezone.String
	}
	return profile, nil
}

func scanTask(row scanner) (domain.Task, error) {
	var (
		task      domain.Task
		notes     pgtype.Text
		due       pgtype.Timestamptz
		completed pgtype.Timestamptz
	)
	err := row.Scan(
		&task.ID,
		&task.Title,
		&notes,
		&task.Priority,
		&due,
		&completed,
		&task.CreatedAt,
		&task.UpdatedAt,
	)
	if err != nil {
		return domain.Task{}, err
	}
	if notes.Valid {
		task.Notes = &notes.String
	}
	if due.Valid {
		value := due.Time
		task.DueAt = &value
	}
	if completed.Valid {
		value := completed.Time
		task.CompletedAt = &value
	}
	return task, nil
}

func scanEvent(row scanner) (domain.Event, error) {
	var (
		event    domain.Event
		notes    pgtype.Text
		location pgtype.Text
		startsAt pgtype.Timestamptz
		endsAt   pgtype.Timestamptz
		startsOn pgtype.Date
		endsOn   pgtype.Date
	)
	err := row.Scan(
		&event.ID,
		&event.Title,
		&notes,
		&location,
		&event.AllDay,
		&event.Timezone,
		&startsAt,
		&endsAt,
		&startsOn,
		&endsOn,
		&event.CreatedAt,
		&event.UpdatedAt,
	)
	if err != nil {
		return domain.Event{}, err
	}
	if notes.Valid {
		event.Notes = &notes.String
	}
	if location.Valid {
		event.Location = &location.String
	}
	if startsAt.Valid {
		value := startsAt.Time
		event.StartsAt = &value
	}
	if endsAt.Valid {
		value := endsAt.Time
		event.EndsAt = &value
	}
	if startsOn.Valid {
		value := startsOn.Time.Format(time.DateOnly)
		event.StartsOn = &value
	}
	if endsOn.Valid {
		value := endsOn.Time.Format(time.DateOnly)
		event.EndsOn = &value
	}
	return event, nil
}

func scanBill(row scanner) (domain.Bill, error) {
	var (
		bill   domain.Bill
		notes  pgtype.Text
		paidAt pgtype.Timestamptz
	)
	err := row.Scan(
		&bill.ID,
		&bill.Title,
		&notes,
		&bill.Amount,
		&bill.Currency,
		&bill.DueAt,
		&paidAt,
		&bill.CreatedAt,
		&bill.UpdatedAt,
	)
	if err != nil {
		return domain.Bill{}, err
	}
	if notes.Valid {
		bill.Notes = &notes.String
	}
	if paidAt.Valid {
		value := paidAt.Time
		bill.PaidAt = &value
	}
	return bill, nil
}

func scanDocument(row scanner) (domain.Document, error) {
	var (
		document domain.Document
		notes    pgtype.Text
		expires  pgtype.Date
	)
	err := row.Scan(
		&document.ID,
		&document.Name,
		&document.Category,
		&notes,
		&expires,
		&document.CreatedAt,
		&document.UpdatedAt,
	)
	if err != nil {
		return domain.Document{}, err
	}
	if notes.Valid {
		document.Notes = &notes.String
	}
	if expires.Valid {
		document.ExpiresOn = expires.Time.Format(time.DateOnly)
	}
	return document, nil
}

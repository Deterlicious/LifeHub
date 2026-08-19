package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"lifehub/services/api/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Store, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	config.MaxConns = 12
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 15 * time.Minute
	config.HealthCheckPeriod = time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
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
	profile, err := scanProfile(s.pool.QueryRow(ctx, `
		INSERT INTO lifehub.profiles (user_id, timezone)
		VALUES ($1::uuid, $2)
		ON CONFLICT (user_id) DO UPDATE
		SET timezone = EXCLUDED.timezone, updated_at = now()
		RETURNING user_id::text, timezone, locale, currency
	`, userID, timezone))
	if err != nil {
		return domain.Profile{}, fmt.Errorf("update profile: %w", err)
	}
	return profile, nil
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
	row := s.pool.QueryRow(ctx, `
		UPDATE lifehub.tasks
		SET completed_at = COALESCE(completed_at, now()),
		    updated_at = CASE WHEN completed_at IS NULL THEN now() ELSE updated_at END
		WHERE id = $1::uuid AND user_id = $2::uuid
		RETURNING id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
	`, taskID, userID)
	task, err := scanTask(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("complete task: %w", err)
	}
	return task, nil
}

// ListTodayTasks returns every matching task so a trusted daily view never
// silently hides work. Completed tasks from other days and future tasks are
// absent; Go owns the final domain ordering.
func (s *Store) ListTodayTasks(ctx context.Context, userID string, start, end time.Time) ([]domain.Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
		FROM lifehub.tasks
		WHERE user_id = $1::uuid
		  AND (
			(completed_at IS NULL AND (due_at IS NULL OR due_at < $3))
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
	`, userID, start, end)
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

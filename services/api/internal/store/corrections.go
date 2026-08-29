package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lifehub/services/api/internal/domain"

	"github.com/jackc/pgx/v5"
)

func (s *Store) GetTask(ctx context.Context, userID, taskID string) (domain.Task, error) {
	task, err := scanTask(s.pool.QueryRow(ctx, `
		SELECT id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
		FROM lifehub.tasks
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
	`, taskID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("get task: %w", err)
	}
	return task, nil
}

func (s *Store) UpdateTask(ctx context.Context, params domain.UpdateTaskParams) (domain.Task, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin update task: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	task, err := scanTask(tx.QueryRow(ctx, `
		UPDATE lifehub.tasks
		SET title = CASE WHEN $3::boolean THEN $4::text ELSE title END,
		    notes = CASE WHEN $5::boolean THEN $6::text ELSE notes END,
		    priority = CASE WHEN $7::boolean THEN $8::text ELSE priority END,
		    due_at = CASE WHEN $9::boolean THEN $10::timestamptz ELSE due_at END,
		    is_exception = CASE WHEN series_id IS NOT NULL THEN true ELSE is_exception END,
		    updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		RETURNING id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
	`,
		params.ID,
		params.UserID,
		params.Title != nil,
		params.Title,
		params.NotesSet,
		params.Notes,
		params.Priority != nil,
		params.Priority,
		params.DueAtSet,
		params.DueAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("update task: %w", err)
	}
	if params.DueAtSet {
		if err := s.refreshSourceRemindersTx(ctx, tx, params.UserID, "task", params.ID, false); err != nil {
			return domain.Task{}, fmt.Errorf("refresh task reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit update task: %w", err)
	}
	return task, nil
}

func (s *Store) DeleteTask(ctx context.Context, userID, taskID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete task: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := deleteOrExcludeOccurrenceTx(ctx, tx, "tasks", userID, taskID); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if err := s.refreshSourceRemindersTx(ctx, tx, userID, "task", taskID, true); err != nil {
		return fmt.Errorf("delete task reminders: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete task: %w", err)
	}
	return nil
}

func (s *Store) UncompleteTask(ctx context.Context, userID, taskID string) (domain.Task, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin uncomplete task: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var wasCompleted bool
	if err := tx.QueryRow(ctx, `
		SELECT completed_at IS NOT NULL
		FROM lifehub.tasks
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		FOR UPDATE
	`, taskID, userID).Scan(&wasCompleted); errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	} else if err != nil {
		return domain.Task{}, fmt.Errorf("lock task for uncomplete: %w", err)
	}
	task, err := scanTask(tx.QueryRow(ctx, `
		UPDATE lifehub.tasks
		SET completed_at = NULL,
		    updated_at = CASE WHEN completed_at IS NULL THEN updated_at ELSE now() END
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		RETURNING id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
	`, taskID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Task{}, ErrNotFound
	}
	if err != nil {
		return domain.Task{}, fmt.Errorf("uncomplete task: %w", err)
	}
	if wasCompleted {
		if err := s.refreshSourceRemindersTx(ctx, tx, userID, "task", taskID, false); err != nil {
			return domain.Task{}, fmt.Errorf("refresh uncompleted task reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit uncomplete task: %w", err)
	}
	return task, nil
}

func (s *Store) ListAgendaTasks(ctx context.Context, userID string, start, end time.Time) ([]domain.Task, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
		FROM lifehub.tasks
		WHERE user_id = $1::uuid
		  AND excluded_at IS NULL
		  AND completed_at IS NULL
		  AND due_at >= $2
		  AND due_at < $3
		ORDER BY due_at, CASE priority WHEN 'high' THEN 0 WHEN 'normal' THEN 1 ELSE 2 END, created_at, id
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("list agenda tasks: %w", err)
	}
	defer rows.Close()

	tasks := make([]domain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agenda task: %w", err)
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agenda tasks: %w", err)
	}
	return tasks, nil
}

func (s *Store) ListEvents(ctx context.Context, userID, from, to string, start, end time.Time) ([]domain.Event, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, notes, location, all_day, timezone,
		       starts_at, ends_at, starts_on, ends_on, created_at, updated_at
		FROM lifehub.events
		WHERE user_id = $1::uuid
		  AND excluded_at IS NULL
		  AND (
		    (
		      all_day = true
		      AND starts_on <= $3::date
		      AND COALESCE(ends_on, starts_on) >= $2::date
		    )
		    OR
		    (
		      all_day = false
		      AND starts_at < $5
		      AND (
		        (ends_at IS NULL AND starts_at >= $4)
		        OR (ends_at IS NOT NULL AND ends_at > $4)
		      )
		    )
		  )
		ORDER BY created_at, id
	`, userID, from, to, start, end)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

func (s *Store) GetEvent(ctx context.Context, userID, eventID string) (domain.Event, error) {
	event, err := scanEvent(s.pool.QueryRow(ctx, `
		SELECT id::text, title, notes, location, all_day, timezone,
		       starts_at, ends_at, starts_on, ends_on, created_at, updated_at
		FROM lifehub.events
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
	`, eventID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("get event: %w", err)
	}
	return event, nil
}

func (s *Store) UpdateEvent(ctx context.Context, params domain.UpdateEventParams) (domain.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("begin update event: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	event, err := scanEvent(tx.QueryRow(ctx, `
		UPDATE lifehub.events
		SET title = CASE WHEN $3::boolean THEN $4::text ELSE title END,
		    notes = CASE WHEN $5::boolean THEN $6::text ELSE notes END,
		    location = CASE WHEN $7::boolean THEN $8::text ELSE location END,
		    all_day = CASE WHEN $9::boolean THEN $10::boolean ELSE all_day END,
		    timezone = CASE WHEN $9::boolean THEN $11::text ELSE timezone END,
		    starts_at = CASE WHEN $9::boolean THEN $12::timestamptz ELSE starts_at END,
		    ends_at = CASE WHEN $9::boolean THEN $13::timestamptz ELSE ends_at END,
		    starts_on = CASE WHEN $9::boolean THEN $14::date ELSE starts_on END,
		    ends_on = CASE WHEN $9::boolean THEN $15::date ELSE ends_on END,
		    is_exception = CASE WHEN series_id IS NOT NULL THEN true ELSE is_exception END,
		    updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		RETURNING id::text, title, notes, location, all_day, timezone,
		          starts_at, ends_at, starts_on, ends_on, created_at, updated_at
	`,
		params.ID,
		params.UserID,
		params.Title != nil,
		params.Title,
		params.NotesSet,
		params.Notes,
		params.LocationSet,
		params.Location,
		params.ScheduleSet,
		params.AllDay,
		params.Timezone,
		params.StartsAt,
		params.EndsAt,
		params.StartsOn,
		params.EndsOn,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Event{}, ErrNotFound
	}
	if err != nil {
		return domain.Event{}, fmt.Errorf("update event: %w", err)
	}
	if params.ScheduleSet {
		if err := s.refreshSourceRemindersTx(ctx, tx, params.UserID, "event", params.ID, false); err != nil {
			return domain.Event{}, fmt.Errorf("refresh event reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, fmt.Errorf("commit update event: %w", err)
	}
	return event, nil
}

func (s *Store) DeleteEvent(ctx context.Context, userID, eventID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete event: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := deleteOrExcludeOccurrenceTx(ctx, tx, "events", userID, eventID); err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	if err := s.refreshSourceRemindersTx(ctx, tx, userID, "event", eventID, true); err != nil {
		return fmt.Errorf("delete event reminders: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete event: %w", err)
	}
	return nil
}

func (s *Store) ListBills(ctx context.Context, userID, state string, limit int, afterAt *time.Time, afterID *string) ([]domain.Bill, error) {
	var (
		rows pgx.Rows
		err  error
	)
	if state == "paid" {
		rows, err = s.pool.Query(ctx, `
			SELECT id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
			FROM lifehub.bills
			WHERE user_id = $1::uuid
			  AND excluded_at IS NULL
			  AND paid_at IS NOT NULL
			  AND (
			    $3::timestamptz IS NULL
			    OR paid_at < $3
			    OR (paid_at = $3 AND id > $4::uuid)
			  )
			ORDER BY paid_at DESC, id ASC
			LIMIT $2
		`, userID, limit, afterAt, afterID)
	} else {
		rows, err = s.pool.Query(ctx, `
			SELECT id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
			FROM lifehub.bills
			WHERE user_id = $1::uuid
			  AND excluded_at IS NULL
			  AND paid_at IS NULL
			  AND (
			    $3::timestamptz IS NULL
			    OR due_at > $3
			    OR (due_at = $3 AND id > $4::uuid)
			  )
			ORDER BY due_at ASC, id ASC
			LIMIT $2
		`, userID, limit, afterAt, afterID)
	}
	if err != nil {
		return nil, fmt.Errorf("list %s bills: %w", state, err)
	}
	defer rows.Close()

	bills := make([]domain.Bill, 0)
	for rows.Next() {
		bill, err := scanBill(rows)
		if err != nil {
			return nil, fmt.Errorf("scan %s bill: %w", state, err)
		}
		bills = append(bills, bill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate %s bills: %w", state, err)
	}
	return bills, nil
}

func (s *Store) GetBill(ctx context.Context, userID, billID string) (domain.Bill, error) {
	bill, err := scanBill(s.pool.QueryRow(ctx, `
		SELECT id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
		FROM lifehub.bills
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
	`, billID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("get bill: %w", err)
	}
	return bill, nil
}

func (s *Store) UpdateBill(ctx context.Context, params domain.UpdateBillParams) (domain.Bill, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin update bill: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	bill, err := scanBill(tx.QueryRow(ctx, `
		UPDATE lifehub.bills
		SET title = CASE WHEN $3::boolean THEN $4::text ELSE title END,
		    notes = CASE WHEN $5::boolean THEN $6::text ELSE notes END,
		    amount = CASE WHEN $7::boolean THEN $8::bigint ELSE amount END,
		    currency = CASE WHEN $9::boolean THEN $10::text ELSE currency END,
		    due_at = CASE WHEN $11::boolean THEN $12::timestamptz ELSE due_at END,
		    is_exception = CASE WHEN series_id IS NOT NULL THEN true ELSE is_exception END,
		    updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		RETURNING id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
	`,
		params.ID,
		params.UserID,
		params.Title != nil,
		params.Title,
		params.NotesSet,
		params.Notes,
		params.Amount != nil,
		params.Amount,
		params.Currency != nil,
		params.Currency,
		params.DueAt != nil,
		params.DueAt,
	))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("update bill: %w", err)
	}
	if params.DueAt != nil {
		if err := s.refreshSourceRemindersTx(ctx, tx, params.UserID, "bill", params.ID, false); err != nil {
			return domain.Bill{}, fmt.Errorf("refresh bill reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit update bill: %w", err)
	}
	return bill, nil
}

func (s *Store) DeleteBill(ctx context.Context, userID, billID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin delete bill: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := deleteOrExcludeOccurrenceTx(ctx, tx, "bills", userID, billID); err != nil {
		return fmt.Errorf("delete bill: %w", err)
	}
	if err := s.refreshSourceRemindersTx(ctx, tx, userID, "bill", billID, true); err != nil {
		return fmt.Errorf("delete bill reminders: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit delete bill: %w", err)
	}
	return nil
}

func (s *Store) MarkBillUnpaid(ctx context.Context, userID, billID string) (domain.Bill, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin mark bill unpaid: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var wasPaid bool
	if err := tx.QueryRow(ctx, `
		SELECT paid_at IS NOT NULL
		FROM lifehub.bills
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		FOR UPDATE
	`, billID, userID).Scan(&wasPaid); errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, ErrNotFound
	} else if err != nil {
		return domain.Bill{}, fmt.Errorf("lock bill for mark unpaid: %w", err)
	}
	bill, err := scanBill(tx.QueryRow(ctx, `
		UPDATE lifehub.bills
		SET paid_at = NULL,
		    updated_at = CASE WHEN paid_at IS NULL THEN updated_at ELSE now() END
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		RETURNING id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
	`, billID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Bill{}, ErrNotFound
	}
	if err != nil {
		return domain.Bill{}, fmt.Errorf("mark bill unpaid: %w", err)
	}
	if wasPaid {
		if err := s.refreshSourceRemindersTx(ctx, tx, userID, "bill", billID, false); err != nil {
			return domain.Bill{}, fmt.Errorf("refresh unpaid bill reminders: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit mark bill unpaid: %w", err)
	}
	return bill, nil
}

func (s *Store) ListAgendaBills(ctx context.Context, userID string, start, end time.Time) ([]domain.Bill, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
		FROM lifehub.bills
		WHERE user_id = $1::uuid
		  AND excluded_at IS NULL
		  AND paid_at IS NULL
		  AND due_at >= $2
		  AND due_at < $3
		ORDER BY due_at, id
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("list agenda bills: %w", err)
	}
	defer rows.Close()

	bills := make([]domain.Bill, 0)
	for rows.Next() {
		bill, err := scanBill(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agenda bill: %w", err)
		}
		bills = append(bills, bill)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agenda bills: %w", err)
	}
	return bills, nil
}

func deleteOrExcludeOccurrenceTx(ctx context.Context, tx pgx.Tx, table, userID, itemID string) error {
	if table != "tasks" && table != "events" && table != "bills" {
		return fmt.Errorf("unsupported recurrence source table")
	}
	var recurring bool
	query := fmt.Sprintf(`
		SELECT series_id IS NOT NULL
		FROM lifehub.%s
		WHERE id = $1::uuid AND user_id = $2::uuid AND excluded_at IS NULL
		FOR UPDATE
	`, table)
	if err := tx.QueryRow(ctx, query, itemID, userID).Scan(&recurring); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	if recurring {
		query = fmt.Sprintf(`
			UPDATE lifehub.%s
			SET excluded_at = now(), is_exception = true, updated_at = now()
			WHERE id = $1::uuid AND user_id = $2::uuid
		`, table)
	} else {
		query = fmt.Sprintf(`DELETE FROM lifehub.%s WHERE id = $1::uuid AND user_id = $2::uuid`, table)
	}
	if _, err := tx.Exec(ctx, query, itemID, userID); err != nil {
		return err
	}
	return nil
}

func (s *Store) ListAgendaDocuments(ctx context.Context, userID, from, to string) ([]domain.Document, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text, name, category, notes, expires_on, created_at, updated_at
		FROM lifehub.documents
		WHERE user_id = $1::uuid
		  AND expires_on >= $2::date
		  AND expires_on <= $3::date
		ORDER BY expires_on, created_at, id
	`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("list agenda documents: %w", err)
	}
	defer rows.Close()

	documents := make([]domain.Document, 0)
	for rows.Next() {
		document, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan agenda document: %w", err)
		}
		documents = append(documents, document)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agenda documents: %w", err)
	}
	return documents, nil
}

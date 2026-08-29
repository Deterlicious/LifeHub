package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"lifehub/services/api/internal/domain"
	"lifehub/services/api/internal/recurrence"
	"lifehub/services/api/internal/timeutil"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrRecurrenceInactive = errors.New("recurrence series is inactive")

func (s *Store) CreateRecurringTask(ctx context.Context, params domain.CreateRecurringTaskParams) (domain.Task, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin recurring task: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := insertRecurrenceSeriesTx(ctx, tx, params.SeriesID, params.Task.UserID, "task", params.Frequency, params.Interval, params.AnchorOn, params.EndsOn, params.Timezone); err != nil {
		return domain.Task{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lifehub.recurrence_task_templates (series_id, title, notes, priority, due_time)
		VALUES ($1::uuid, $2, $3, $4, $5::time)
	`, params.SeriesID, params.Task.Title, params.Task.Notes, params.Task.Priority, params.TimeLocal); err != nil {
		return domain.Task{}, fmt.Errorf("insert recurring task template: %w", err)
	}
	series := recurrenceSeriesRecord{
		ID: params.SeriesID, UserID: params.Task.UserID, SourceKind: "task", Frequency: params.Frequency,
		Interval: params.Interval, AnchorOn: dateOnly(params.AnchorOn), EndsOn: datePointer(params.EndsOn),
		Timezone: params.Timezone, Generation: 1, Active: true,
	}
	template := recurrenceTaskTemplate{Title: params.Task.Title, Notes: params.Task.Notes, Priority: params.Task.Priority, TimeLocal: params.TimeLocal}
	if err := s.materializeTaskSeriesTx(ctx, tx, series, template, params.FromOn, params.ThroughOn, params.Task.ID); err != nil {
		return domain.Task{}, err
	}
	task, err := scanTask(tx.QueryRow(ctx, `
		SELECT id::text, title, notes, priority, due_at, completed_at, created_at, updated_at
		FROM lifehub.tasks
		WHERE series_id = $1::uuid AND occurrence_on = $2::date
	`, params.SeriesID, params.AnchorOn))
	if err != nil {
		return domain.Task{}, fmt.Errorf("load recurring task anchor: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit recurring task: %w", err)
	}
	return task, nil
}

func (s *Store) CreateRecurringEvent(ctx context.Context, params domain.CreateRecurringEventParams) (domain.Event, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Event{}, fmt.Errorf("begin recurring event: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := insertRecurrenceSeriesTx(ctx, tx, params.SeriesID, params.Event.UserID, "event", params.Frequency, params.Interval, params.AnchorOn, params.EndsOn, params.Event.Timezone); err != nil {
		return domain.Event{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lifehub.recurrence_event_templates (
			series_id, title, notes, location, all_day, starts_time, duration_seconds, all_day_span
		)
		VALUES ($1::uuid, $2, $3, $4, $5, $6::time, $7, $8)
	`, params.SeriesID, params.Event.Title, params.Event.Notes, params.Event.Location, params.Event.AllDay, params.TimeLocal, params.DurationSeconds, nullableAllDaySpan(params.Event.AllDay, params.AllDaySpan)); err != nil {
		return domain.Event{}, fmt.Errorf("insert recurring event template: %w", err)
	}
	series := recurrenceSeriesRecord{
		ID: params.SeriesID, UserID: params.Event.UserID, SourceKind: "event", Frequency: params.Frequency,
		Interval: params.Interval, AnchorOn: dateOnly(params.AnchorOn), EndsOn: datePointer(params.EndsOn),
		Timezone: params.Event.Timezone, Generation: 1, Active: true,
	}
	template := recurrenceEventTemplate{
		Title: params.Event.Title, Notes: params.Event.Notes, Location: params.Event.Location, AllDay: params.Event.AllDay,
		TimeLocal: params.TimeLocal, DurationSeconds: params.DurationSeconds, AllDaySpan: params.AllDaySpan,
	}
	if err := s.materializeEventSeriesTx(ctx, tx, series, template, params.FromOn, params.ThroughOn, params.Event.ID); err != nil {
		return domain.Event{}, err
	}
	event, err := scanEvent(tx.QueryRow(ctx, `
		SELECT id::text, title, notes, location, all_day, timezone,
		       starts_at, ends_at, starts_on, ends_on, created_at, updated_at
		FROM lifehub.events
		WHERE series_id = $1::uuid AND occurrence_on = $2::date
	`, params.SeriesID, params.AnchorOn))
	if err != nil {
		return domain.Event{}, fmt.Errorf("load recurring event anchor: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Event{}, fmt.Errorf("commit recurring event: %w", err)
	}
	return event, nil
}

func (s *Store) CreateRecurringBill(ctx context.Context, params domain.CreateRecurringBillParams) (domain.Bill, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.Bill{}, fmt.Errorf("begin recurring bill: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := insertRecurrenceSeriesTx(ctx, tx, params.SeriesID, params.Bill.UserID, "bill", params.Frequency, params.Interval, params.AnchorOn, params.EndsOn, params.Timezone); err != nil {
		return domain.Bill{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO lifehub.recurrence_bill_templates (series_id, title, notes, amount, currency, due_time)
		VALUES ($1::uuid, $2, $3, $4, $5, $6::time)
	`, params.SeriesID, params.Bill.Title, params.Bill.Notes, params.Bill.Amount, params.Bill.Currency, params.TimeLocal); err != nil {
		return domain.Bill{}, fmt.Errorf("insert recurring bill template: %w", err)
	}
	series := recurrenceSeriesRecord{
		ID: params.SeriesID, UserID: params.Bill.UserID, SourceKind: "bill", Frequency: params.Frequency,
		Interval: params.Interval, AnchorOn: dateOnly(params.AnchorOn), EndsOn: datePointer(params.EndsOn),
		Timezone: params.Timezone, Generation: 1, Active: true,
	}
	template := recurrenceBillTemplate{Title: params.Bill.Title, Notes: params.Bill.Notes, Amount: params.Bill.Amount, Currency: params.Bill.Currency, TimeLocal: params.TimeLocal}
	if err := s.materializeBillSeriesTx(ctx, tx, series, template, params.FromOn, params.ThroughOn, params.Bill.ID); err != nil {
		return domain.Bill{}, err
	}
	bill, err := scanBill(tx.QueryRow(ctx, `
		SELECT id::text, title, notes, amount, currency, due_at, paid_at, created_at, updated_at
		FROM lifehub.bills
		WHERE series_id = $1::uuid AND occurrence_on = $2::date
	`, params.SeriesID, params.AnchorOn))
	if err != nil {
		return domain.Bill{}, fmt.Errorf("load recurring bill anchor: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Bill{}, fmt.Errorf("commit recurring bill: %w", err)
	}
	return bill, nil
}

func (s *Store) MaterializeRecurrences(ctx context.Context, userID string, fromOn, throughOn time.Time) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id::text
		FROM lifehub.recurrence_series
		WHERE user_id = $1::uuid AND active
		ORDER BY id
	`, userID)
	if err != nil {
		return fmt.Errorf("list recurrence series for materialization: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("scan recurrence series id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate recurrence series ids: %w", err)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.materializeOwnedSeries(ctx, userID, id, fromOn, throughOn); err != nil {
			return err
		}
	}
	return nil
}

// MaterializeAllRecurrences keeps a rolling occurrence window available even
// when nobody opens Today or Agenda. The River worker calls this through a
// durable, retryable maintenance job. Each series is still locked and inserted
// idempotently by materializeOwnedSeries.
func (s *Store) MaterializeAllRecurrences(ctx context.Context, now time.Time, horizonDays int) error {
	if horizonDays < 1 || horizonDays > 366 {
		return fmt.Errorf("invalid recurrence materialization horizon")
	}
	rows, err := s.pool.Query(ctx, `
		SELECT user_id::text, id::text, timezone
		FROM lifehub.recurrence_series
		WHERE active
		ORDER BY user_id, id
	`)
	if err != nil {
		return fmt.Errorf("list active recurrence series: %w", err)
	}
	type candidate struct {
		userID, seriesID, timezone string
	}
	candidates := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.userID, &item.seriesID, &item.timezone); err != nil {
			rows.Close()
			return fmt.Errorf("scan active recurrence series: %w", err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate active recurrence series: %w", err)
	}
	rows.Close()

	for _, item := range candidates {
		location, err := timeutil.LoadLocation(item.timezone)
		if err != nil {
			return fmt.Errorf("load recurrence timezone: %w", err)
		}
		local := now.In(location)
		fromOn := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)
		if err := s.materializeOwnedSeries(ctx, item.userID, item.seriesID, fromOn, fromOn.AddDate(0, 0, horizonDays)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListRecurrenceSeries(ctx context.Context, userID string) ([]domain.RecurrenceSeries, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT series.id::text, series.source_kind,
		       COALESCE(task.title, event.title, bill.title),
		       series.frequency, series.interval, series.anchor_on, series.ends_on,
		       series.timezone, series.active, series.created_at, series.updated_at
		FROM lifehub.recurrence_series AS series
		LEFT JOIN lifehub.recurrence_task_templates AS task ON task.series_id = series.id
		LEFT JOIN lifehub.recurrence_event_templates AS event ON event.series_id = series.id
		LEFT JOIN lifehub.recurrence_bill_templates AS bill ON bill.series_id = series.id
		WHERE series.user_id = $1::uuid
		ORDER BY series.active DESC, series.created_at, series.id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list recurrence series: %w", err)
	}
	defer rows.Close()
	items := make([]domain.RecurrenceSeries, 0)
	for rows.Next() {
		item, err := scanRecurrenceSeries(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recurrence series: %w", err)
	}
	return items, nil
}

func (s *Store) GetRecurrenceSeries(ctx context.Context, userID, seriesID string) (domain.RecurrenceSeries, error) {
	item, err := scanRecurrenceSeries(s.pool.QueryRow(ctx, `
		SELECT series.id::text, series.source_kind,
		       COALESCE(task.title, event.title, bill.title),
		       series.frequency, series.interval, series.anchor_on, series.ends_on,
		       series.timezone, series.active, series.created_at, series.updated_at
		FROM lifehub.recurrence_series AS series
		LEFT JOIN lifehub.recurrence_task_templates AS task ON task.series_id = series.id
		LEFT JOIN lifehub.recurrence_event_templates AS event ON event.series_id = series.id
		LEFT JOIN lifehub.recurrence_bill_templates AS bill ON bill.series_id = series.id
		WHERE series.id = $1::uuid AND series.user_id = $2::uuid
	`, seriesID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.RecurrenceSeries{}, ErrNotFound
	}
	return item, err
}

func (s *Store) UpdateRecurrenceSeries(ctx context.Context, params domain.UpdateRecurrenceSeriesParams) (domain.RecurrenceSeries, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.RecurrenceSeries{}, fmt.Errorf("begin update recurrence series: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	series, err := loadRecurrenceSeriesRecordTx(ctx, tx, params.UserID, params.ID)
	if err != nil {
		return domain.RecurrenceSeries{}, err
	}
	if !series.Active {
		return domain.RecurrenceSeries{}, ErrRecurrenceInactive
	}
	if err := s.deleteRegeneratedSeriesOccurrencesTx(ctx, tx, params.UserID, params.ID, series.SourceKind, dateOnly(params.FromOn)); err != nil {
		return domain.RecurrenceSeries{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lifehub.recurrence_series
		SET frequency=$3, interval=$4, ends_on=$5::date,
		    generation=generation+1, updated_at=now()
		WHERE id=$1::uuid AND user_id=$2::uuid
	`, params.ID, params.UserID, params.Frequency, params.Interval, params.EndsOn); err != nil {
		return domain.RecurrenceSeries{}, fmt.Errorf("update recurrence rule: %w", err)
	}
	series.Frequency = params.Frequency
	series.Interval = params.Interval
	series.EndsOn = datePointer(params.EndsOn)
	series.Generation++
	switch series.SourceKind {
	case "task":
		template, loadErr := loadTaskTemplateTx(ctx, tx, series.ID)
		if loadErr != nil {
			return domain.RecurrenceSeries{}, loadErr
		}
		if err := s.materializeTaskSeriesTx(ctx, tx, series, template, params.FromOn, params.ThroughOn, ""); err != nil {
			return domain.RecurrenceSeries{}, err
		}
	case "event":
		template, loadErr := loadEventTemplateTx(ctx, tx, series.ID)
		if loadErr != nil {
			return domain.RecurrenceSeries{}, loadErr
		}
		if err := s.materializeEventSeriesTx(ctx, tx, series, template, params.FromOn, params.ThroughOn, ""); err != nil {
			return domain.RecurrenceSeries{}, err
		}
	case "bill":
		template, loadErr := loadBillTemplateTx(ctx, tx, series.ID)
		if loadErr != nil {
			return domain.RecurrenceSeries{}, loadErr
		}
		if err := s.materializeBillSeriesTx(ctx, tx, series, template, params.FromOn, params.ThroughOn, ""); err != nil {
			return domain.RecurrenceSeries{}, err
		}
	default:
		return domain.RecurrenceSeries{}, fmt.Errorf("unsupported recurrence source kind")
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.RecurrenceSeries{}, fmt.Errorf("commit update recurrence series: %w", err)
	}
	return s.GetRecurrenceSeries(ctx, params.UserID, params.ID)
}

func (s *Store) StopRecurrenceSeries(ctx context.Context, userID, seriesID string, fromOn time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin stop recurrence: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var sourceKind string
	if err := tx.QueryRow(ctx, `
		SELECT source_kind
		FROM lifehub.recurrence_series
		WHERE id = $1::uuid AND user_id = $2::uuid
		FOR UPDATE
	`, seriesID, userID).Scan(&sourceKind); errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return fmt.Errorf("lock recurrence series: %w", err)
	}
	if err := s.deleteFutureSeriesOccurrencesTx(ctx, tx, userID, seriesID, sourceKind, dateOnly(fromOn)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE lifehub.recurrence_series
		SET active = false, updated_at = now()
		WHERE id = $1::uuid AND user_id = $2::uuid
	`, seriesID, userID); err != nil {
		return fmt.Errorf("stop recurrence series: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit stop recurrence: %w", err)
	}
	return nil
}

type recurrenceSeriesRecord struct {
	ID, UserID, SourceKind, Frequency, Timezone string
	Interval, Generation                        int
	AnchorOn                                    time.Time
	EndsOn                                      *time.Time
	Active                                      bool
}

type recurrenceTaskTemplate struct {
	Title, Priority, TimeLocal string
	Notes                      *string
}

type recurrenceEventTemplate struct {
	Title           string
	Notes, Location *string
	AllDay          bool
	TimeLocal       *string
	DurationSeconds *int64
	AllDaySpan      int
}

type recurrenceBillTemplate struct {
	Title, Currency, TimeLocal string
	Notes                      *string
	Amount                     int64
}

func insertRecurrenceSeriesTx(ctx context.Context, tx pgx.Tx, id, userID, sourceKind, frequency string, interval int, anchorOn time.Time, endsOn *time.Time, timezone string) error {
	if _, err := tx.Exec(ctx, `
		INSERT INTO lifehub.recurrence_series (
			id, user_id, source_kind, frequency, interval, anchor_on, ends_on, timezone
		)
		VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6::date, $7::date, $8)
	`, id, userID, sourceKind, frequency, interval, anchorOn, endsOn, timezone); err != nil {
		return fmt.Errorf("insert recurrence series: %w", err)
	}
	return nil
}

func (s *Store) materializeOwnedSeries(ctx context.Context, userID, seriesID string, fromOn, throughOn time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin materialize recurrence series: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	series, err := loadRecurrenceSeriesRecordTx(ctx, tx, userID, seriesID)
	if err != nil {
		return err
	}
	if !series.Active {
		return nil
	}
	switch series.SourceKind {
	case "task":
		template, err := loadTaskTemplateTx(ctx, tx, series.ID)
		if err != nil {
			return err
		}
		if err := s.materializeTaskSeriesTx(ctx, tx, series, template, fromOn, throughOn, ""); err != nil {
			return err
		}
	case "event":
		template, err := loadEventTemplateTx(ctx, tx, series.ID)
		if err != nil {
			return err
		}
		if err := s.materializeEventSeriesTx(ctx, tx, series, template, fromOn, throughOn, ""); err != nil {
			return err
		}
	case "bill":
		template, err := loadBillTemplateTx(ctx, tx, series.ID)
		if err != nil {
			return err
		}
		if err := s.materializeBillSeriesTx(ctx, tx, series, template, fromOn, throughOn, ""); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported recurrence source kind")
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit materialize recurrence series: %w", err)
	}
	return nil
}

func loadRecurrenceSeriesRecordTx(ctx context.Context, tx pgx.Tx, userID, seriesID string) (recurrenceSeriesRecord, error) {
	var item recurrenceSeriesRecord
	var ends pgtype.Date
	err := tx.QueryRow(ctx, `
		SELECT id::text, user_id::text, source_kind, frequency, interval, anchor_on, ends_on,
		       timezone, generation, active
		FROM lifehub.recurrence_series
		WHERE id = $1::uuid AND user_id = $2::uuid
		FOR UPDATE
	`, seriesID, userID).Scan(
		&item.ID, &item.UserID, &item.SourceKind, &item.Frequency, &item.Interval, &item.AnchorOn, &ends,
		&item.Timezone, &item.Generation, &item.Active,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return recurrenceSeriesRecord{}, ErrNotFound
	}
	if err != nil {
		return recurrenceSeriesRecord{}, fmt.Errorf("load recurrence series: %w", err)
	}
	if ends.Valid {
		value := dateOnly(ends.Time)
		item.EndsOn = &value
	}
	item.AnchorOn = dateOnly(item.AnchorOn)
	return item, nil
}

func (s *Store) materializeTaskSeriesTx(ctx context.Context, tx pgx.Tx, series recurrenceSeriesRecord, template recurrenceTaskTemplate, fromOn, throughOn time.Time, anchorID string) error {
	dates, err := recurrence.Dates(series.AnchorOn, fromOn, throughOn, recurrence.Rule{Frequency: series.Frequency, Interval: series.Interval, EndsOn: series.EndsOn})
	if err != nil {
		return err
	}
	location, err := timeutil.LoadLocation(series.Timezone)
	if err != nil {
		return err
	}
	for _, occurrenceOn := range dates {
		id := anchorID
		if id == "" || !occurrenceOn.Equal(series.AnchorOn) {
			id, err = NewUUID()
			if err != nil {
				return err
			}
		}
		dueAt, err := timeutil.ResolveRecurringLocalWallTime(occurrenceOn.Format(time.DateOnly)+"T"+template.TimeLocal, location)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO lifehub.tasks (
				id, user_id, title, notes, priority, due_at,
				series_id, occurrence_on, series_generation
			)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7::uuid, $8::date, $9)
			ON CONFLICT (series_id, occurrence_on) WHERE series_id IS NOT NULL DO NOTHING
		`, id, series.UserID, template.Title, template.Notes, template.Priority, dueAt, series.ID, occurrenceOn, series.Generation); err != nil {
			return fmt.Errorf("materialize recurring task: %w", err)
		}
	}
	return nil
}

func (s *Store) materializeEventSeriesTx(ctx context.Context, tx pgx.Tx, series recurrenceSeriesRecord, template recurrenceEventTemplate, fromOn, throughOn time.Time, anchorID string) error {
	dates, err := recurrence.Dates(series.AnchorOn, fromOn, throughOn, recurrence.Rule{Frequency: series.Frequency, Interval: series.Interval, EndsOn: series.EndsOn})
	if err != nil {
		return err
	}
	location, err := timeutil.LoadLocation(series.Timezone)
	if err != nil {
		return err
	}
	for _, occurrenceOn := range dates {
		id := anchorID
		if id == "" || !occurrenceOn.Equal(series.AnchorOn) {
			id, err = NewUUID()
			if err != nil {
				return err
			}
		}
		var startsAt, endsAt *time.Time
		var startsOn, endsOn *time.Time
		if template.AllDay {
			startDate := occurrenceOn
			endDate := occurrenceOn.AddDate(0, 0, template.AllDaySpan)
			startsOn, endsOn = &startDate, &endDate
		} else {
			if template.TimeLocal == nil {
				return fmt.Errorf("recurring timed event has no local time")
			}
			start, err := timeutil.ResolveRecurringLocalWallTime(occurrenceOn.Format(time.DateOnly)+"T"+*template.TimeLocal, location)
			if err != nil {
				return err
			}
			startsAt = &start
			if template.DurationSeconds != nil {
				end := start.Add(time.Duration(*template.DurationSeconds) * time.Second)
				endsAt = &end
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO lifehub.events (
				id, user_id, title, notes, location, all_day, timezone,
				starts_at, ends_at, starts_on, ends_on,
				series_id, occurrence_on, series_generation
			)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10::date, $11::date, $12::uuid, $13::date, $14)
			ON CONFLICT (series_id, occurrence_on) WHERE series_id IS NOT NULL DO NOTHING
		`, id, series.UserID, template.Title, template.Notes, template.Location, template.AllDay, series.Timezone,
			startsAt, endsAt, startsOn, endsOn, series.ID, occurrenceOn, series.Generation); err != nil {
			return fmt.Errorf("materialize recurring event: %w", err)
		}
	}
	return nil
}

func (s *Store) materializeBillSeriesTx(ctx context.Context, tx pgx.Tx, series recurrenceSeriesRecord, template recurrenceBillTemplate, fromOn, throughOn time.Time, anchorID string) error {
	dates, err := recurrence.Dates(series.AnchorOn, fromOn, throughOn, recurrence.Rule{Frequency: series.Frequency, Interval: series.Interval, EndsOn: series.EndsOn})
	if err != nil {
		return err
	}
	location, err := timeutil.LoadLocation(series.Timezone)
	if err != nil {
		return err
	}
	for _, occurrenceOn := range dates {
		id := anchorID
		if id == "" || !occurrenceOn.Equal(series.AnchorOn) {
			id, err = NewUUID()
			if err != nil {
				return err
			}
		}
		dueAt, err := timeutil.ResolveRecurringLocalWallTime(occurrenceOn.Format(time.DateOnly)+"T"+template.TimeLocal, location)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO lifehub.bills (
				id, user_id, title, notes, amount, currency, due_at,
				series_id, occurrence_on, series_generation
			)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::uuid, $9::date, $10)
			ON CONFLICT (series_id, occurrence_on) WHERE series_id IS NOT NULL DO NOTHING
		`, id, series.UserID, template.Title, template.Notes, template.Amount, template.Currency, dueAt, series.ID, occurrenceOn, series.Generation); err != nil {
			return fmt.Errorf("materialize recurring bill: %w", err)
		}
	}
	return nil
}

func loadTaskTemplateTx(ctx context.Context, tx pgx.Tx, seriesID string) (recurrenceTaskTemplate, error) {
	var item recurrenceTaskTemplate
	if err := tx.QueryRow(ctx, `
		SELECT title, notes, priority, to_char(due_time, 'HH24:MI:SS')
		FROM lifehub.recurrence_task_templates WHERE series_id = $1::uuid
	`, seriesID).Scan(&item.Title, &item.Notes, &item.Priority, &item.TimeLocal); err != nil {
		return recurrenceTaskTemplate{}, fmt.Errorf("load recurring task template: %w", err)
	}
	return item, nil
}

func loadEventTemplateTx(ctx context.Context, tx pgx.Tx, seriesID string) (recurrenceEventTemplate, error) {
	var item recurrenceEventTemplate
	var local pgtype.Text
	var duration pgtype.Int8
	var span pgtype.Int4
	if err := tx.QueryRow(ctx, `
		SELECT title, notes, location, all_day, to_char(starts_time, 'HH24:MI:SS'), duration_seconds, all_day_span
		FROM lifehub.recurrence_event_templates WHERE series_id = $1::uuid
	`, seriesID).Scan(&item.Title, &item.Notes, &item.Location, &item.AllDay, &local, &duration, &span); err != nil {
		return recurrenceEventTemplate{}, fmt.Errorf("load recurring event template: %w", err)
	}
	if local.Valid {
		item.TimeLocal = &local.String
	}
	if duration.Valid {
		value := duration.Int64
		item.DurationSeconds = &value
	}
	if span.Valid {
		item.AllDaySpan = int(span.Int32)
	}
	return item, nil
}

func loadBillTemplateTx(ctx context.Context, tx pgx.Tx, seriesID string) (recurrenceBillTemplate, error) {
	var item recurrenceBillTemplate
	if err := tx.QueryRow(ctx, `
		SELECT title, notes, amount, currency, to_char(due_time, 'HH24:MI:SS')
		FROM lifehub.recurrence_bill_templates WHERE series_id = $1::uuid
	`, seriesID).Scan(&item.Title, &item.Notes, &item.Amount, &item.Currency, &item.TimeLocal); err != nil {
		return recurrenceBillTemplate{}, fmt.Errorf("load recurring bill template: %w", err)
	}
	return item, nil
}

func (s *Store) deleteFutureSeriesOccurrencesTx(ctx context.Context, tx pgx.Tx, userID, seriesID, sourceKind string, fromOn time.Time) error {
	return s.deleteSeriesOccurrencesTx(ctx, tx, userID, seriesID, sourceKind, fromOn, false)
}

func (s *Store) deleteRegeneratedSeriesOccurrencesTx(ctx context.Context, tx pgx.Tx, userID, seriesID, sourceKind string, fromOn time.Time) error {
	return s.deleteSeriesOccurrencesTx(ctx, tx, userID, seriesID, sourceKind, fromOn, true)
}

func (s *Store) deleteSeriesOccurrencesTx(ctx context.Context, tx pgx.Tx, userID, seriesID, sourceKind string, fromOn time.Time, preserveExceptions bool) error {
	var query string
	exceptionClause := ""
	if preserveExceptions {
		exceptionClause = " AND NOT is_exception"
	}
	switch sourceKind {
	case "task":
		query = `SELECT id::text FROM lifehub.tasks WHERE user_id=$1::uuid AND series_id=$2::uuid AND occurrence_on >= $3::date AND completed_at IS NULL` + exceptionClause + ` FOR UPDATE`
	case "event":
		query = `SELECT id::text FROM lifehub.events WHERE user_id=$1::uuid AND series_id=$2::uuid AND occurrence_on >= $3::date` + exceptionClause + ` FOR UPDATE`
	case "bill":
		query = `SELECT id::text FROM lifehub.bills WHERE user_id=$1::uuid AND series_id=$2::uuid AND occurrence_on >= $3::date AND paid_at IS NULL` + exceptionClause + ` FOR UPDATE`
	default:
		return fmt.Errorf("unsupported recurrence source kind")
	}
	rows, err := tx.Query(ctx, query, userID, seriesID, fromOn)
	if err != nil {
		return fmt.Errorf("lock future recurrence occurrences: %w", err)
	}
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, id := range ids {
		if err := s.refreshSourceRemindersTx(ctx, tx, userID, sourceKind, id, true); err != nil {
			return err
		}
	}
	table := map[string]string{"task": "tasks", "event": "events", "bill": "bills"}[sourceKind]
	if _, err := tx.Exec(ctx, fmt.Sprintf(`DELETE FROM lifehub.%s WHERE id = ANY($1::uuid[])`, table), ids); err != nil {
		return fmt.Errorf("delete future recurrence occurrences: %w", err)
	}
	return nil
}

func scanRecurrenceSeries(row scanner) (domain.RecurrenceSeries, error) {
	var item domain.RecurrenceSeries
	var anchor time.Time
	var ends pgtype.Date
	if err := row.Scan(&item.ID, &item.SourceKind, &item.Title, &item.Frequency, &item.Interval, &anchor, &ends, &item.Timezone, &item.Active, &item.CreatedAt, &item.UpdatedAt); err != nil {
		return domain.RecurrenceSeries{}, err
	}
	item.AnchorOn = anchor.Format(time.DateOnly)
	if ends.Valid {
		value := ends.Time.Format(time.DateOnly)
		item.EndsOn = &value
	}
	return item, nil
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func datePointer(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	date := dateOnly(*value)
	return &date
}

func nullableAllDaySpan(allDay bool, span int) *int {
	if !allDay {
		return nil
	}
	return &span
}

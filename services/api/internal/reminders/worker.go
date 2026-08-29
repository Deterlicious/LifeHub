package reminders

import (
	"context"
	"log/slog"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const (
	JobKind         = "lifehub_reminder_fire"
	QueueName       = "reminders"
	MaxAttempts     = 25
	MaxWorkers      = 4
	WorkerTimeout   = 10 * time.Second
	SoftStopTimeout = 15 * time.Second
)

// FireArgs deliberately contains no owner, source title, notes, or other
// private payload. The worker rehydrates and authorizes the immutable schedule
// generation from PostgreSQL immediately before creating a notification.
type FireArgs struct {
	ScheduleID string `json:"schedule_id"`
	Generation int64  `json:"generation"`
}

func (FireArgs) Kind() string { return JobKind }

type Processor interface {
	ProcessReminder(ctx context.Context, scheduleID string, generation int64) error
}

type Worker struct {
	river.WorkerDefaults[FireArgs]
	processor Processor
}

func NewWorker(processor Processor) *Worker {
	return &Worker{processor: processor}
}

func (worker *Worker) Timeout(*river.Job[FireArgs]) time.Duration {
	return WorkerTimeout
}

func (worker *Worker) Work(ctx context.Context, job *river.Job[FireArgs]) error {
	return worker.processor.ProcessReminder(ctx, job.Args.ScheduleID, job.Args.Generation)
}

func NewWorkers(processor Processor) *river.Workers {
	workers := river.NewWorkers()
	river.AddWorker(workers, NewWorker(processor))
	return workers
}

type FailureRecorder interface {
	InvalidateReminderJob(ctx context.Context, riverJobID int64) error
}

// ErrorHandler makes terminal retry exhaustion visible in LifeHub state. A
// startup reconciliation provides the same guarantee if PostgreSQL is
// unavailable at the exact moment this callback runs.
type ErrorHandler struct {
	recorder FailureRecorder
	logger   *slog.Logger
}

func NewErrorHandler(recorder FailureRecorder, logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{recorder: recorder, logger: logger}
}

func (handler *ErrorHandler) HandleError(ctx context.Context, job *rivertype.JobRow, _ error) *river.ErrorHandlerResult {
	handler.recordTerminalFailure(ctx, job)
	return nil
}

func (handler *ErrorHandler) HandlePanic(ctx context.Context, job *rivertype.JobRow, _ any, _ string) *river.ErrorHandlerResult {
	handler.recordTerminalFailure(ctx, job)
	return nil
}

func (handler *ErrorHandler) recordTerminalFailure(ctx context.Context, job *rivertype.JobRow) {
	if job.Kind != JobKind || job.Attempt < job.MaxAttempts {
		return
	}
	if err := handler.recorder.InvalidateReminderJob(ctx, job.ID); err != nil && handler.logger != nil {
		handler.logger.Error("record terminal reminder job failure", "river_job_id", job.ID, "error", err)
	}
}

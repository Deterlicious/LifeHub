package recurrence

import (
	"context"
	"time"

	"github.com/riverqueue/river"
)

const (
	SweepJobKind     = "lifehub_recurrence_materialize"
	SweepQueueName   = "maintenance"
	SweepPeriodicID  = "lifehub_recurrence_materialize_v1"
	SweepHorizonDays = 90
	SweepMaxAttempts = 10
	SweepMaxWorkers  = 1
	SweepTimeout     = 2 * time.Minute
	SweepInterval    = 12 * time.Hour
)

// SweepArgs deliberately has no private payload. The worker discovers active
// owned series from PostgreSQL at execution time.
type SweepArgs struct{}

func (SweepArgs) Kind() string { return SweepJobKind }

type SweepProcessor interface {
	MaterializeAllRecurrences(ctx context.Context, now time.Time, horizonDays int) error
}

type SweepWorker struct {
	river.WorkerDefaults[SweepArgs]
	processor SweepProcessor
	clock     func() time.Time
}

func NewSweepWorker(processor SweepProcessor, clock func() time.Time) *SweepWorker {
	if clock == nil {
		clock = time.Now
	}
	return &SweepWorker{processor: processor, clock: clock}
}

func (worker *SweepWorker) Timeout(*river.Job[SweepArgs]) time.Duration {
	return SweepTimeout
}

func (worker *SweepWorker) Work(ctx context.Context, _ *river.Job[SweepArgs]) error {
	return worker.processor.MaterializeAllRecurrences(ctx, worker.clock().UTC(), SweepHorizonDays)
}

func NewSweepPeriodicJob() *river.PeriodicJob {
	return river.NewPeriodicJob(
		river.PeriodicInterval(SweepInterval),
		func() (river.JobArgs, *river.InsertOpts) {
			return SweepArgs{}, &river.InsertOpts{Queue: SweepQueueName, MaxAttempts: SweepMaxAttempts}
		},
		&river.PeriodicJobOpts{ID: SweepPeriodicID, RunOnStart: true},
	)
}

package recurrence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/riverqueue/river"
)

type sweepProcessorFake struct {
	now         time.Time
	horizonDays int
	err         error
}

func (fake *sweepProcessorFake) MaterializeAllRecurrences(_ context.Context, now time.Time, horizonDays int) error {
	fake.now = now
	fake.horizonDays = horizonDays
	return fake.err
}

func TestSweepWorkerUsesBoundedRollingWindowAndPropagatesFailure(t *testing.T) {
	now := time.Date(2026, time.August, 27, 3, 4, 5, 0, time.UTC)
	wantErr := errors.New("database unavailable")
	processor := &sweepProcessorFake{err: wantErr}
	worker := NewSweepWorker(processor, func() time.Time { return now })
	err := worker.Work(context.Background(), &river.Job[SweepArgs]{Args: SweepArgs{}})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error=%v, want %v", err, wantErr)
	}
	if !processor.now.Equal(now) || processor.horizonDays != SweepHorizonDays {
		t.Fatalf("sweep now=%s horizon=%d", processor.now, processor.horizonDays)
	}
	if timeout := worker.Timeout(nil); timeout != SweepTimeout {
		t.Fatalf("timeout=%s, want %s", timeout, SweepTimeout)
	}
}

func TestSweepConstantsAreOperationallyBounded(t *testing.T) {
	if SweepInterval < time.Hour || SweepMaxWorkers != 1 || SweepMaxAttempts < 2 {
		t.Fatalf("unsafe sweep bounds: interval=%s workers=%d attempts=%d", SweepInterval, SweepMaxWorkers, SweepMaxAttempts)
	}
	if NewSweepPeriodicJob() == nil {
		t.Fatal("periodic sweep job is nil")
	}
}

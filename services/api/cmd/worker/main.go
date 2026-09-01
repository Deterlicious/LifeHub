package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"lifehub/services/api/internal/recurrence"
	"lifehub/services/api/internal/reminders"
	"lifehub/services/api/internal/riverinfra"
	"lifehub/services/api/internal/store"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Error("DATABASE_URL is required")
		os.Exit(1)
	}
	runtimeMode, err := parseRuntimeMode(os.Getenv("WORKER_RUN_DURATION"), os.Getenv("WORKER_PERIODIC_JOBS"))
	if err != nil {
		logger.Error("invalid worker runtime configuration", "error", err)
		os.Exit(1)
	}

	startupContext, startupCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer startupCancel()
	if err := riverinfra.Validate(startupContext, databaseURL); err != nil {
		logger.Error("River schema is not at the pinned target; run cmd/migrate first", "error", err)
		os.Exit(1)
	}
	storage, err := store.Open(startupContext, databaseURL)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer storage.Close()
	if err := storage.Ready(startupContext); err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	if err := storage.ReconcileTerminalReminderSchedules(startupContext); err != nil {
		logger.Error("terminal reminder reconciliation failed", "error", err)
		os.Exit(1)
	}

	workers := river.NewWorkers()
	river.AddWorker(workers, reminders.NewWorker(storage))
	river.AddWorker(workers, recurrence.NewSweepWorker(storage, nil))
	var periodicJobs []*river.PeriodicJob
	if runtimeMode.periodicJobs {
		periodicJobs = []*river.PeriodicJob{recurrence.NewSweepPeriodicJob()}
	}
	client, err := river.NewClient(riverpgxv5.New(storage.Pool()), &river.Config{
		Schema: riverinfra.Schema,
		Queues: map[string]river.QueueConfig{
			reminders.QueueName:       {MaxWorkers: reminders.MaxWorkers},
			recurrence.SweepQueueName: {MaxWorkers: recurrence.SweepMaxWorkers},
		},
		Workers:         workers,
		PeriodicJobs:    periodicJobs,
		MaxAttempts:     reminders.MaxAttempts,
		JobTimeout:      reminders.WorkerTimeout,
		SoftStopTimeout: reminders.SoftStopTimeout,
		ErrorHandler:    reminders.NewErrorHandler(storage, logger),
		Logger:          logger,
	})
	if err != nil {
		logger.Error("River worker initialization failed", "error", err)
		os.Exit(1)
	}
	workerContext := context.Background()
	if err := client.Start(workerContext); err != nil {
		logger.Error("River worker failed to start", "error", err)
		os.Exit(1)
	}
	logger.Info("background worker started",
		"reminder_queue", reminders.QueueName,
		"reminder_max_workers", reminders.MaxWorkers,
		"maintenance_queue", recurrence.SweepQueueName,
		"maintenance_max_workers", recurrence.SweepMaxWorkers,
		"run_duration", runtimeMode.runDuration,
		"periodic_jobs", runtimeMode.periodicJobs,
	)

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	if runtimeMode.runDuration > 0 {
		timer := time.NewTimer(runtimeMode.runDuration)
		select {
		case <-shutdownSignal:
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			logger.Info("bounded worker window completed")
		}
	} else {
		<-shutdownSignal
	}
	signal.Stop(shutdownSignal)
	logger.Info("reminder worker shutdown requested")

	if err := stopWorker(client, reminders.SoftStopTimeout+5*time.Second, 5*time.Second); err != nil {
		logger.Error("reminder worker graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	<-client.Stopped()
	logger.Info("reminder worker stopped")
}

type workerStopper interface {
	Stop(ctx context.Context) error
	StopAndCancel(ctx context.Context) error
}

func stopWorker(client workerStopper, softTimeout, hardTimeout time.Duration) error {
	softContext, softCancel := context.WithTimeout(context.Background(), softTimeout)
	defer softCancel()
	if err := client.Stop(softContext); err != nil {
		hardContext, hardCancel := context.WithTimeout(context.Background(), hardTimeout)
		defer hardCancel()
		if hardErr := client.StopAndCancel(hardContext); hardErr != nil {
			return errors.Join(fmt.Errorf("soft stop: %w", err), fmt.Errorf("hard stop: %w", hardErr))
		}
		return fmt.Errorf("soft stop required hard cancellation: %w", err)
	}
	return nil
}

// Keep the transaction type assertion compile-time visible beside worker
// construction. River's pgx driver must remain on pgx/v5 transactions.
var _ *river.Client[pgx.Tx]

const (
	minWorkerRunDuration = 5 * time.Second
	maxWorkerRunDuration = 5 * time.Minute
)

type runtimeMode struct {
	runDuration  time.Duration
	periodicJobs bool
}

func parseRuntimeMode(runDurationValue, periodicJobsValue string) (runtimeMode, error) {
	mode := runtimeMode{periodicJobs: true}
	if value := strings.TrimSpace(runDurationValue); value != "" {
		runDuration, err := time.ParseDuration(value)
		if err != nil {
			return runtimeMode{}, fmt.Errorf("WORKER_RUN_DURATION: %w", err)
		}
		if runDuration < minWorkerRunDuration || runDuration > maxWorkerRunDuration {
			return runtimeMode{}, fmt.Errorf("WORKER_RUN_DURATION must be between %s and %s", minWorkerRunDuration, maxWorkerRunDuration)
		}
		mode.runDuration = runDuration
	}
	if value := strings.TrimSpace(periodicJobsValue); value != "" {
		periodicJobs, err := strconv.ParseBool(value)
		if err != nil {
			return runtimeMode{}, fmt.Errorf("WORKER_PERIODIC_JOBS must be true or false")
		}
		mode.periodicJobs = periodicJobs
	}
	return mode, nil
}

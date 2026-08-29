package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
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
	client, err := river.NewClient(riverpgxv5.New(storage.Pool()), &river.Config{
		Schema: riverinfra.Schema,
		Queues: map[string]river.QueueConfig{
			reminders.QueueName:       {MaxWorkers: reminders.MaxWorkers},
			recurrence.SweepQueueName: {MaxWorkers: recurrence.SweepMaxWorkers},
		},
		Workers: workers,
		PeriodicJobs: []*river.PeriodicJob{
			recurrence.NewSweepPeriodicJob(),
		},
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
	)

	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, os.Interrupt, syscall.SIGTERM)
	<-shutdownSignal
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

package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lifehub/services/api/internal/application"
	"lifehub/services/api/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	settings, err := config.Load()
	if err != nil {
		logger.Error("configuration invalid", "error", err)
		os.Exit(1)
	}

	rootContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	app, err := application.New(rootContext, settings, logger)
	if err != nil {
		if settings.Production() {
			logger.Error("API initialization failed", "stage", application.Stage(err))
		} else {
			logger.Error("API initialization failed", "stage", application.Stage(err), "error", err)
		}
		os.Exit(1)
	}
	defer app.Close()

	server := &http.Server{
		Addr:              settings.HTTPAddr,
		Handler:           app.Handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API listening", "address", settings.HTTPAddr, "environment", settings.AppEnv)
		serverErrors <- server.ListenAndServe()
	}()

	failed := false
	select {
	case <-rootContext.Done():
		logger.Info("shutdown requested")
	case serveErr := <-serverErrors:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("API server failed", "error", serveErr)
			failed = true
		}
		stop()
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		_ = server.Close()
		os.Exit(1)
	}
	logger.Info("API stopped")
	if failed {
		os.Exit(1)
	}
}

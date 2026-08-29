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

	"lifehub/services/api/internal/auth"
	"lifehub/services/api/internal/config"
	"lifehub/services/api/internal/httpapi"
	"lifehub/services/api/internal/riverinfra"
	"lifehub/services/api/internal/store"
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
	validationContext, validationCancel := context.WithTimeout(rootContext, 15*time.Second)
	if err := riverinfra.Validate(validationContext, settings.DatabaseURL); err != nil {
		validationCancel()
		logger.Error("River schema is not at the pinned target; run cmd/migrate first", "error", err)
		os.Exit(1)
	}
	validationCancel()

	storage, err := store.Open(rootContext, settings.DatabaseURL)
	if err != nil {
		logger.Error("database initialization failed", "error", err)
		os.Exit(1)
	}
	defer storage.Close()
	pingContext, pingCancel := context.WithTimeout(rootContext, 10*time.Second)
	err = storage.Ready(pingContext)
	pingCancel()
	if err != nil {
		logger.Error("database unavailable", "error", err)
		os.Exit(1)
	}

	var (
		verifier  auth.Verifier
		devIssuer httpapi.DevTokenIssuer
	)
	if settings.Production() {
		jwksVerifier, verifyErr := auth.NewJWKSVerifier(
			rootContext,
			settings.SupabaseJWKSURL,
			settings.SupabaseIssuer,
			settings.SupabaseAudience,
		)
		if verifyErr != nil {
			logger.Error("JWKS initialization failed", "error", verifyErr)
			os.Exit(1)
		}
		verifier = jwksVerifier
	} else {
		devAuth, devErr := auth.NewDevAuth(settings.DevAuthSecret)
		if devErr != nil {
			logger.Error("development auth initialization failed", "error", devErr)
			os.Exit(1)
		}
		verifier = devAuth
		devIssuer = devAuth
	}

	server := &http.Server{
		Addr: settings.HTTPAddr,
		Handler: httpapi.New(httpapi.Options{
			Store: storage, Verifier: verifier, DevIssuer: devIssuer,
			WebOrigins: settings.WebOrigins, Logger: logger, Production: settings.Production(),
		}),
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

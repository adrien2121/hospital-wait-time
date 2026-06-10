package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/adrien2121/GoProject/internal/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := buildLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := openAPIStorage(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}
	defer store.Close()

	hospitalSvc, waitTimeSvc := buildServices(store.hospitalRepo, store.waitTimeRepo, store.externalSignalRepo)

	srv := buildServer(cfg.APIAddr, buildMux(hospitalSvc, waitTimeSvc, store.db, logger))

	srvErr := make(chan error, 1)
	go func() {
		logger.Info("api server starting", "addr", cfg.APIAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	select {
	case err := <-srvErr:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}
	stop()
	logger.Info("shutting down")

	// shutdownCtx is rooted in context.Background, not ctx: ctx is already cancelled by the time
	// we reach this line (we got here via <-ctx.Done()), so inheriting would abort shutdown immediately.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

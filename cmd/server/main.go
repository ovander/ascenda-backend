package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	sentry "github.com/getsentry/sentry-go"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"ascenda/internal/config"
)

// shutdownTimeout is the maximum time we allow in-flight HTTP requests to
// complete before the listener is forcibly closed.
const shutdownTimeout = 30 * time.Second

func main() {
	// Load .env file if present (does not override existing env vars)
	if err := godotenv.Load(); err != nil {
		fmt.Println("No .env file found, using environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// ── Sub-command: migrate ──────────────────────────────────────────────────
	// Usage: ./kerplan-api migrate
	//
	// Applies all pending SQL migrations using the embedded migration files and
	// exits immediately — no HTTP server is started. Designed for use in deploy
	// scripts before the server process is started:
	//
	//   $BIN_PATH migrate    # exit 0 on success, 1 on error (triggers set -e)
	//   $BIN_PATH            # start the HTTP server normally
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		runMigrateCommand(cfg.DatabaseURL)
		return // runMigrateCommand calls os.Exit; return keeps the linter happy
	}

	// Bootstrap the application (connects DB, wires repos/services/handlers/router)
	resources, err := Bootstrap(cfg)
	if err != nil {
		// Bootstrap logs its own errors; we only need a final fatal here.
		fmt.Fprintf(os.Stderr, "bootstrap failed: %v\n", err)
		os.Exit(1)
	}

	shutLog := resources.Logger.WithField("phase", "shutdown")

	// Start HTTP server in background; surface fatal errors on a channel.
	serverErr := make(chan error, 1)
	go func() {
		resources.Logger.WithFields(logrus.Fields{
			"phase": "server",
			"port":  cfg.Port,
		}).Info("listening for requests")
		if err := resources.Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
		close(serverErr)
	}()

	// Block until SIGINT / SIGTERM, or an unrecoverable server error.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		shutLog.WithField("signal", sig.String()).Info("received shutdown signal")
	case err := <-serverErr:
		if err != nil {
			resources.Logger.WithError(err).Fatal("unrecoverable server error — forcing exit")
		}
		// Listener was closed without a signal (e.g. test teardown); exit cleanly.
		return
	}

	exitCode := 0

	// ── Step 1: stop accepting new connections; drain in-flight requests ─────
	//
	// Shutdown(ctx) stops the listener immediately and then waits for all
	// active connections to reach an idle state, up to the timeout.
	// Unlike Close(), it does NOT interrupt keep-alive connections mid-request.
	shutLog.Infof("waiting up to %s for in-flight requests to complete", shutdownTimeout)
	shutCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := resources.Server.Shutdown(shutCtx); err != nil {
		shutLog.WithError(err).Error("HTTP shutdown did not complete cleanly within timeout")
		exitCode = 1
	} else {
		shutLog.Info("HTTP server stopped — all in-flight requests drained")
	}

	// ── Step 2a: drain in-flight AI usage recording goroutines ───────────────
	//
	// AIUsageRecorder fires goroutines with context.Background() so they
	// intentionally outlive the HTTP request. They must finish before we
	// close the DB pool or the event emitter (both of which they may write
	// to). Wait() blocks until the last goroutine calls wg.Done().
	if resources.AIAccessMW != nil {
		shutLog.Info("waiting for AI usage recording goroutines to finish")
		resources.AIAccessMW.Wait()
		shutLog.Info("AI usage recording goroutines finished")
	}

	// ── Step 2b: drain the async event emitter ───────────────────────────────
	//
	// Must happen AFTER the HTTP server stops so that no new events are
	// published (by request handlers) while we are flushing the queue.
	// emitter.Close() closes the async channel and calls wg.Wait(), which
	// blocks until all async subscribers (audit logger, cache invalidator)
	// have processed their pending events.
	if resources.Emitter != nil {
		shutLog.Info("draining event emitter (flushing audit log and cache invalidation)")
		resources.Emitter.Close()
		shutLog.Info("event emitter drained")
	}

	// ── Step 3: close the database connection pool ────────────────────────────
	//
	// Closing the pool sends connection-close notices to PostgreSQL and
	// releases file descriptors. Must happen last so that the emitter's
	// async subscribers (which may write to the DB) finish first.
	if resources.DB != nil {
		sqlDB, err := resources.DB.DB()
		if err != nil {
			shutLog.WithError(err).Error("failed to retrieve underlying sql.DB")
			exitCode = 1
		} else if err := sqlDB.Close(); err != nil {
			shutLog.WithError(err).Error("failed to close database connection pool")
			exitCode = 1
		} else {
			shutLog.Info("database connection pool closed")
		}
	}

	// ── Step 4: flush Sentry ──────────────────────────────────────────────────
	//
	// Sentry batches events; Flush blocks until the internal queue is empty or
	// the timeout is reached. Must happen last so all errors are captured first.
	sentry.Flush(2 * time.Second)

	shutLog.WithField("exit_code", exitCode).Info("shutdown complete")
	os.Exit(exitCode)
}

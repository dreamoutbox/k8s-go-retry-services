package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"service-b/internal/handler"
	"service-b/internal/worker"
)

// getEnvInt parses an integer environment variable or returns the fallback value.
//
// key: environment variable name.
// fallback: default value if key is not set or invalid.
//
// Example usage:
//
//	val := getEnvInt("FAIL_COUNT", 2)
func getEnvInt(key string, fallback int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return fallback
	}
	return val
}

// getEnvString retrieves an environment variable or returns the fallback value.
//
// key: environment variable name.
// fallback: default value if key is not set.
//
// Example usage:
//
//	port := getEnvString("PORT", "8080")
func getEnvString(key string, fallback string) string {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	return val
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	failCount := getEnvInt("FAIL_COUNT", 2)
	port := getEnvString("PORT", "8080")

	workerService := worker.NewService(failCount)
	h := handler.New(workerService, logger)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("starting service-b",
			slog.String("port", port),
			slog.Int("fail_count", failCount),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down service-b gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", slog.String("error", err.Error()))
	}

	logger.Info("service-b stopped")
}

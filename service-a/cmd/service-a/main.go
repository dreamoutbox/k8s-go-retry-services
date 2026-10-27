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

	"service-a/internal/handler"
	"service-a/internal/retry"
)

// getEnvDuration parses a duration environment variable string or returns the fallback duration.
//
// key: environment variable name.
// fallback: default duration if variable is unset or cannot be parsed.
//
// Example usage:
//
//	d := getEnvDuration("RETRY_BASE_DELAY", 100*time.Millisecond)
func getEnvDuration(key string, fallback time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return fallback
	}
	d, err := time.ParseDuration(valStr)
	if err != nil {
		return fallback
	}
	return d
}

// getEnvInt parses an integer environment variable or returns the fallback value.
//
// key: environment variable name.
// fallback: default integer if variable is unset or cannot be parsed.
//
// Example usage:
//
//	attempts := getEnvInt("RETRY_MAX_ATTEMPTS", 5)
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
// fallback: default string if key is unset.
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

	port := getEnvString("PORT", "8080")
	serviceBURL := getEnvString("SERVICE_B_URL", "http://service-b.retry-demo.svc.cluster.local:8080")

	retryCfg := retry.Config{
		MaxAttempts:    getEnvInt("RETRY_MAX_ATTEMPTS", 5),
		BaseDelay:      getEnvDuration("RETRY_BASE_DELAY", 100*time.Millisecond),
		MaxDelay:       getEnvDuration("RETRY_MAX_DELAY", 2*time.Second),
		AttemptTimeout: getEnvDuration("RETRY_ATTEMPT_TIMEOUT", 3*time.Second),
		TotalTimeout:   getEnvDuration("RETRY_TOTAL_TIMEOUT", 15*time.Second),
	}

	httpClient := &http.Client{
		Timeout: 0, // Individual attempt timeout managed by context in retry.Client
	}

	retryClient := retry.NewClient(httpClient, retryCfg, nil, logger)
	h := handler.New(retryClient, serviceBURL, logger)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           h.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		logger.Info("starting service-a",
			slog.String("port", port),
			slog.String("service_b_url", serviceBURL),
			slog.Int("max_attempts", retryCfg.MaxAttempts),
			slog.Duration("base_delay", retryCfg.BaseDelay),
			slog.Duration("max_delay", retryCfg.MaxDelay),
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down service-a gracefully")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced to shutdown", slog.String("error", err.Error()))
	}
	logger.Info("service-a stopped")
}

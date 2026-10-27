package retry_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"service-a/internal/retry"
)

// noOpSleeper returns nil immediately for fast test execution.
func noOpSleeper(_ context.Context, _ time.Duration) error {
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestExecuteWorkSucceedsOnNthAttempt verifies that retries continue until success.
func TestExecuteWorkSucceedsOnNthAttempt(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := count.Add(1)
		if attempt < 3 {
			http.Error(w, "temporary failure", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"OK"}`))
	}))
	defer srv.Close()

	cfg := retry.Config{
		MaxAttempts:    5,
		BaseDelay:      10 * time.Millisecond,
		MaxDelay:       50 * time.Millisecond,
		AttemptTimeout: 1 * time.Second,
		TotalTimeout:   5 * time.Second,
	}

	client := retry.NewClient(srv.Client(), cfg, noOpSleeper, discardLogger())
	res := client.ExecuteWork(context.Background(), srv.URL)

	if res.Outcome != retry.OutcomeSuccess {
		t.Fatalf("expected success, got %s: %s", res.Outcome, res.ErrorMessage)
	}
	if res.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", res.Attempts)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", res.StatusCode)
	}
}

// TestExecuteWorkExhaustsAttempts verifies failure when all retries return 500 errors.
func TestExecuteWorkExhaustsAttempts(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := retry.Config{
		MaxAttempts:  3,
		BaseDelay:    10 * time.Millisecond,
		MaxDelay:     20 * time.Millisecond,
		TotalTimeout: 2 * time.Second,
	}

	client := retry.NewClient(srv.Client(), cfg, noOpSleeper, discardLogger())
	res := client.ExecuteWork(context.Background(), srv.URL)

	if res.Outcome != retry.OutcomeFailure {
		t.Fatalf("expected failure, got %s", res.Outcome)
	}
	if res.Attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", res.Attempts)
	}
	if int(count.Load()) != 3 {
		t.Fatalf("expected server to be hit 3 times, got %d", count.Load())
	}
}

// TestExecuteWorkDoesNotRetryOn4xx verifies that client errors (4xx) abort retry immediately.
func TestExecuteWorkDoesNotRetryOn4xx(t *testing.T) {
	var count atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	cfg := retry.Config{
		MaxAttempts: 4,
		BaseDelay:   10 * time.Millisecond,
		MaxDelay:    50 * time.Millisecond,
	}

	client := retry.NewClient(srv.Client(), cfg, noOpSleeper, discardLogger())
	res := client.ExecuteWork(context.Background(), srv.URL)

	if res.Outcome != retry.OutcomeFailure {
		t.Fatalf("expected failure, got %s", res.Outcome)
	}
	if res.Attempts != 1 {
		t.Fatalf("expected exactly 1 attempt on 4xx, got %d", res.Attempts)
	}
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.StatusCode)
	}
}

// TestExecuteWorkRespectsContextCancellation verifies that cancelled context aborts backoff.
func TestExecuteWorkRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fail", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := retry.Config{
		MaxAttempts: 5,
		BaseDelay:   100 * time.Millisecond,
		MaxDelay:    500 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel when sleeping on first retry
	cancellableSleeper := func(sCtx context.Context, _ time.Duration) error {
		cancel()
		return sCtx.Err()
	}

	client := retry.NewClient(srv.Client(), cfg, cancellableSleeper, discardLogger())
	res := client.ExecuteWork(ctx, srv.URL)

	if res.Outcome != retry.OutcomeFailure {
		t.Fatalf("expected failure on cancelled context, got %s", res.Outcome)
	}
	if res.Attempts != 1 {
		t.Fatalf("expected 1 attempt before cancellation, got %d", res.Attempts)
	}
}

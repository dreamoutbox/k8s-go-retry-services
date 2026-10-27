package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"service-a/internal/handler"
	"service-a/internal/retry"
)

func noOpSleeper(_ context.Context, _ time.Duration) error {
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestHandlerHealthz verifies that GET /healthz returns 200 OK.
func TestHandlerHealthz(t *testing.T) {
	cfg := retry.Config{MaxAttempts: 1}
	client := retry.NewClient(http.DefaultClient, cfg, noOpSleeper, discardLogger())
	h := handler.New(client, "http://localhost:8082", discardLogger())

	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}

// TestHandlerTriggerSuccess verifies /trigger completes and reports successful outcome.
func TestHandlerTriggerSuccess(t *testing.T) {
	// Mock Service B
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"SUCCESS"}`))
	}))
	defer srvB.Close()

	cfg := retry.Config{MaxAttempts: 3}
	client := retry.NewClient(srvB.Client(), cfg, noOpSleeper, discardLogger())
	h := handler.New(client, srvB.URL, discardLogger())

	srvA := httptest.NewServer(h.Routes())
	defer srvA.Close()

	resp, err := http.Post(srvA.URL+"/trigger", "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	var triggerResp handler.TriggerResponse
	if err := json.NewDecoder(resp.Body).Decode(&triggerResp); err != nil {
		t.Fatalf("failed decoding trigger response: %v", err)
	}

	if triggerResp.Outcome != retry.OutcomeSuccess || triggerResp.Attempts != 1 {
		t.Fatalf("unexpected trigger response: %+v", triggerResp)
	}
}

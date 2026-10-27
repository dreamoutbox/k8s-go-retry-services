package handler_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"service-b/internal/handler"
	"service-b/internal/worker"
)

// TestHandlerHealthz verifies that GET /healthz returns 200 OK.
func TestHandlerHealthz(t *testing.T) {
	svc := worker.NewService(2)
	h := handler.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
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

// TestHandlerWorkSequenceAndReset verifies /work deterministic status code sequence and /reset behavior.
func TestHandlerWorkSequenceAndReset(t *testing.T) {
	svc := worker.NewService(2)
	h := handler.New(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	srv := httptest.NewServer(h.Routes())
	defer srv.Close()

	// Request 1: should return 503
	resp1, err := http.Post(srv.URL+"/work", "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error request 1: %v", err)
	}
	if resp1.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp1.StatusCode)
	}
	var workResp1 handler.WorkResponse
	if err := json.NewDecoder(resp1.Body).Decode(&workResp1); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	resp1.Body.Close()
	if workResp1.Status != worker.OutcomeFailed || workResp1.RequestNumber != 1 {
		t.Fatalf("unexpected response payload: %+v", workResp1)
	}

	// Request 2: should return 503
	resp2, err := http.Post(srv.URL+"/work", "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error request 2: %v", err)
	}
	if resp2.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", resp2.StatusCode)
	}
	resp2.Body.Close()

	// Request 3: should return 200
	resp3, err := http.Post(srv.URL+"/work", "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error request 3: %v", err)
	}
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}
	var workResp3 handler.WorkResponse
	if err := json.NewDecoder(resp3.Body).Decode(&workResp3); err != nil {
		t.Fatalf("failed decoding response: %v", err)
	}
	resp3.Body.Close()
	if workResp3.Status != worker.OutcomeSuccess || workResp3.RequestNumber != 3 {
		t.Fatalf("unexpected response payload: %+v", workResp3)
	}

	// Reset counter
	resetResp, err := http.Post(srv.URL+"/reset", "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error reset: %v", err)
	}
	if resetResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on reset, got %d", resetResp.StatusCode)
	}
	resetResp.Body.Close()

	// Next request should fail (request 1 after reset)
	respAfterReset, err := http.Post(srv.URL+"/work", "application/json", nil)
	if err != nil {
		t.Fatalf("unexpected error request after reset: %v", err)
	}
	defer respAfterReset.Body.Close()
	if respAfterReset.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 after reset, got %d", respAfterReset.StatusCode)
	}
}

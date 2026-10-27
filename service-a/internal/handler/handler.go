package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"service-a/internal/retry"
)

// TriggerResponse represents the JSON response returned by POST /trigger.
type TriggerResponse struct {
	Outcome      retry.Outcome `json:"outcome"`
	Attempts     int           `json:"attempts"`
	StatusCode   int           `json:"status_code"`
	DurationMs   int64         `json:"duration_ms"`
	Message      string        `json:"message"`
	ResponseBody string        `json:"response_body,omitempty"`
	Error        string        `json:"error,omitempty"`
}

// WorkClient abstracts the retry client interface for testability.
type WorkClient interface {
	ExecuteWork(ctx contextContext, targetURL string) retry.Result
}

// contextContext is an alias to avoid importing context in interface if desired or use context.Context directly.
type contextContext = interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}

// Handler handles HTTP requests for Service A.
type Handler struct {
	client        *retry.Client
	serviceBURL   string
	logger        *slog.Logger
}

// New constructs an HTTP Handler for Service A.
//
// client: retry client configured to call Service B.
// serviceBURL: base URL for Service B (e.g. http://service-b:8080).
// logger: structured logger.
//
// Example usage:
//
//	h := handler.New(client, "http://service-b:8080", slog.Default())
//	mux := h.Routes()
func New(client *retry.Client, serviceBURL string, logger *slog.Logger) *Handler {
	trimmedURL := strings.TrimRight(serviceBURL, "/")
	return &Handler{
		client:      client,
		serviceBURL: trimmedURL,
		logger:      logger,
	}
}

// Routes sets up the HTTP multiplexer with routes for health and trigger.
//
// Example usage:
//
//	mux := h.Routes()
//	http.ListenAndServe(":8080", mux)
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("POST /trigger", h.handleTrigger)
	return mux
}

// handleHealthz responds with HTTP 200 for health checks.
func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleTrigger executes the retry workflow against Service B's /work endpoint.
func (h *Handler) handleTrigger(w http.ResponseWriter, r *http.Request) {
	targetURL := fmt.Sprintf("%s/work", h.serviceBURL)
	h.logger.Info("triggering work on service-b", slog.String("target_url", targetURL))

	res := h.client.ExecuteWork(r.Context(), targetURL)

	w.Header().Set("Content-Type", "application/json")
	if res.Outcome == retry.OutcomeFailure {
		w.WriteHeader(http.StatusBadGateway)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	msg := fmt.Sprintf("work finished with outcome: %s after %d attempt(s)", res.Outcome, res.Attempts)
	_ = json.NewEncoder(w).Encode(TriggerResponse{
		Outcome:      res.Outcome,
		Attempts:     res.Attempts,
		StatusCode:   res.StatusCode,
		DurationMs:   res.Duration.Milliseconds(),
		Message:      msg,
		ResponseBody: res.ResponseBody,
		Error:        res.ErrorMessage,
	})
}

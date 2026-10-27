package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"service-b/internal/worker"
)

// WorkResponse represents the response payload for /work requests.
type WorkResponse struct {
	Status        worker.Outcome `json:"status"`
	RequestNumber int            `json:"request_number"`
	FailCount     int            `json:"fail_count"`
	Message       string         `json:"message"`
}

// ResetResponse represents the response payload for /reset requests.
type ResetResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Handler holds the HTTP route handlers for Service B.
type Handler struct {
	worker *worker.Service
	logger *slog.Logger
}

// New constructs an HTTP Handler for Service B with the provided worker service and logger.
//
// worker: worker service handling counter and failure sequence.
// logger: structured slog logger instance.
//
// Example usage:
//
//	svc := worker.NewService(2)
//	h := handler.New(svc, slog.Default())
//	mux := h.Routes()
func New(w *worker.Service, logger *slog.Logger) *Handler {
	return &Handler{
		worker: w,
		logger: logger,
	}
}

// Routes registers the HTTP routes and returns an http.Handler.
//
// Example usage:
//
//	mux := h.Routes()
//	http.ListenAndServe(":8080", mux)
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealthz)
	mux.HandleFunc("POST /work", h.handleWork)
	mux.HandleFunc("POST /reset", h.handleReset)
	return mux
}

// handleHealthz responds with HTTP 200 for health checks.
func (h *Handler) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("OK"))
}

// handleWork handles incoming work requests and applies deterministic failure.
func (h *Handler) handleWork(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	res := h.worker.Process()
	latency := time.Since(start)

	h.logger.Info("processed work request",
		slog.Int("request_number", res.RequestNumber),
		slog.String("outcome", string(res.Outcome)),
		slog.Duration("latency", latency),
	)

	w.Header().Set("Content-Type", "application/json")

	if res.Outcome == worker.OutcomeFailed {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(WorkResponse{
			Status:        res.Outcome,
			RequestNumber: res.RequestNumber,
			FailCount:     res.FailCount,
			Message:       "simulated service failure",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(WorkResponse{
		Status:        res.Outcome,
		RequestNumber: res.RequestNumber,
		FailCount:     res.FailCount,
		Message:       "work completed successfully",
	})
}

// handleReset resets the deterministic failure sequence.
func (h *Handler) handleReset(w http.ResponseWriter, r *http.Request) {
	h.worker.Reset()
	h.logger.Info("reset worker counter to zero")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ResetResponse{
		Status:  "RESET",
		Message: "counter reset to 0",
	})
}

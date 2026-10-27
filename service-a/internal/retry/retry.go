package retry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"
)

// Outcome represents the final status outcome of an execution attempt.
type Outcome string

const (
	// OutcomeSuccess indicates the operation completed with HTTP 2xx.
	OutcomeSuccess Outcome = "SUCCESS"
	// OutcomeFailure indicates the operation failed or exhausted all retries.
	OutcomeFailure Outcome = "FAILURE"
)

// Config holds retry and backoff parameters.
type Config struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	AttemptTimeout time.Duration
	TotalTimeout   time.Duration
}

// Sleeper abstracts sleep delays for deterministic unit testing.
type Sleeper func(ctx context.Context, d time.Duration) error

// defaultSleeper sleeps for the duration d or returns early if ctx is cancelled.
func defaultSleeper(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// CalculateBackoff computes exponential backoff with full jitter for attempt n (1-indexed).
//
// attempt: current 1-based attempt index (e.g. 1, 2, 3).
// baseDelay: minimum base duration.
// maxDelay: upper bound duration cap.
//
// Example usage:
//
//	delay := retry.CalculateBackoff(2, 100*time.Millisecond, 2*time.Second)
func CalculateBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	if attempt <= 1 {
		if baseDelay <= 0 {
			return 0
		}
		return time.Duration(rand.Int64N(int64(baseDelay)))
	}

	shift := min(attempt-1, 30)
	multiplier := int64(1) << shift
	calculated := time.Duration(int64(baseDelay) * multiplier)

	ceiling := min(calculated, maxDelay)
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(ceiling)))
}

// IsRetryable determines if an HTTP response code or error should trigger a retry attempt.
//
// resp: optional HTTP response from the attempt.
// err: optional network or protocol error from the attempt.
//
// Example usage:
//
//	if retry.IsRetryable(resp, err) { ... }
func IsRetryable(resp *http.Response, err error) bool {
	if err != nil {
		return !errors.Is(err, context.Canceled)
	}

	if resp != nil {
		if resp.StatusCode >= 500 && resp.StatusCode <= 599 {
			return true
		}
		// 4xx status codes are client errors and are not retried.
		return false
	}

	return false
}

// Result represents the aggregate outcome of executing an HTTP request with retries.
type Result struct {
	Outcome      Outcome       `json:"outcome"`
	Attempts     int           `json:"attempts"`
	StatusCode   int           `json:"status_code"`
	Duration     time.Duration `json:"duration"`
	ResponseBody string        `json:"response_body,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
}

// Client wraps an HTTP client and retry configuration to invoke Service B.
type Client struct {
	httpClient *http.Client
	config     Config
	sleeper    Sleeper
	logger     *slog.Logger
}

// NewClient constructs a Client with retry policy, optional custom sleeper, and logger.
//
// httpClient: standard HTTP client used for individual requests.
// cfg: retry and backoff configuration parameters.
// sleeper: optional sleep function; if nil, default timer-based sleeper is used.
// logger: structured logger for logging attempts and backoffs.
//
// Example usage:
//
//	cfg := retry.Config{MaxAttempts: 3, BaseDelay: 100*time.Millisecond, MaxDelay: time.Second}
//	client := retry.NewClient(http.DefaultClient, cfg, nil, slog.Default())
func NewClient(httpClient *http.Client, cfg Config, sleeper Sleeper, logger *slog.Logger) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if sleeper == nil {
		sleeper = defaultSleeper
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		httpClient: httpClient,
		config:     cfg,
		sleeper:    sleeper,
		logger:     logger,
	}
}

// ExecuteWork calls the target URL with POST and applies retry + exponential backoff logic.
//
// parentCtx: root context controlling overall request lifetime.
// targetURL: full URL to call (e.g. http://service-b:8080/work).
//
// Example usage:
//
//	res := client.ExecuteWork(ctx, "http://service-b:8080/work")
//	if res.Outcome == retry.OutcomeSuccess { ... }
func (c *Client) ExecuteWork(parentCtx context.Context, targetURL string) Result {
	start := time.Now()

	ctx := parentCtx
	var cancel context.CancelFunc
	if c.config.TotalTimeout > 0 {
		ctx, cancel = context.WithTimeout(parentCtx, c.config.TotalTimeout)
		defer cancel()
	}

	var lastStatusCode int
	var lastErr error
	var lastBody string

	for attempt := 1; attempt <= c.config.MaxAttempts; attempt++ {
		attemptStart := time.Now()

		attemptCtx := ctx
		var attemptCancel context.CancelFunc
		if c.config.AttemptTimeout > 0 {
			attemptCtx, attemptCancel = context.WithTimeout(ctx, c.config.AttemptTimeout)
		}

		req, err := http.NewRequestWithContext(attemptCtx, http.MethodPost, targetURL, nil)
		if err != nil {
			if attemptCancel != nil {
				attemptCancel()
			}
			return Result{
				Outcome:      OutcomeFailure,
				Attempts:     attempt,
				Duration:     time.Since(start),
				ErrorMessage: fmt.Sprintf("failed creating request: %v", err),
			}
		}

		resp, reqErr := c.httpClient.Do(req)
		attemptLatency := time.Since(attemptStart)

		if reqErr != nil {
			lastErr = reqErr
			lastStatusCode = 0
			c.logger.Warn("request failed",
				slog.Int("attempt", attempt),
				slog.Duration("latency", attemptLatency),
				slog.String("error", reqErr.Error()),
			)
		} else {
			lastStatusCode = resp.StatusCode
			bodyBytes, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastBody = string(bodyBytes)

			c.logger.Info("received response",
				slog.Int("attempt", attempt),
				slog.Int("status_code", resp.StatusCode),
				slog.Duration("latency", attemptLatency),
			)

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				if attemptCancel != nil {
					attemptCancel()
				}
				return Result{
					Outcome:      OutcomeSuccess,
					Attempts:     attempt,
					StatusCode:   resp.StatusCode,
					Duration:     time.Since(start),
					ResponseBody: lastBody,
				}
			}
		}

		if attemptCancel != nil {
			attemptCancel()
		}

		// Check if error or status is retryable.
		var dummyResp *http.Response
		if lastStatusCode > 0 {
			dummyResp = &http.Response{StatusCode: lastStatusCode}
		}
		if !IsRetryable(dummyResp, lastErr) {
			errMsg := "non-retryable failure"
			if lastErr != nil {
				errMsg = lastErr.Error()
			}
			return Result{
				Outcome:      OutcomeFailure,
				Attempts:     attempt,
				StatusCode:   lastStatusCode,
				Duration:     time.Since(start),
				ResponseBody: lastBody,
				ErrorMessage: errMsg,
			}
		}

		// If this was the last allowed attempt, do not backoff.
		if attempt == c.config.MaxAttempts {
			break
		}

		backoff := CalculateBackoff(attempt, c.config.BaseDelay, c.config.MaxDelay)
		c.logger.Info("backing off before retry",
			slog.Int("attempt", attempt),
			slog.Duration("backoff", backoff),
		)

		if err := c.sleeper(ctx, backoff); err != nil {
			return Result{
				Outcome:      OutcomeFailure,
				Attempts:     attempt,
				StatusCode:   lastStatusCode,
				Duration:     time.Since(start),
				ResponseBody: lastBody,
				ErrorMessage: fmt.Sprintf("retry aborted by context: %v", err),
			}
		}
	}

	errMsg := "max retry attempts exhausted"
	if lastErr != nil {
		errMsg = fmt.Sprintf("retries exhausted: %v", lastErr)
	}

	return Result{
		Outcome:      OutcomeFailure,
		Attempts:     c.config.MaxAttempts,
		StatusCode:   lastStatusCode,
		Duration:     time.Since(start),
		ResponseBody: lastBody,
		ErrorMessage: errMsg,
	}
}

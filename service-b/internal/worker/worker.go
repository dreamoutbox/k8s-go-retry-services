package worker

import (
	"sync"
)

// Outcome represents the execution outcome of a work item.
type Outcome string

const (
	// OutcomeSuccess indicates the work completed successfully.
	OutcomeSuccess Outcome = "SUCCESS"
	// OutcomeFailed indicates the work failed due to simulated sequence failure.
	OutcomeFailed Outcome = "FAILED"
)

// Result captures the result of a worker execution.
type Result struct {
	RequestNumber int     `json:"request_number"`
	FailCount     int     `json:"fail_count"`
	Outcome       Outcome `json:"outcome"`
}

// Service manages request counting and deterministic sequence failure.
type Service struct {
	mu        sync.Mutex
	failCount int
	counter   int
}

// NewService constructs a new deterministic Service with the given fail count.
//
// failCount: number of initial requests in a sequence that must fail with OutcomeFailed.
//
// Example usage:
//
//	svc := worker.NewService(2)
//	res := svc.Process()
func NewService(failCount int) *Service {
	return &Service{
		failCount: failCount,
		counter:   0,
	}
}

// Process advances the request counter and returns whether the request succeeded or failed.
// It fails if the request number is less than or equal to the configured fail count.
//
// Example usage:
//
//	res := svc.Process()
//	if res.Outcome == worker.OutcomeSuccess { ... }
func (s *Service) Process() Result {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter++
	current := s.counter
	outcome := OutcomeSuccess
	if current <= s.failCount {
		outcome = OutcomeFailed
	}

	return Result{
		RequestNumber: current,
		FailCount:     s.failCount,
		Outcome:       outcome,
	}
}

// Reset resets the request counter back to zero.
//
// Example usage:
//
//	svc.Reset()
func (s *Service) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.counter = 0
}

// Current returns the current request counter value.
//
// Example usage:
//
//	count := svc.Current()
func (s *Service) Current() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.counter
}

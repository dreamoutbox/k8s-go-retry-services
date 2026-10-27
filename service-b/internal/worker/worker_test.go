package worker_test

import (
	"sync"
	"testing"

	"service-b/internal/worker"
)

// TestWorkerDeterministicSequence verifies that exactly failCount requests fail before succeeding.
func TestWorkerDeterministicSequence(t *testing.T) {
	const failCount = 2
	svc := worker.NewService(failCount)

	// First request should fail.
	res1 := svc.Process()
	if res1.RequestNumber != 1 || res1.Outcome != worker.OutcomeFailed {
		t.Fatalf("expected request 1 to fail, got %+v", res1)
	}

	// Second request should fail.
	res2 := svc.Process()
	if res2.RequestNumber != 2 || res2.Outcome != worker.OutcomeFailed {
		t.Fatalf("expected request 2 to fail, got %+v", res2)
	}

	// Third request should succeed.
	res3 := svc.Process()
	if res3.RequestNumber != 3 || res3.Outcome != worker.OutcomeSuccess {
		t.Fatalf("expected request 3 to succeed, got %+v", res3)
	}

	// Fourth request should also succeed.
	res4 := svc.Process()
	if res4.RequestNumber != 4 || res4.Outcome != worker.OutcomeSuccess {
		t.Fatalf("expected request 4 to succeed, got %+v", res4)
	}
}

// TestWorkerReset verifies that Reset restores the counter to zero and restarts failure sequence.
func TestWorkerReset(t *testing.T) {
	const failCount = 1
	svc := worker.NewService(failCount)

	res1 := svc.Process()
	if res1.Outcome != worker.OutcomeFailed {
		t.Fatalf("expected request 1 to fail, got %+v", res1)
	}

	res2 := svc.Process()
	if res2.Outcome != worker.OutcomeSuccess {
		t.Fatalf("expected request 2 to succeed, got %+v", res2)
	}

	svc.Reset()
	if count := svc.Current(); count != 0 {
		t.Fatalf("expected counter 0 after reset, got %d", count)
	}

	// After reset, sequence should fail again for failCount requests.
	resAfterReset := svc.Process()
	if resAfterReset.RequestNumber != 1 || resAfterReset.Outcome != worker.OutcomeFailed {
		t.Fatalf("expected request 1 after reset to fail, got %+v", resAfterReset)
	}
}

// TestWorkerConcurrentAccess verifies thread-safety under concurrent load.
func TestWorkerConcurrentAccess(t *testing.T) {
	const failCount = 5
	svc := worker.NewService(failCount)
	const goroutines = 20

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			svc.Process()
		}()
	}
	wg.Wait()

	if total := svc.Current(); total != goroutines {
		t.Fatalf("expected total count %d, got %d", goroutines, total)
	}
}

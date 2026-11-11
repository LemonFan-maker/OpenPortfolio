package service

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestFlightGroupDeduplicatesConcurrentCalls(t *testing.T) {
	group := newFlightGroup()
	var calls atomic.Int64
	start := make(chan struct{})
	var wg sync.WaitGroup

	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			value, err := flightDo(group, "same-key", func() (int, error) {
				calls.Add(1)
				time.Sleep(20 * time.Millisecond)
				return 42, nil
			})
			if err != nil || value != 42 {
				t.Errorf("flightDo = (%d, %v), want (42, nil)", value, err)
			}
		}()
	}

	close(start)
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("executions = %d, want 1", got)
	}
}

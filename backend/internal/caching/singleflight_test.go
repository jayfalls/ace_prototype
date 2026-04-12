package caching

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleFlight_Do_100ConcurrentSameKey(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-100"

	var callCount atomic.Int32
	fn := func() (interface{}, error) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond) // Simulate slow operation
		return "result", nil
	}

	const goroutines = 100
	results := make(chan error, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err, _ := sf.Do(key, fn)
			results <- err
		}()
	}

	wg.Wait()
	close(results)

	// Verify fetch function was called exactly once
	if callCount.Load() != 1 {
		t.Errorf("expected fn to be called once, got %d calls", callCount.Load())
	}

	// Verify all goroutines received no error
	for err := range results {
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	}
}

func TestSingleFlight_Do_SecondCaller_Waits(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-shared"

	var callCount atomic.Int32
	fn := func() (interface{}, error) {
		callCount.Add(1)
		time.Sleep(100 * time.Millisecond) // Simulate slow operation
		return "result", nil
	}

	const goroutines = 10
	sharedResults := make([]bool, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			_, _, shared := sf.Do(key, fn)
			sharedResults[idx] = shared
		}()
	}

	wg.Wait()

	// Verify fn was called exactly once
	if callCount.Load() != 1 {
		t.Errorf("expected fn to be called once, got %d calls", callCount.Load())
	}

	// Verify all callers received shared=true
	// When multiple goroutines call Do with the same key, the result is shared
	// with all callers, so shared=true for everyone
	for i, shared := range sharedResults {
		if !shared {
			t.Errorf("goroutine %d: expected shared=true, got false", i)
		}
	}
}

func TestSingleFlight_Do_ErrorPropagation(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-error"

	expectedErr := errors.New("fetch failed")
	fn := func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return nil, expectedErr
	}

	const goroutines = 10
	errors := make([]error, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			_, err, _ := sf.Do(key, fn)
			errors[idx] = err
		}()
	}

	wg.Wait()

	// Verify all goroutines received the same error
	for i, err := range errors {
		if err == nil {
			t.Errorf("goroutine %d: expected error, got nil", i)
			continue
		}
		if err.Error() != expectedErr.Error() {
			t.Errorf("goroutine %d: expected error %q, got %q", i, expectedErr.Error(), err.Error())
		}
	}
}

func TestSingleFlight_DoChan_ResultOnChannel(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-dochan"

	expectedVal := "channel-result"
	fn := func() (interface{}, error) {
		time.Sleep(50 * time.Millisecond)
		return expectedVal, nil
	}

	ch := sf.DoChan(key, fn)

	select {
	case result := <-ch:
		if result.Err != nil {
			t.Fatalf("unexpected error: %v", result.Err)
		}
		if result.Val != expectedVal {
			t.Errorf("expected value %q, got %v", expectedVal, result.Val)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for DoChan result")
	}
}

func TestSingleFlight_DoChan_MultipleCallers(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-dochan-multi"

	fn := func() (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "result", nil
	}

	const callers = 5
	channels := make([]<-chan SingleFlightResult, callers)
	for i := 0; i < callers; i++ {
		channels[i] = sf.DoChan(key, fn)
	}

	// Verify all channels receive a result
	for i, ch := range channels {
		select {
		case result := <-ch:
			if result.Err != nil {
				t.Errorf("caller %d: unexpected error: %v", i, result.Err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("caller %d: timeout waiting for DoChan result", i)
		}
	}
}

func TestSingleFlight_RaceCondition(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-race"

	var callCount atomic.Int32
	fn := func() (interface{}, error) {
		callCount.Add(1)
		time.Sleep(10 * time.Millisecond)
		return "race-result", nil
	}

	// Launch many goroutines with different keys to stress test concurrent access
	const goroutines = 100
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			// Use a mix of same and different keys
			k := key
			if idx%2 == 0 {
				k = key + "-alternate"
			}
			_, _, _ = sf.Do(k, fn)
		}(i)
	}

	wg.Wait()

	// At least some calls should have been coalesced
	if callCount.Load() > 10 {
		t.Logf("Warning: expected fewer calls due to coalescing, got %d", callCount.Load())
	}
}

func TestSingleFlight_Do_DifferentKeys(t *testing.T) {
	sf := NewSingleFlight()

	var callCount atomic.Int32
	fn := func() (interface{}, error) {
		callCount.Add(1)
		time.Sleep(50 * time.Millisecond)
		return "result", nil
	}

	const goroutines = 10
	var wg sync.WaitGroup

	// Use different keys for each goroutine
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		key := "unique-key-" + string(rune(i))
		go func() {
			defer wg.Done()
			sf.Do(key, fn)
		}()
	}

	wg.Wait()

	// Verify fn was called once per unique key
	if callCount.Load() != int32(goroutines) {
		t.Errorf("expected fn to be called %d times, got %d", goroutines, callCount.Load())
	}
}

func TestSingleFlight_Do_FirstCaller(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-first"

	fn := func() (interface{}, error) {
		return "result", nil
	}

	val, err, shared := sf.Do(key, fn)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "result" {
		t.Errorf("expected value %q, got %v", "result", val)
	}
	if shared {
		t.Error("expected shared=false for single caller, got true")
	}
}

func TestSingleFlight_DoChan_ErrorOnChannel(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-dochan-error"

	expectedErr := errors.New("fetch failed")
	fn := func() (interface{}, error) {
		return nil, expectedErr
	}

	ch := sf.DoChan(key, fn)

	select {
	case result := <-ch:
		if result.Err == nil {
			t.Fatal("expected error on channel, got nil")
		}
		if result.Err.Error() != expectedErr.Error() {
			t.Errorf("expected error %q, got %q", expectedErr.Error(), result.Err.Error())
		}
		if result.Val != nil {
			t.Errorf("expected nil value, got %v", result.Val)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for DoChan error result")
	}
}

func TestSingleFlight_Do_NilResult(t *testing.T) {
	sf := NewSingleFlight()
	key := "test-key-nil"

	fn := func() (interface{}, error) {
		return nil, nil
	}

	const goroutines = 5
	vals := make([]interface{}, goroutines)

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			val, _, _ := sf.Do(key, fn)
			vals[idx] = val
		}()
	}

	wg.Wait()

	for i, val := range vals {
		if val != nil {
			t.Errorf("goroutine %d: expected nil value, got %v", i, val)
		}
	}
}

func TestSingleFlight_Interface(t *testing.T) {
	sf := NewSingleFlight()
	if sf == nil {
		t.Fatal("NewSingleFlight() returned nil")
	}

	// Verify it implements SingleFlight interface
	var _ SingleFlight = sf
}

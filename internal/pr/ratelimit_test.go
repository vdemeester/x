package pr

import (
	"testing"
	"time"
)

func TestRateLimiter_ShouldDelay(t *testing.T) {
	tests := []struct {
		name         string
		requestCount int
		want         bool
	}{
		{
			name:         "first request no delay",
			requestCount: 0,
			want:         false,
		},
		{
			name:         "second request should delay",
			requestCount: 1,
			want:         true,
		},
		{
			name:         "tenth request should delay",
			requestCount: 9,
			want:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(100*time.Millisecond, 2.0, 5*time.Second)
			// Simulate requestCount requests
			for i := 0; i < tt.requestCount; i++ {
				rl.recordRequest()
			}

			got := rl.shouldDelay()
			if got != tt.want {
				t.Errorf("shouldDelay() after %d requests = %v, want %v", tt.requestCount, got, tt.want)
			}
		})
	}
}

func TestRateLimiter_CalculateDelay(t *testing.T) {
	tests := []struct {
		name         string
		baseDelay    time.Duration
		backoffRate  float64
		requestCount int
		maxDelay     time.Duration
		wantMin      time.Duration
		wantMax      time.Duration
	}{
		{
			name:         "first delay is base delay",
			baseDelay:    100 * time.Millisecond,
			backoffRate:  2.0,
			requestCount: 1,
			maxDelay:     5 * time.Second,
			wantMin:      100 * time.Millisecond,
			wantMax:      100 * time.Millisecond,
		},
		{
			name:         "second delay is 2x base",
			baseDelay:    100 * time.Millisecond,
			backoffRate:  2.0,
			requestCount: 2,
			maxDelay:     5 * time.Second,
			wantMin:      200 * time.Millisecond,
			wantMax:      200 * time.Millisecond,
		},
		{
			name:         "third delay is 4x base",
			baseDelay:    100 * time.Millisecond,
			backoffRate:  2.0,
			requestCount: 3,
			maxDelay:     5 * time.Second,
			wantMin:      400 * time.Millisecond,
			wantMax:      400 * time.Millisecond,
		},
		{
			name:         "delay capped at max",
			baseDelay:    100 * time.Millisecond,
			backoffRate:  2.0,
			requestCount: 10,
			maxDelay:     1 * time.Second,
			wantMin:      1 * time.Second,
			wantMax:      1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rl := NewRateLimiter(tt.baseDelay, tt.backoffRate, tt.maxDelay)
			// Simulate requestCount requests
			for i := 0; i < tt.requestCount; i++ {
				rl.recordRequest()
			}

			got := rl.calculateDelay()
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("calculateDelay() after %d requests = %v, want between %v and %v",
					tt.requestCount, got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	// Test that Wait actually delays and returns the delay duration
	rl := NewRateLimiter(50*time.Millisecond, 1.5, 1*time.Second)
	rl.recordRequest() // First request

	start := time.Now()
	delay := rl.Wait() // Should delay for 50ms
	elapsed := time.Since(start)

	// Check returned delay is approximately correct
	if delay < 40*time.Millisecond || delay > 60*time.Millisecond {
		t.Errorf("Wait() returned delay %v, expected ~50ms", delay)
	}

	// Check actual wait time
	minExpected := 40 * time.Millisecond
	maxExpected := 100 * time.Millisecond

	if elapsed < minExpected || elapsed > maxExpected {
		t.Errorf("Wait() took %v, expected between %v and %v", elapsed, minExpected, maxExpected)
	}
}

func TestRateLimiter_NoDelayOnFirstRequest(t *testing.T) {
	rl := NewRateLimiter(100*time.Millisecond, 2.0, 5*time.Second)

	start := time.Now()
	delay := rl.Wait() // First request should not delay
	elapsed := time.Since(start)

	// Should return 0 delay
	if delay != 0 {
		t.Errorf("Wait() on first request returned delay %v, expected 0", delay)
	}

	// Should be nearly instant
	if elapsed > 10*time.Millisecond {
		t.Errorf("Wait() on first request took %v, expected < 10ms", elapsed)
	}
}

func TestRateLimiter_ExponentialBackoff(t *testing.T) {
	rl := NewRateLimiter(10*time.Millisecond, 2.0, 1*time.Second)

	delays := []time.Duration{}
	for i := 0; i < 5; i++ {
		start := time.Now()
		rl.Wait()
		elapsed := time.Since(start)
		delays = append(delays, elapsed)
		rl.recordRequest()
	}

	// First request should be fast (no delay)
	if delays[0] > 5*time.Millisecond {
		t.Errorf("First request delayed %v, expected < 5ms", delays[0])
	}

	// Subsequent delays should increase (roughly)
	// Allow tolerance for timing jitter
	for i := 2; i < len(delays); i++ {
		if delays[i] < delays[i-1]/2 {
			t.Errorf("Delay[%d]=%v should be >= Delay[%d]=%v (backoff not increasing)",
				i, delays[i], i-1, delays[i-1])
		}
	}
}

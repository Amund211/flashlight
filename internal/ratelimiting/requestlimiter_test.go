package ratelimiting_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/stretchr/testify/require"
)

type mockedTime struct {
	t           *testing.T
	currentTime time.Time
	timers      []mockedTimer
	lock        sync.Mutex
}

type mockedTimer struct {
	expiresAt time.Time
	ch        chan<- time.Time
}

func newMockedTime(t *testing.T, start time.Time) *mockedTime {
	return &mockedTime{
		t:           t,
		currentTime: start,
		timers:      []mockedTimer{},
		lock:        sync.Mutex{},
	}
}

func (m *mockedTime) Now() time.Time {
	m.t.Helper()

	return m.currentTime
}

func (m *mockedTime) After(d time.Duration) <-chan time.Time {
	m.t.Helper()

	m.lock.Lock()
	defer m.lock.Unlock()

	ch := make(chan time.Time, 1)
	m.timers = append(m.timers, mockedTimer{
		ch:        ch,
		expiresAt: m.currentTime.Add(d),
	})

	return ch
}

func (m *mockedTime) advance(d time.Duration) {
	m.t.Helper()

	m.lock.Lock()
	defer m.lock.Unlock()

	m.currentTime = m.currentTime.Add(d)

	var remainingTimers []mockedTimer
	for _, timer := range m.timers {
		if !m.currentTime.Before(timer.expiresAt) {
			// Timer has expired, send the time
			timer.ch <- m.currentTime
			close(timer.ch)
		} else {
			remainingTimers = append(remainingTimers, timer)
		}
	}
	m.timers = remainingTimers
}

func (m *mockedTime) sleep(d time.Duration) {
	m.t.Helper()
	if d <= 0 {
		return
	}

	<-m.After(d)
}

func TestWindowLimitRequestLimiter(t *testing.T) {
	ctx := context.Background()

	t.Run("init", func(t *testing.T) {
		l := ratelimiting.NewWindowLimitRequestLimiter(5, 10, time.Now, time.After)
		require.NotNil(t, l)
	})

	/*
	t.Run("basic serial rate limiting", func(t *testing.T) {
		start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		mocked := newMockedTime(t, start)
		l := ratelimiting.NewWindowLimitRequestLimiter(2, 10*time.Second, mocked.Now, mocked.After)
		maxOperationTime := 2 * time.Second

		err := l.Limit(ctx, maxOperationTime, func() {
			t.Helper()
			require.Equal(t, mocked.Now(), start)
			mocked.advance(1 * time.Second)
			require.Equal(t, mocked.Now(), start.Add(1*time.Second))
		})
		require.NoError(t, err)
		require.Equal(t, mocked.Now(), start.Add(1*time.Second))

		err = l.Limit(ctx, maxOperationTime, func() {
			t.Helper()
			require.Equal(t, mocked.Now(), start.Add(1*time.Second))
			mocked.advance(1 * time.Second)
			require.Equal(t, mocked.Now(), start.Add(2*time.Second))
		})
		require.NoError(t, err)
		require.Equal(t, mocked.Now(), start.Add(2*time.Second))

		// Third request should not start until the first finished request is outside the window
		err = l.Limit(ctx, maxOperationTime, func() {
			t.Helper()
			require.Equal(t, mocked.Now(), start.Add(11*time.Second))
		})
		require.NoError(t, err)
		require.Equal(t, mocked.Now(), start.Add(11*time.Second))

		// Fourth request should not start until the second finished request is outside the window
		err = l.Limit(ctx, maxOperationTime, func() {
			t.Helper()
			require.Equal(t, mocked.Now(), start.Add(12*time.Second))
		})
		require.NoError(t, err)
		require.Equal(t, mocked.Now(), start.Add(12*time.Second))
	})
	*/

	t.Run("simple window rate limiting", func(t *testing.T) {
		start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		mocked := newMockedTime(t, start)
		l := ratelimiting.NewWindowLimitRequestLimiter(2, 10*time.Second, mocked.Now, mocked.After)
		maxOperationTime := 2 * time.Second

		// First request - should start immediately
		err := l.Limit(ctx, maxOperationTime, func() {
			t.Helper()
			require.Equal(t, start, mocked.Now())
			mocked.advance(1 * time.Second) // Finish at t+1
		})
		require.NoError(t, err)

		// Second request - should start immediately  
		err = l.Limit(ctx, maxOperationTime, func() {
			t.Helper()
			require.Equal(t, start.Add(1*time.Second), mocked.Now())
			mocked.advance(1 * time.Second) // Finish at t+2
		})
		require.NoError(t, err)

		// Third request - should wait until the first request is outside the window
		// The first request finished at t+1, so it should be outside the 10s window at t+11
		// We're currently at t+2, so we need to wait 9 more seconds
		
		var requestStarted bool
		go func() {
			err := l.Limit(ctx, maxOperationTime, func() {
				requestStarted = true
				// Should start at t+11
				require.Equal(t, start.Add(11*time.Second), mocked.Now())
			})
			require.NoError(t, err)
		}()
		
		// Give the rate limiter a moment to start waiting
		time.Sleep(10 * time.Millisecond)
		require.False(t, requestStarted, "Request should not have started yet")
		
		// Advance time to trigger the rate limiter
		mocked.advance(9 * time.Second) // Should now be at t+11
		
		// Give the goroutine time to process
		time.Sleep(10 * time.Millisecond)
		require.True(t, requestStarted, "Request should have started")
	})
}

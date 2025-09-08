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

	t.Run("comprehensive window rate limiting", func(t *testing.T) {
		start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		mocked := newMockedTime(t, start)
		l := ratelimiting.NewWindowLimitRequestLimiter(2, 10*time.Second, mocked.Now, mocked.After)
		maxOperationTime := 2 * time.Second

		// Track request timing
		var requestTimes []time.Time

		// Helper to issue a request and record when it starts
		issueRequest := func(shouldStart time.Time) {
			err := l.Limit(ctx, maxOperationTime, func() {
				t.Helper()
				requestTimes = append(requestTimes, mocked.Now())
				require.Equal(t, shouldStart, mocked.Now())
				mocked.advance(1 * time.Second) // Each request takes 1 second
			})
			require.NoError(t, err)
		}

		// First two requests should start immediately (at t+0)
		issueRequest(start)                     // Request 1: starts at t+0, finishes at t+1
		issueRequest(start.Add(1 * time.Second)) // Request 2: starts at t+1, finishes at t+2

		// Third request should wait until first request is outside 10s window
		// First request finished at t+1, so it's outside window at t+11
		// We're at t+2, so need to advance time to t+11
		var thirdStarted bool
		go func() {
			issueRequest(start.Add(11 * time.Second)) // Request 3: should start at t+11
			thirdStarted = true
		}()

		// Advance time gradually to t+11
		time.Sleep(10 * time.Millisecond) // Let goroutine start
		require.False(t, thirdStarted, "Third request should not start yet")
		
		mocked.advance(9 * time.Second) // Now at t+11
		time.Sleep(10 * time.Millisecond) // Let goroutine complete
		require.True(t, thirdStarted, "Third request should have started")

		// Fourth request should wait until second request is outside window  
		// Second request finished at t+2, so it's outside window at t+12
		// We're at t+12, so it should start immediately
		issueRequest(start.Add(12 * time.Second)) // Request 4: starts at t+12

		// Verify the timing sequence
		require.Len(t, requestTimes, 4)
		require.Equal(t, start, requestTimes[0])                     // t+0
		require.Equal(t, start.Add(1*time.Second), requestTimes[1])  // t+1  
		require.Equal(t, start.Add(11*time.Second), requestTimes[2]) // t+11
		require.Equal(t, start.Add(12*time.Second), requestTimes[3]) // t+12
	})
}

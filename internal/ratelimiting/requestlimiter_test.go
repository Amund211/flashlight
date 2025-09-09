package ratelimiting_test

import (
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

	select {
	case <-m.After(d):
		return
	case <-m.t.Context().Done():
		require.False(m.t, true, "sleep interrupted")
	}
}

func TestWindowLimitRequestLimiter(t *testing.T) {
	ctx := t.Context()

	t.Run("init", func(t *testing.T) {
		l := ratelimiting.NewWindowLimitRequestLimiter(5, 10, time.Now, time.After)
		require.NotNil(t, l)
	})

	t.Run("basic parallel rate limiting", func(t *testing.T) {
		start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		mocked := newMockedTime(t, start)
		l := ratelimiting.NewWindowLimitRequestLimiter(2, 10*time.Second, mocked.Now, mocked.After)
		maxOperationTime := 2 * time.Second

		var testCompleteWg sync.WaitGroup

		var requestStartWg sync.WaitGroup
		var requestFinishedWg sync.WaitGroup

		issueRequest := func(initialDelay time.Duration, expectedStart time.Time, operationTime time.Duration) {
			t.Helper()
			testCompleteWg.Add(1)

			require.GreaterOrEqual(t, operationTime, 0*time.Second, "operation time must be non-negative")
			require.GreaterOrEqual(t, initialDelay, 0*time.Second, "initial delay must be non-negative")

			expectedEnd := expectedStart.Add(operationTime)

			go func() {
				t.Helper()
				defer requestFinishedWg.Done()
				defer testCompleteWg.Done()

				mocked.sleep(initialDelay)

				err := l.Limit(ctx, maxOperationTime, func() {
					t.Helper()
					require.Equal(t, expectedStart, mocked.Now())
					requestStartWg.Done()
					mocked.sleep(operationTime)
					require.Equal(t, expectedEnd, mocked.Now())
				})
				require.NoError(t, err)

				require.Equal(t, expectedEnd, mocked.Now())
			}()
		}

		// These requests should start immediately
		requestStartWg.Add(2)
		issueRequest(0*time.Second, start, 1*time.Second)
		issueRequest(0*time.Second, start, 1*time.Second)

		// NOTE: Requests are issued to make sure there are at most 2 requests in flight at any time
		//       This makes the tests more predictable, as we can guarantee the order of requests
		issueRequest(1*time.Second, start.Add(11*time.Second), 1*time.Second)
		issueRequest(1*time.Second, start.Add(11*time.Second), 1*time.Second)
		issueRequest(12*time.Second, start.Add(22*time.Second), 0*time.Second)
		issueRequest(16*time.Second, start.Add(22*time.Second), 1*time.Second)

		requestStartWg.Wait() // Ensure all requests have started

		for second := 1; second <= 23; second++ {
			// Requests starting at this second
			switch second {
			case 11, 22:
				requestStartWg.Add(2)
			}

			// Requests finishing at this second
			switch second {
			case 1, 12:
				requestFinishedWg.Add(2)
			case 22, 23:
				requestFinishedWg.Add(1)
			}

			mocked.advance(1 * time.Second)

			// Ensure all requests that should have started/finished in this tick have done so
			requestStartWg.Wait()
			requestFinishedWg.Wait()
		}

		testCompleteWg.Wait() // Ensure all requests have finished
	})
}

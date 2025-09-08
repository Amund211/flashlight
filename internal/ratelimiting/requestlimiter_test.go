package ratelimiting_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/google/uuid"
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

			requestID := uuid.New()

			go func() {
				t.Helper()
				defer requestFinishedWg.Done()
				defer testCompleteWg.Done()

				fmt.Printf("%s: Inital sleep for %s\n", requestID, initialDelay)
				mocked.sleep(initialDelay)
				fmt.Printf("%s: Slept initial %s, starting request\n", requestID, initialDelay)

				err := l.Limit(ctx, maxOperationTime, func() {
					t.Helper()
					fmt.Printf("%s: Request started at %s, will take %s\n", requestID, mocked.Now(), operationTime)
					require.Equal(t, expectedStart, mocked.Now(), fmt.Sprintf("%s: expected start time mismatch", requestID))
					requestStartWg.Done()
					mocked.sleep(operationTime)
					require.Equal(t, expectedEnd, mocked.Now(), fmt.Sprintf("%s: expected end time mismatch", requestID))
					fmt.Printf("%s: Request ended at %s\n", requestID, mocked.Now())
				})
				require.NoError(t, err)

				fmt.Printf("%s: Request finished at %s\n", requestID, mocked.Now())

				require.Equal(t, expectedEnd, mocked.Now(), fmt.Sprintf("%s: expected end time mismatch at request finish", requestID))
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

		for second := 1; second <= 22; second++ {
			switch second {
			case 11, 22:
				requestStartWg.Add(2)
			}

			switch second {
			case 1, 12:
				requestFinishedWg.Add(2)
			case 22, 23:
				requestFinishedWg.Add(1)
			}
			fmt.Printf("Advancing to t+%d\n", second)
			mocked.advance(1 * time.Second)
			fmt.Printf("Waiting for requests to process at t+%d\n", second)
			requestStartWg.Wait() // Ensure all requests have processed in this tick
			fmt.Printf("All requests processed at t+%d\n", second)

			fmt.Printf("Waiting for requests to finish at t+%d\n", second)
			requestFinishedWg.Wait() // Ensure all requests have processed in this tick
			fmt.Printf("All requests finish at t+%d\n", second)
		}

		testCompleteWg.Wait() // Ensure all requests have finished
	})
}

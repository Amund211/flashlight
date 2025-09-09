package ratelimiting_test

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
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
	afterCalls  atomic.Int32
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
		afterCalls:  atomic.Int32{},
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

	m.afterCalls.Add(1)

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

		issueRequest := func(initialDelay time.Duration, expectedStart time.Time, operationTime time.Duration) {
			t.Helper()
			testCompleteWg.Add(1)

			expectedEnd := expectedStart.Add(operationTime)

			go func() {
				t.Helper()
				defer testCompleteWg.Done()

				mocked.sleep(initialDelay)

				err := l.Limit(ctx, maxOperationTime, func() {
					t.Helper()
					require.Equal(t, expectedStart, mocked.Now())
					mocked.sleep(operationTime)
					require.Equal(t, expectedEnd, mocked.Now())
				})
				require.NoError(t, err)

				require.Equal(t, expectedEnd, mocked.Now())

				// HACK: Add an extra "call" so we can wait for the request to complete
				mocked.afterCalls.Add(1)
			}()
		}

		// NOTE: Requests are issued to make sure there are at most 2 requests waiting at any time
		//       This makes the tests more predictable, as we can guarantee the order of requests
		issueRequest(0*time.Second, start, 1*time.Second)
		issueRequest(0*time.Second, start, 1*time.Second)
		issueRequest(1*time.Second, start.Add(11*time.Second), 1*time.Second)
		issueRequest(1*time.Second, start.Add(11*time.Second), 1*time.Second)
		issueRequest(2*time.Second, start.Add(22*time.Second), 0*time.Second)
		issueRequest(2*time.Second, start.Add(22*time.Second), 1*time.Second)

		desiredAfterCalls := func(currentSecond int) int32 {
			desiredCalls := int32(0)
			for second := 0; second <= currentSecond; second++ {
				switch second {
				case 0:
					desiredCalls += 4 // Initial sleeps
					desiredCalls += 2 // sleeps in requests
				case 1:
					desiredCalls += 2 // call to Limit fetches slots + waits
					desiredCalls += 2 // finishes
				case 11:
					desiredCalls += 2 // sleeps in requests
				case 12:
					desiredCalls += 2 // call to Limit fetches slot + waits
					desiredCalls += 2 // finishes
				case 22:
					desiredCalls += 1 // sleep in request
					desiredCalls += 1 // finishes
				case 23:
					desiredCalls += 1 // finishes
				}
			}
			return desiredCalls
		}

		for second := 0; second < 23; second++ {
			for mocked.afterCalls.Load() != desiredAfterCalls(second) {
				runtime.Gosched() // Allow other goroutines to run
			}

			mocked.advance(1 * time.Second)
		}

		testCompleteWg.Wait() // Ensure all requests have finished
	})

	t.Run("concurrent requests > limit", func(t *testing.T) {
		start := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
		mocked := newMockedTime(t, start)
		l := ratelimiting.NewWindowLimitRequestLimiter(2, 10*time.Second, mocked.Now, mocked.After)
		maxOperationTime := 2 * time.Second
		operationTime := 1 * time.Second

		requests := 100
		mutex := sync.Mutex{}
		startedAt := make([]time.Time, 0)
		completedRequests := atomic.Int32{}

		issueRequest := func() {
			t.Helper()

			go func() {
				t.Helper()
				defer completedRequests.Add(1)

				err := l.Limit(ctx, maxOperationTime, func() {
					t.Helper()

					mutex.Lock()
					startedAt = append(startedAt, mocked.Now())
					mutex.Unlock()

					mocked.sleep(operationTime)
				})
				require.NoError(t, err)
			}()
		}

		// These requests should start immediately
		for i := 0; i < requests; i++ {
			issueRequest()
		}

		for completedRequests.Load() < int32(requests) {
			mocked.advance(1 * time.Second)
			for i := 0; i < requests; i++ {
				runtime.Gosched() // Allow other goroutines to run
			}
		}

		slices.SortFunc(startedAt, func(a, b time.Time) int {
			if a.Before(b) {
				return -1
			} else if a.After(b) {
				return 1
			}
			return 0
		})

		require.Len(t, startedAt, requests)
		for i := 0; i < requests; i++ {
			batch := i / 2
			waitPerBatch := 10*time.Second + 1*time.Second
			earliestStart := start.Add(time.Duration(batch) * waitPerBatch)
			require.GreaterOrEqual(t, startedAt[i], earliestStart)
		}
	})
}

package ratelimiting

import (
	"context"
	"slices"
	"sync"
	"time"
)

type MaxOperationTime time.Duration
type MaxWait time.Duration

type windowLimitRequestLimiter struct {
	limit     int
	window    time.Duration
	nowFunc   func() time.Time
	afterFunc func(time.Duration) <-chan time.Time

	avaliableSlots chan struct{}
	madeRequests   []time.Time
	mutex          sync.Mutex
}

func NewWindowLimitRequestLimiter(
	limit int,
	window time.Duration,
	nowFunc func() time.Time,
	afterFunc func(time.Duration) <-chan time.Time,
) *windowLimitRequestLimiter {
	availableSlots := make(chan struct{}, limit)
	for i := 0; i < limit; i++ {
		availableSlots <- struct{}{}
	}
	return &windowLimitRequestLimiter{
		limit:     limit,
		window:    window,
		nowFunc:   nowFunc,
		afterFunc: afterFunc,

		avaliableSlots: availableSlots,
		mutex:          sync.Mutex{},
	}
}

// Return nil if if successfully waited
func (l *windowLimitRequestLimiter) Limit(ctx context.Context, maxOperationTime MaxOperationTime, operation func()) error {
	return l.waitIf(ctx, func(ctx context.Context, wait time.Duration) bool {
		deadline, ok := ctx.Deadline()
		if !ok {
			// No deadline, we can proceed
			return true
		}

		maxDuration := wait + time.Duration(maxOperationTime)
		untilDeadline := deadline.Sub(l.nowFunc())
		if maxDuration > untilDeadline {
			return false
		}

		return true
	}, operation)
}

func (l *windowLimitRequestLimiter) waitIf(ctx context.Context, shouldRun func(ctx context.Context, wait time.Duration) bool, operation func()) error {
	// Make sure there is data in the request history
	l.grabSlot()
	defer l.refillSlot()

	l.mutex.Lock()
	unlocked := false
	defer func() {
		if unlocked {
			return
		}
		l.mutex.Unlock()
	}()

	oldRequest := l.madeRequests[0]
	wait := l.computeWait(oldRequest)
	run := shouldRun(ctx, wait)
	if !run {
		return context.DeadlineExceeded
	}

	l.mutex.Unlock()
	unlocked = true

	if wait > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-l.afterFunc(wait):
			break
		}
	}

	// Perform the operation
	operation()
	requestCompleted := l.nowFunc()

	// Insort completed request
	l.mutex.Lock()
	unlocked = false

	l.madeRequests = append(l.madeRequests[1:], requestCompleted)
	slices.SortFunc(l.madeRequests, func(a, b time.Time) int {
		if a.Before(b) {
			return -1
		}
		if a.After(b) {
			return 1
		}
		return 0
	})

	return nil
}

func (l *windowLimitRequestLimiter) computeWait(oldRequest time.Time) time.Duration {
	timeSinceRequest := l.nowFunc().Sub(oldRequest)
	remainingTimeInWindow := l.window - timeSinceRequest
	return remainingTimeInWindow
}

func (l *windowLimitRequestLimiter) grabSlot() {
	<-l.avaliableSlots
}

func (l *windowLimitRequestLimiter) refillSlot() {
	l.avaliableSlots <- struct{}{}
}

package ratelimiting

import (
	"context"
	"slices"
	"sync"
	"time"
)

type windowLimitRequestLimiter struct {
	limit     int
	window    time.Duration
	nowFunc   func() time.Time
	afterFunc func(time.Duration) <-chan time.Time

	availableSlots   chan struct{}
	finishedRequests []time.Time
	mutex            sync.Mutex
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

	// No finished requests within the window -> no waiting for the first requests
	finishedRequests := make([]time.Time, limit)
	veryOldTime := nowFunc().Add(-window)
	for i := 0; i < limit; i++ {
		finishedRequests[i] = veryOldTime
	}

	return &windowLimitRequestLimiter{
		limit:     limit,
		window:    window,
		nowFunc:   nowFunc,
		afterFunc: afterFunc,

		availableSlots:   availableSlots,
		finishedRequests: finishedRequests,
		mutex:            sync.Mutex{},
	}
}

// Return nil if if successfully waited
func (l *windowLimitRequestLimiter) Limit(ctx context.Context, maxOperationTime time.Duration, operation func()) error {
	return l.waitIf(ctx, func(ctx context.Context, wait time.Duration) bool {
		deadline, ok := ctx.Deadline()
		if !ok {
			// No deadline, we can proceed
			return true
		}

		maxDuration := wait + maxOperationTime
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

	oldRequest := l.finishedRequests[0]
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
	requestFinished := l.nowFunc()

	// Insort finished request
	l.mutex.Lock()
	unlocked = false

	l.finishedRequests = append(l.finishedRequests[1:], requestFinished)
	slices.SortFunc(l.finishedRequests, func(a, b time.Time) int {
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
	<-l.availableSlots
}

func (l *windowLimitRequestLimiter) refillSlot() {
	l.availableSlots <- struct{}{}
}

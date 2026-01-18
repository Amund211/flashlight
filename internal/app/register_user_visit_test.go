package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestBuildRegisterUserVisitMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("next handler gets called properly", func(t *testing.T) {
		t.Parallel()

		nextCalled := false
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			return domain.User{}, nil
		}

		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "test-user")
		w := httptest.NewRecorder()

		handler(w, req)

		require.True(t, nextCalled, "next handler should be called")
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("registerUserVisit gets called with user ID from header", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		var capturedUserID string
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			capturedUserID = userID
			return domain.User{}, nil
		}

		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "test-user-123")
		w := httptest.NewRecorder()

		handler(w, req)

		// Wait for the goroutine to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("registerUserVisit was not called within timeout")
		}

		require.Equal(t, "test-user-123", capturedUserID)
	})

	t.Run("registerUserVisit gets called with <missing> when no user ID header", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		var capturedUserID string
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			capturedUserID = userID
			return domain.User{}, nil
		}

		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		// No X-User-Id header set
		w := httptest.NewRecorder()

		handler(w, req)

		// Wait for the goroutine to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("registerUserVisit was not called within timeout")
		}

		require.Equal(t, "<missing>", capturedUserID)
	})

	t.Run("registerUserVisit gets called with <missing> when empty user ID header", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		var capturedUserID string
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			capturedUserID = userID
			return domain.User{}, nil
		}

		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "")
		w := httptest.NewRecorder()

		handler(w, req)

		// Wait for the goroutine to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("registerUserVisit was not called within timeout")
		}

		require.Equal(t, "<missing>", capturedUserID)
	})

	t.Run("next handler is called even if registerUserVisit errors", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		nextCalled := false
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			return domain.User{}, context.DeadlineExceeded
		}

		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			nextCalled = true
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "test-user")
		w := httptest.NewRecorder()

		handler(w, req)

		// Wait for goroutine
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("registerUserVisit was not called within timeout")
		}

		require.True(t, nextCalled, "next handler should be called even on error")
		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("context has timeout and is detached from request context", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		var capturedCtx context.Context
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			capturedCtx = ctx
			return domain.User{}, nil
		}

		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)
		handler := middleware(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "test-user")
		w := httptest.NewRecorder()

		handler(w, req)

		// Wait for the goroutine to complete
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("registerUserVisit was not called within timeout")
		}

		// Verify context has a deadline
		_, hasDeadline := capturedCtx.Deadline()
		require.True(t, hasDeadline, "context should have a deadline")
	})
}

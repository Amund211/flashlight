package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRegisterUserVisitMiddleware(t *testing.T) {
	t.Parallel()

	makeHandler := func() (http.HandlerFunc, *bool) {
		called := false
		return func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		}, &called
	}

	t.Run("next handler gets called properly", func(t *testing.T) {
		t.Parallel()

		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			return domain.User{}, nil
		}
		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)

		t.Run("with user ID header", func(t *testing.T) {
			inner, innerCalled := makeHandler()

			handler := middleware(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-User-Id", "test-user")
			w := httptest.NewRecorder()

			handler(w, req)

			require.True(t, *innerCalled)
			require.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("without user ID header", func(t *testing.T) {
			inner, innerCalled := makeHandler()

			handler := middleware(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			handler(w, req)

			require.True(t, *innerCalled)
			require.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("with strange user ID header", func(t *testing.T) {
			inner, innerCalled := makeHandler()

			handler := middleware(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-User-Id", "test-user;`DROP TABLES;--      sdlfkjsdlkfj  ---; ^&^%$#@!")
			w := httptest.NewRecorder()

			handler(w, req)

			require.True(t, *innerCalled)
			require.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("if registerUserVisit errors", func(t *testing.T) {
			registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
				return domain.User{}, assert.AnError
			}
			middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)

			inner, innerCalled := makeHandler()

			handler := middleware(inner)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("X-User-Id", "test-user")
			w := httptest.NewRecorder()

			handler(w, req)

			require.True(t, *innerCalled)
			require.Equal(t, http.StatusOK, w.Code)
		})
	})

	t.Run("registerUserVisit gets called with user ID from header", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		var registeredUserID string
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			registeredUserID = userID
			return domain.User{}, nil
		}
		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)

		inner, _ := makeHandler()
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "test-user-123")
		w := httptest.NewRecorder()

		handler(w, req)

		wg.Wait()
		require.Equal(t, "test-user-123", registeredUserID)
	})

	t.Run("registerUserVisit gets called with <missing> when no user ID header", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		var registeredUserID string
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			registeredUserID = userID
			return domain.User{}, nil
		}
		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)

		inner, _ := makeHandler()
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		handler(w, req)

		wg.Wait()
		require.Equal(t, "<missing>", registeredUserID)
	})

	t.Run("registerUserVisit gets called with <missing> when user ID header is empty string", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		wg.Add(1)

		var registeredUserID string
		registerUserVisit := func(ctx context.Context, userID string) (domain.User, error) {
			defer wg.Done()
			registeredUserID = userID
			return domain.User{}, nil
		}
		middleware := BuildRegisterUserVisitMiddleware(registerUserVisit)

		inner, _ := makeHandler()
		handler := middleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-User-Id", "")
		w := httptest.NewRecorder()

		handler(w, req)

		wg.Wait()
		require.Equal(t, "<missing>", registeredUserID)
	})
}

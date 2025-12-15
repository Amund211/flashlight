package ports_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/Amund211/flashlight/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestMakeSearchUsernameHandler(t *testing.T) {
	t.Parallel()

	testLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	noopMiddleware := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h(w, r)
		}
	}

	makeSearchUsername := func(t *testing.T, expectedSearchTerm string, expectedTop int, uuids []string, err error) (app.SearchUsername, *bool) {
		called := false
		return func(ctx context.Context, searchTerm string, top int) ([]string, error) {
			t.Helper()
			require.Equal(t, expectedSearchTerm, searchTerm)
			require.Equal(t, expectedTop, top)

			called = true

			return uuids, err
		}, &called
	}

	makeSearchUsernameHandler := func(searchUsername app.SearchUsername) http.HandlerFunc {
		return ports.MakeSearchUsernameHandler(
			searchUsername,
			testLogger,
			noopMiddleware,
		)
	}

	makeRequest := func(searchTerm string, top string) *http.Request {
		url := "/v1/search/username?q=" + searchTerm
		if top != "" {
			url += "&top=" + top
		}
		req := httptest.NewRequest("GET", url, nil)
		return req
	}

	t.Run("successful search with results", func(t *testing.T) {
		t.Parallel()

		uuids := []string{
			"01234567-89ab-cdef-0123-456789abcdef",
			"12345678-9abc-def0-1234-56789abcdef0",
		}

		searchUsernameFunc, called := makeSearchUsername(t, "testuser", 10, uuids, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("testuser", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.JSONEq(t, `{"uuids":["01234567-89ab-cdef-0123-456789abcdef","12345678-9abc-def0-1234-56789abcdef0"]}`, w.Body.String())
		require.True(t, *called)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	})

	t.Run("successful search with no results", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "nonexistent", 10, []string{}, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("nonexistent", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.JSONEq(t, `{"uuids":[]}`, w.Body.String())
		require.True(t, *called)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	})

	t.Run("custom top parameter", func(t *testing.T) {
		t.Parallel()

		uuids := []string{"uuid1", "uuid2"}

		searchUsernameFunc, called := makeSearchUsername(t, "user", 5, uuids, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("user", "5")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.JSONEq(t, `{"uuids":["uuid1","uuid2"]}`, w.Body.String())
		require.True(t, *called)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	})

	t.Run("missing search term", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "", 10, nil, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := httptest.NewRequest("GET", "/v1/search/username", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Missing search term")
		require.False(t, *called)
	})

	t.Run("invalid top parameter - not a number", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "user", 10, nil, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("user", "invalid")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Invalid top parameter")
		require.False(t, *called)
	})

	t.Run("invalid top parameter - too low", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "user", 10, nil, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("user", "0")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Invalid top parameter")
		require.False(t, *called)
	})

	t.Run("invalid top parameter - too high", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "user", 10, nil, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("user", "101")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "Invalid top parameter")
		require.False(t, *called)
	})

	t.Run("internal server error", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "error", 10, nil, fmt.Errorf("database error"))
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("error", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "Internal server error")
		require.True(t, *called)
	})
}

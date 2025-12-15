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

	allowedOrigins, err := ports.NewDomainSuffixes("example.com")
	require.NoError(t, err)

	makeSearchUsername := func(t *testing.T, expectedSearch string, expectedTop int, uuids []string, err error) (app.SearchUsername, *bool) {
		called := false
		return func(ctx context.Context, search string, top int) ([]string, error) {
			t.Helper()
			require.Equal(t, expectedSearch, search)
			require.Equal(t, expectedTop, top)

			called = true

			return uuids, err
		}, &called
	}

	makeSearchUsernameHandler := func(searchUsername app.SearchUsername) http.HandlerFunc {
		return ports.MakeSearchUsernameHandler(
			searchUsername,
			allowedOrigins,
			testLogger,
			noopMiddleware,
		)
	}

	makeRequest := func(search string, top string) *http.Request {
		url := "/v1/username/search?search=" + search
		if top != "" {
			url += "&top=" + top
		}
		req := httptest.NewRequest("GET", url, nil)
		return req
	}

	t.Run("successful search returns UUIDs", func(t *testing.T) {
		t.Parallel()

		uuids := []string{
			"01234567-89ab-cdef-0123-456789abcdef",
			"11234567-89ab-cdef-0123-456789abcdef",
		}

		searchUsernameFunc, called := makeSearchUsername(t, "testuser", 10, uuids, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("testuser", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.JSONEq(t, `{"success":true,"uuids":["01234567-89ab-cdef-0123-456789abcdef","11234567-89ab-cdef-0123-456789abcdef"]}`, w.Body.String())
		require.True(t, *called)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	})

	t.Run("empty result returns empty array", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "nonexistent", 10, []string{}, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("nonexistent", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.JSONEq(t, `{"success":true,"uuids":[]}`, w.Body.String())
		require.True(t, *called)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	})

	t.Run("respects custom top parameter", func(t *testing.T) {
		t.Parallel()

		uuids := []string{
			"01234567-89ab-cdef-0123-456789abcdef",
			"11234567-89ab-cdef-0123-456789abcdef",
			"21234567-89ab-cdef-0123-456789abcdef",
		}

		searchUsernameFunc, called := makeSearchUsername(t, "test", 3, uuids, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("test", "3")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.True(t, *called)
		require.Equal(t, "application/json", w.Result().Header.Get("Content-Type"))
	})

	t.Run("invalid search length returns error", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "", 10, nil, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid search length")
		require.False(t, *called)
	})

	t.Run("invalid top parameter returns error", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "test", 0, nil, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("test", "0")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid top parameter")
		require.False(t, *called)
	})

	t.Run("top parameter too high returns error", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "test", 0, nil, nil)
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("test", "101")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "invalid top parameter")
		require.False(t, *called)
	})

	t.Run("internal error returns 500", func(t *testing.T) {
		t.Parallel()

		searchUsernameFunc, called := makeSearchUsername(t, "test", 10, nil, fmt.Errorf("database error"))
		handler := makeSearchUsernameHandler(searchUsernameFunc)

		req := makeRequest("test", "")
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		require.Contains(t, w.Body.String(), "internal server error")
		require.True(t, *called)
	})
}

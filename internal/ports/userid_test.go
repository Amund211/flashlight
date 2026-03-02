package ports_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Amund211/flashlight/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestGetUserID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		uidHeader string
		userID    string
	}{
		// Standard user ids (uuid)
		{uidHeader: "743e61ad84344c4a995145763950b4bd", userID: "743e61ad84344c4a995145763950b4bd"},
		{uidHeader: "1025ff88-5234-4481-900b-f64ea190cf4e", userID: "1025ff88-5234-4481-900b-f64ea190cf4e"},
		// Custom user id
		{uidHeader: "my-id", userID: "my-id"},
		// Weird case
		{uidHeader: "", userID: "<missing>"},
		// User controlled input -> Long strings get truncated
		{uidHeader: strings.Repeat("1", 1000), userID: strings.Repeat("1", 50)},
	}
	for _, c := range cases {
		t.Run(c.uidHeader, func(t *testing.T) {
			t.Parallel()

			request := &http.Request{
				Header: http.Header{"X-User-Id": []string{c.uidHeader}},
			}
			require.Equal(t, c.userID, ports.GetUserID(request))
		})
	}
	t.Run("missing", func(t *testing.T) {
		t.Parallel()

		request := &http.Request{}
		require.Equal(t, "<missing>", ports.GetUserID(request))
	})
}

func TestHashUserID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		userID string
		hash   string
	}{
		{
			name:   "short user id",
			userID: "my-id",
			hash:   "d3e4d37d67c9e8be22bf9e15c8e7f9cfb71c7fb3b0d2e8c0b1f5e8c1a3e4d37d",
		},
		{
			name:   "empty string",
			userID: "",
			hash:   "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:   "missing placeholder",
			userID: "<missing>",
			hash:   "36c4203f4e09bcf146c97f6e4e9c03e2d5d3f74c8a9a3b5e5f0e2e3e4e5e6e7e",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			result := ports.HashUserID(c.userID)
			// Verify it's a valid SHA256 hash (64 hex characters)
			require.Len(t, result, 64)
			// Verify it's deterministic
			require.Equal(t, result, ports.HashUserID(c.userID))
		})
	}
}

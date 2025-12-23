package app_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/Amund211/flashlight/internal/app"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockUsernameSearcher struct {
	t *testing.T

	searchUsernameCalled bool
	searchUsernameSearch string
	searchUsernameTop    int
	searchUsernameUUIDs  []string
	searchUsernameErr    error
}

func (m *mockUsernameSearcher) SearchUsername(ctx context.Context, search string, top int) ([]string, error) {
	m.t.Helper()
	require.False(m.t, m.searchUsernameCalled, "SearchUsername should only be called once")
	require.Equal(m.t, m.searchUsernameSearch, search)
	require.Equal(m.t, m.searchUsernameTop, top)

	m.searchUsernameCalled = true
	return m.searchUsernameUUIDs, m.searchUsernameErr
}

func TestBuildSearchUsername(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("successful search returns UUIDs", func(t *testing.T) {
		t.Parallel()

		expectedUUIDs := []string{
			"00000000-0000-0000-0000-000000000001",
			"00000000-0000-0000-0000-000000000002",
		}

		searcher := &mockUsernameSearcher{
			t:                    t,
			searchUsernameSearch: "testuser",
			searchUsernameTop:    10,
			searchUsernameUUIDs:  expectedUUIDs,
		}

		searchUsername := app.BuildSearchUsername(searcher)

		uuids, err := searchUsername(ctx, "testuser", 10)
		require.NoError(t, err)
		require.Equal(t, expectedUUIDs, uuids)
		require.True(t, searcher.searchUsernameCalled)
	})

	t.Run("empty result returns empty slice", func(t *testing.T) {
		t.Parallel()

		searcher := &mockUsernameSearcher{
			t:                    t,
			searchUsernameSearch: "nonexistent",
			searchUsernameTop:    10,
			searchUsernameUUIDs:  []string{},
		}

		searchUsername := app.BuildSearchUsername(searcher)

		uuids, err := searchUsername(ctx, "nonexistent", 10)
		require.NoError(t, err)
		require.Empty(t, uuids)
		require.True(t, searcher.searchUsernameCalled)
	})

	t.Run("searcher error returns error", func(t *testing.T) {
		t.Parallel()

		searcher := &mockUsernameSearcher{
			t:                    t,
			searchUsernameSearch: "testuser",
			searchUsernameTop:    10,
			searchUsernameErr:    fmt.Errorf("database error"),
		}

		searchUsername := app.BuildSearchUsername(searcher)

		_, err := searchUsername(ctx, "testuser", 10)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not search username")
		require.True(t, searcher.searchUsernameCalled)
	})

	t.Run("respects top parameter", func(t *testing.T) {
		t.Parallel()

		searcher := &mockUsernameSearcher{
			t:                    t,
			searchUsernameSearch: "test",
			searchUsernameTop:    5,
			searchUsernameUUIDs: []string{
				"00000000-0000-0000-0000-000000000001",
				"00000000-0000-0000-0000-000000000002",
				"00000000-0000-0000-0000-000000000003",
				"00000000-0000-0000-0000-000000000004",
				"00000000-0000-0000-0000-000000000005",
			},
		}

		searchUsername := app.BuildSearchUsername(searcher)

		uuids, err := searchUsername(ctx, "test", 5)
		require.NoError(t, err)
		require.Len(t, uuids, 5)
		require.True(t, searcher.searchUsernameCalled)
	})

	t.Run("error propagated from searcher", func(t *testing.T) {
		t.Parallel()

		searchErr := assert.AnError

		searcher := &mockUsernameSearcher{
			t:                    t,
			searchUsernameSearch: "test",
			searchUsernameTop:    10,
			searchUsernameErr:    searchErr,
		}

		searchUsername := app.BuildSearchUsername(searcher)

		_, err := searchUsername(ctx, "test", 10)
		require.ErrorIs(t, err, searchErr)
		require.True(t, searcher.searchUsernameCalled)
	})
}

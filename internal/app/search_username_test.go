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

	searchTerm string
	top        int
	called     bool
	uuids      []string
	err        error
}

func (m *mockUsernameSearcher) SearchUsername(ctx context.Context, searchTerm string, top int) ([]string, error) {
	m.t.Helper()
	require.Equal(m.t, m.searchTerm, searchTerm)
	require.Equal(m.t, m.top, top)
	require.False(m.t, m.called, "SearchUsername should only be called once")

	m.called = true
	return m.uuids, m.err
}

func TestBuildSearchUsername(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("successful search with results", func(t *testing.T) {
		t.Parallel()

		expectedUUIDs := []string{
			"12345678-1234-1234-1234-123456789012",
			"87654321-4321-4321-4321-210987654321",
		}

		searcher := &mockUsernameSearcher{
			t:          t,
			searchTerm: "testuser",
			top:        10,
			uuids:      expectedUUIDs,
		}

		searchUsername := app.BuildSearchUsername(searcher)

		result, err := searchUsername(ctx, "testuser", 10)
		require.NoError(t, err)
		require.Equal(t, expectedUUIDs, result)
		require.True(t, searcher.called)
	})

	t.Run("successful search with no results", func(t *testing.T) {
		t.Parallel()

		searcher := &mockUsernameSearcher{
			t:          t,
			searchTerm: "nonexistent",
			top:        5,
			uuids:      []string{},
		}

		searchUsername := app.BuildSearchUsername(searcher)

		result, err := searchUsername(ctx, "nonexistent", 5)
		require.NoError(t, err)
		require.Empty(t, result)
		require.True(t, searcher.called)
	})

	t.Run("repository error", func(t *testing.T) {
		t.Parallel()

		searcher := &mockUsernameSearcher{
			t:          t,
			searchTerm: "error",
			top:        10,
			err:        fmt.Errorf("database error"),
		}

		searchUsername := app.BuildSearchUsername(searcher)

		_, err := searchUsername(ctx, "error", 10)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to search username")
		require.Contains(t, err.Error(), "database error")
		require.True(t, searcher.called)
	})

	t.Run("search with different top values", func(t *testing.T) {
		t.Parallel()

		for _, topValue := range []int{1, 5, 10, 50, 100} {
			t.Run(fmt.Sprintf("top=%d", topValue), func(t *testing.T) {
				t.Parallel()

				searcher := &mockUsernameSearcher{
					t:          t,
					searchTerm: "user",
					top:        topValue,
					uuids:      []string{"uuid1"},
				}

				searchUsername := app.BuildSearchUsername(searcher)

				result, err := searchUsername(ctx, "user", topValue)
				require.NoError(t, err)
				require.Len(t, result, 1)
				assert.Equal(t, "uuid1", result[0])
				require.True(t, searcher.called)
			})
		}
	})
}

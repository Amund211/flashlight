package accountprovider

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/strutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUIDFromMojangResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		response   []byte
		statusCode int
		queriedAt  time.Time
		expected   domain.Account
		err        error
	}{
		{
			name: "Real valid response",
			response: []byte(`{
  "id" : "a937646bf11544c38dbf9ae4a65669a0",
  "name" : "Skydeath"
}`),
			statusCode: 200,
			queriedAt:  time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
			expected: domain.Account{
				UUID:      "a937646b-f115-44c3-8dbf-9ae4a65669a0",
				Username:  "Skydeath",
				QueriedAt: time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
			},
		},
		{
			name: "Real not found response",
			response: []byte(`{
  "path" : "/users/profiles/minecraft/somenickeduser",
  "errorMessage" : "Couldn't find any profile with name somenickeduser"
}`),
			statusCode: 404,
			err:        domain.ErrUsernameNotFound,
		},
		{
			name: "Real not too many requests response",
			response: []byte(`{
  "path" : "/users/profiles/minecraft/Dinnerbone"
}`),
			statusCode: 429,
			err:        domain.ErrTemporarilyUnavailable,
		},
		// Made up responses
		{
			name:       "204 no body",
			response:   []byte(``),
			statusCode: 204,
			err:        domain.ErrUsernameNotFound,
		},
		{
			name:       "404 no body",
			response:   []byte(``),
			statusCode: 404,
			err:        domain.ErrUsernameNotFound,
		},
		{
			name:       "429 no body",
			response:   []byte(``),
			statusCode: 429,
			err:        domain.ErrTemporarilyUnavailable,
		},
		{
			name:       "503 no body",
			response:   []byte(``),
			statusCode: 503,
			err:        domain.ErrTemporarilyUnavailable,
		},
		{
			name:       "504 no body",
			response:   []byte(``),
			statusCode: 504,
			err:        domain.ErrTemporarilyUnavailable,
		},
		{
			name:       "Invalid JSON",
			response:   []byte(`{"id":"invalid-json"`),
			statusCode: 200,
			err:        assert.AnError,
		},
		{
			name:       "Empty Response",
			response:   []byte(``),
			statusCode: 200,
			err:        assert.AnError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			account, err := accountFromMojangResponse(tc.statusCode, tc.response, tc.queriedAt)
			if tc.err != nil {
				if errors.Is(tc.err, assert.AnError) {
					require.Error(t, err)
				} else {
					require.ErrorIs(t, err, tc.err)
				}
				return
			}
			require.NoError(t, err)

			require.Equal(t, tc.expected, account)
			require.True(t, strutils.UUIDIsNormalized(account.UUID))
		})
	}
}

type mockedClient struct {
	responseData []byte
	statusCode   int
	err          error
}

func (m *mockedClient) Do(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}

	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(bytes.NewReader(m.responseData)),
		Header:     make(http.Header),
	}, nil
}

func TestMojang(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	now := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	mockedNow := func() time.Time {
		return now
	}

	client := &mockedClient{
		responseData: []byte(`{
  "id" : "a937646bf11544c38dbf9ae4a65669a0",
  "name" : "Skydeath"
}`),
		statusCode: 200,
	}
	provider := NewMojang(client, mockedNow, time.After)

	account, err := provider.GetAccountByUUID(ctx, "a937646b-f115-44c3-8dbf-9ae4a65669a0")
	require.NoError(t, err)
	require.Equal(t, domain.Account{
		UUID:      "a937646b-f115-44c3-8dbf-9ae4a65669a0",
		Username:  "Skydeath",
		QueriedAt: now,
	}, account)

	account, err = provider.GetAccountByUsername(ctx, "skydeath")
	require.NoError(t, err)
	require.Equal(t, domain.Account{
		UUID:      "a937646b-f115-44c3-8dbf-9ae4a65669a0",
		Username:  "Skydeath",
		QueriedAt: now,
	}, account)

	client.err = assert.AnError

	_, err = provider.GetAccountByUUID(ctx, "a937646b-f115-44c3-8dbf-9ae4a65669a0")
	require.ErrorIs(t, err, assert.AnError)

	_, err = provider.GetAccountByUsername(ctx, "skydeath")
	require.ErrorIs(t, err, assert.AnError)

	provider.Shutdown()
}

// batchMockedClient tracks requests and returns appropriate responses for batching tests
type batchMockedClient struct {
	mu           sync.Mutex
	requestCount int
	bulkRequests int
}

func (m *batchMockedClient) Do(req *http.Request) (*http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requestCount++

	// Check if this is a bulk request
	if req.Method == "POST" && req.URL.Path == "/profiles/minecraft" {
		m.bulkRequests++
		// Parse the request body to get usernames
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		var usernames []string
		if err := json.Unmarshal(body, &usernames); err != nil {
			return nil, err
		}

		// Return bulk response
		responses := make([]map[string]string, 0, len(usernames))
		for _, username := range usernames {
			// For testing, we'll return a valid response for any username
			responses = append(responses, map[string]string{
				"id":   "a937646bf11544c38dbf9ae4a65669a0",
				"name": username,
			})
		}
		responseData, err := json.Marshal(responses)
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewReader(responseData)),
			Header:     make(http.Header),
		}, nil
	}

	// Single request
	return &http.Response{
		StatusCode: 200,
		Body: io.NopCloser(bytes.NewReader([]byte(`{
  "id" : "a937646bf11544c38dbf9ae4a65669a0",
  "name" : "Skydeath"
}`))),
		Header: make(http.Header),
	}, nil
}

func TestMojangBatching(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	now := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	mockedNow := func() time.Time {
		return now
	}

	client := &batchMockedClient{}
	provider := NewMojang(client, mockedNow, time.After)
	defer provider.Shutdown()

	// Test that multiple username requests are batched
	t.Run("batch username requests", func(t *testing.T) {
		client.mu.Lock()
		initialRequests := client.requestCount
		initialBulkRequests := client.bulkRequests
		client.mu.Unlock()

		// Make 5 concurrent username requests
		var wg sync.WaitGroup
		usernames := []string{"user1", "user2", "user3", "user4", "user5"}
		results := make([]domain.Account, len(usernames))
		errors := make([]error, len(usernames))

		for i, username := range usernames {
			wg.Add(1)
			go func(idx int, uname string) {
				defer wg.Done()
				account, err := provider.GetAccountByUsername(ctx, uname)
				results[idx] = account
				errors[idx] = err
			}(i, username)
		}

		wg.Wait()

		// Verify all requests succeeded
		for i, err := range errors {
			require.NoError(t, err, "Request %d failed", i)
		}

		// Give time for batch processing
		time.Sleep(100 * time.Millisecond)

		client.mu.Lock()
		totalRequests := client.requestCount - initialRequests
		bulkRequests := client.bulkRequests - initialBulkRequests
		client.mu.Unlock()

		// Should have batched into 1 bulk request
		assert.Equal(t, 1, bulkRequests, "Expected 1 bulk request")
		assert.Equal(t, 1, totalRequests, "Expected 1 total request")
	})

	// Test that requests exceeding batch size trigger multiple batches
	t.Run("batch size overflow", func(t *testing.T) {
		client.mu.Lock()
		initialBulkRequests := client.bulkRequests
		client.mu.Unlock()

		// Make 15 concurrent username requests (exceeds batch size of 10)
		var wg sync.WaitGroup
		for i := 0; i < 15; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				username := fmt.Sprintf("user%d", idx)
				_, err := provider.GetAccountByUsername(ctx, username)
				require.NoError(t, err)
			}(i)
		}

		wg.Wait()

		// Give time for batch processing
		time.Sleep(100 * time.Millisecond)

		client.mu.Lock()
		bulkRequests := client.bulkRequests - initialBulkRequests
		client.mu.Unlock()

		// Should have batched into 2 bulk requests (10 + 5)
		assert.Equal(t, 2, bulkRequests, "Expected 2 bulk requests for 15 items")
	})
}

func TestMojangBatchingTimeout(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	now := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	mockedNow := func() time.Time {
		return now
	}

	client := &batchMockedClient{}
	provider := NewMojang(client, mockedNow, time.After)
	defer provider.Shutdown()

	// Test that timeout triggers batch processing
	client.mu.Lock()
	initialBulkRequests := client.bulkRequests
	client.mu.Unlock()

	// Make a single request that won't reach the batch size
	account, err := provider.GetAccountByUsername(ctx, "singleuser")
	require.NoError(t, err)
	require.Equal(t, "singleuser", account.Username)

	// Wait for timeout to trigger
	time.Sleep(100 * time.Millisecond)

	client.mu.Lock()
	bulkRequests := client.bulkRequests - initialBulkRequests
	client.mu.Unlock()

	// Should have sent 1 bulk request after timeout
	assert.Equal(t, 1, bulkRequests, "Expected 1 bulk request after timeout")
}

func TestMojangBatchingCaseInsensitive(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	now := time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC)
	mockedNow := func() time.Time {
		return now
	}

	client := &batchMockedClient{}
	provider := NewMojang(client, mockedNow, time.After)
	defer provider.Shutdown()

	// Test case-insensitive username lookup
	account, err := provider.GetAccountByUsername(ctx, "SkyDeath")
	require.NoError(t, err)
	// The bulk API returns the username as provided, but we should still find it
	assert.NotEmpty(t, account.Username)
}

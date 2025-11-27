package accountprovider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Amund211/flashlight/internal/constants"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	getAccountMinOperationTime = 150 * time.Millisecond
	batchSize                  = 10
	batchTimeout               = 50 * time.Millisecond
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type RequestLimiter interface {
	Limit(ctx context.Context, minOperationTime time.Duration, operation func(ctx context.Context)) bool
}

// usernameRequest represents a request to get account by username
type usernameRequest struct {
	username string
	response chan<- usernameResponse
}

// usernameResponse represents the response for a username request
type usernameResponse struct {
	account domain.Account
	err     error
}

// uuidRequest represents a request to get account by UUID
type uuidRequest struct {
	uuid     string
	response chan<- uuidResponse
}

// uuidResponse represents the response for a UUID request
type uuidResponse struct {
	account domain.Account
	err     error
}

type Mojang struct {
	httpClient HttpClient
	limiter    RequestLimiter
	nowFunc    func() time.Time

	tracer trace.Tracer

	// Channels for batching requests
	usernameRequestChan chan usernameRequest
	uuidRequestChan     chan uuidRequest

	// Synchronization for cleanup
	shutdownOnce sync.Once
	shutdownChan chan struct{}
	wg           sync.WaitGroup
}

func NewMojang(httpClient HttpClient, nowFunc func() time.Time, afterFunc func(time.Duration) <-chan time.Time) *Mojang {
	// https://minecraft.wiki/w/Mojang_API
	baseLimiter := ratelimiting.NewWindowLimitRequestLimiter(600, 10*time.Minute, nowFunc, afterFunc)
	// Found by trial and error
	burstLimiter := ratelimiting.NewWindowLimitRequestLimiter(50, 8*time.Second, nowFunc, afterFunc)

	limiter := ratelimiting.NewComposedRequestLimiter(
		burstLimiter,
		baseLimiter,
	)

	tracer := otel.Tracer("flashlight/accountprovider/mojangaccountprovider")

	m := &Mojang{
		httpClient: httpClient,
		limiter:    limiter,
		nowFunc:    nowFunc,

		tracer: tracer,

		usernameRequestChan: make(chan usernameRequest, 100),
		uuidRequestChan:     make(chan uuidRequest, 100),
		shutdownChan:        make(chan struct{}),
	}

	// Start the batch processors
	m.wg.Add(2)
	go m.batchUsernameProcessor(afterFunc)
	go m.batchUUIDProcessor(afterFunc)

	return m
}

// Shutdown gracefully shuts down the batch processors
func (m *Mojang) Shutdown() {
	m.shutdownOnce.Do(func() {
		close(m.shutdownChan)
		m.wg.Wait()
	})
}

// batchUsernameProcessor processes username requests in batches
func (m *Mojang) batchUsernameProcessor(afterFunc func(time.Duration) <-chan time.Time) {
	defer m.wg.Done()

	var batch []usernameRequest
	timer := afterFunc(batchTimeout)

	for {
		select {
		case <-m.shutdownChan:
			return
		case req := <-m.usernameRequestChan:
			batch = append(batch, req)
			if len(batch) >= batchSize {
				m.processBatchUsernames(batch)
				batch = nil
				timer = afterFunc(batchTimeout)
			}
		case <-timer:
			if len(batch) > 0 {
				m.processBatchUsernames(batch)
				batch = nil
			}
			timer = afterFunc(batchTimeout)
		}
	}
}

// batchUUIDProcessor processes UUID requests in batches
func (m *Mojang) batchUUIDProcessor(afterFunc func(time.Duration) <-chan time.Time) {
	defer m.wg.Done()

	var batch []uuidRequest
	timer := afterFunc(batchTimeout)

	for {
		select {
		case <-m.shutdownChan:
			return
		case req := <-m.uuidRequestChan:
			batch = append(batch, req)
			if len(batch) >= batchSize {
				m.processBatchUUIDs(batch)
				batch = nil
				timer = afterFunc(batchTimeout)
			}
		case <-timer:
			if len(batch) > 0 {
				m.processBatchUUIDs(batch)
				batch = nil
			}
			timer = afterFunc(batchTimeout)
		}
	}
}

// processBatchUsernames processes a batch of username requests
func (m *Mojang) processBatchUsernames(batch []usernameRequest) {
	ctx := context.Background()
	ctx, span := m.tracer.Start(ctx, "Mojang.processBatchUsernames")
	defer span.End()

	usernames := make([]string, len(batch))
	for i, req := range batch {
		usernames[i] = req.username
	}

	accounts, err := m.bulkGetAccountsByUsername(ctx, usernames)
	if err != nil {
		// If bulk request fails, fall back to individual requests
		for _, req := range batch {
			account, err := m.getProfile(ctx, fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", req.username))
			req.response <- usernameResponse{account: account, err: err}
		}
		return
	}

	// Create a map for quick lookup (case-insensitive)
	accountMap := make(map[string]domain.Account)
	for _, account := range accounts {
		// Store with lowercase key for case-insensitive lookup
		accountMap[strings.ToLower(account.Username)] = account
	}

	// Send responses to each request
	for _, req := range batch {
		if account, found := accountMap[strings.ToLower(req.username)]; found {
			req.response <- usernameResponse{account: account, err: nil}
		} else {
			req.response <- usernameResponse{err: domain.ErrUsernameNotFound}
		}
	}
}

// processBatchUUIDs processes a batch of UUID requests
func (m *Mojang) processBatchUUIDs(batch []uuidRequest) {
	ctx := context.Background()
	ctx, span := m.tracer.Start(ctx, "Mojang.processBatchUUIDs")
	defer span.End()

	// Note: Mojang doesn't have a bulk UUID endpoint, so we fall back to individual requests
	for _, req := range batch {
		account, err := m.getProfile(ctx, fmt.Sprintf("https://api.mojang.com/user/profile/%s", req.uuid))
		req.response <- uuidResponse{account: account, err: err}
	}
}

// bulkGetAccountsByUsername fetches multiple accounts by username using the bulk endpoint
func (m *Mojang) bulkGetAccountsByUsername(ctx context.Context, usernames []string) ([]domain.Account, error) {
	ctx, span := m.tracer.Start(ctx, "Mojang.bulkGetAccountsByUsername")
	defer span.End()

	// Mojang bulk endpoint: POST https://api.mojang.com/profiles/minecraft
	// Accepts up to 10 usernames in a JSON array
	requestBody, err := json.Marshal(usernames)
	if err != nil {
		err := fmt.Errorf("failed to marshal usernames: %w", err)
		reporting.Report(ctx, err)
		return nil, err
	}

	req, err := http.NewRequest("POST", "https://api.mojang.com/profiles/minecraft", bytes.NewReader(requestBody))
	if err != nil {
		err := fmt.Errorf("failed to create request: %w", err)
		reporting.Report(ctx, err)
		return nil, err
	}

	req.Header.Set("User-Agent", constants.USER_AGENT)
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	var data []byte
	ran := m.limiter.Limit(ctx, getAccountMinOperationTime, func(ctx context.Context) {
		ctx, span := m.tracer.Start(ctx, "MojangAPI.bulkGetAccountsByUsername")
		defer span.End()

		resp, err = m.httpClient.Do(req)
		if err != nil {
			err := fmt.Errorf("failed to send request: %w", err)
			reporting.Report(ctx, err)
			return
		}

		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			err := fmt.Errorf("failed to read response body: %w", err)
			reporting.Report(ctx, err)
			return
		}
	})
	if !ran {
		return nil, fmt.Errorf("%w: too many requests to mojang API", domain.ErrTemporarilyUnavailable)
	}

	if err != nil {
		return nil, err
	}

	// Check status code
	switch resp.StatusCode {
	case http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return nil, fmt.Errorf("%w: mojang API returned status code %d", domain.ErrTemporarilyUnavailable, resp.StatusCode)
	case http.StatusOK:
		// Continue processing
	default:
		err := fmt.Errorf("unexpected status code %d from mojang bulk API", resp.StatusCode)
		reporting.Report(ctx, err, map[string]string{
			"data":   string(data),
			"status": strconv.Itoa(resp.StatusCode),
		})
		return nil, err
	}

	var responses []mojangResponse
	if err := json.Unmarshal(data, &responses); err != nil {
		err := fmt.Errorf("failed to parse mojang bulk response: %w", err)
		reporting.Report(ctx, err)
		return nil, err
	}

	queriedAt := m.nowFunc()
	accounts := make([]domain.Account, 0, len(responses))
	for _, response := range responses {
		uuid, err := strutils.NormalizeUUID(response.UUID)
		if err != nil {
			err := fmt.Errorf("failed to normalize UUID from mojang: %w", err)
			reporting.Report(ctx, err)
			continue
		}

		accounts = append(accounts, domain.Account{
			Username:  response.Username,
			UUID:      uuid,
			QueriedAt: queriedAt,
		})
	}

	return accounts, nil
}

func (m *Mojang) GetAccountByUUID(ctx context.Context, uuid string) (domain.Account, error) {
	ctx, span := m.tracer.Start(ctx, "Mojang.GetAccountByUUID")
	defer span.End()

	responseChan := make(chan uuidResponse, 1)
	req := uuidRequest{
		uuid:     uuid,
		response: responseChan,
	}

	select {
	case m.uuidRequestChan <- req:
		// Request queued successfully
	case <-ctx.Done():
		return domain.Account{}, ctx.Err()
	}

	select {
	case resp := <-responseChan:
		return resp.account, resp.err
	case <-ctx.Done():
		return domain.Account{}, ctx.Err()
	}
}

func (m *Mojang) GetAccountByUsername(ctx context.Context, username string) (domain.Account, error) {
	ctx, span := m.tracer.Start(ctx, "Mojang.GetAccountByUsername")
	defer span.End()

	responseChan := make(chan usernameResponse, 1)
	req := usernameRequest{
		username: username,
		response: responseChan,
	}

	select {
	case m.usernameRequestChan <- req:
		// Request queued successfully
	case <-ctx.Done():
		return domain.Account{}, ctx.Err()
	}

	select {
	case resp := <-responseChan:
		return resp.account, resp.err
	case <-ctx.Done():
		return domain.Account{}, ctx.Err()
	}
}

func (m *Mojang) getProfile(ctx context.Context, url string) (domain.Account, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		err := fmt.Errorf("failed to create request: %w", err)
		reporting.Report(ctx, err)
		return domain.Account{}, err
	}

	req.Header.Set("User-Agent", constants.USER_AGENT)

	var resp *http.Response
	var data []byte
	ran := m.limiter.Limit(ctx, getAccountMinOperationTime, func(ctx context.Context) {
		ctx, span := m.tracer.Start(ctx, "MojangAPI.getProfile")
		defer span.End()

		resp, err = m.httpClient.Do(req)
		if err != nil {
			err := fmt.Errorf("failed to send request: %w", err)
			reporting.Report(ctx, err)
			return
		}

		defer resp.Body.Close()
		data, err = io.ReadAll(resp.Body)
		if err != nil {
			err := fmt.Errorf("failed to read response body: %w", err)
			reporting.Report(ctx, err)
			return
		}
	})
	if !ran {
		return domain.Account{}, fmt.Errorf("%w: too many requests to mojang API", domain.ErrTemporarilyUnavailable)
	}

	if err != nil {
		return domain.Account{}, err
	}

	identity, err := accountFromMojangResponse(resp.StatusCode, data, m.nowFunc())
	if err != nil {
		if errors.Is(err, domain.ErrUsernameNotFound) {
			// Pass through error but don't report
			return domain.Account{}, err
		}

		err := fmt.Errorf("failed to get identity from mojang response: %w", err)
		extra := map[string]string{
			"data":   string(data),
			"status": strconv.Itoa(resp.StatusCode),
		}
		for header, values := range resp.Header {
			switch len(values) {
			case 0:
				extra["header_"+header] = "<empty slice>"
			case 1:
				extra["header_"+header] = values[0]
			default:
				extra["header_"+header] = fmt.Sprintf("list: %v", values)
			}
		}
		reporting.Report(ctx, err, extra)
		return domain.Account{}, err
	}

	return identity, nil
}

type mojangResponse struct {
	UUID     string `json:"id"`
	Username string `json:"name"`
}

func accountFromMojangResponse(statusCode int, data []byte, queriedAt time.Time) (domain.Account, error) {
	switch statusCode {
	case http.StatusTooManyRequests,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return domain.Account{}, fmt.Errorf("%w: mojang API returned status code %d", domain.ErrTemporarilyUnavailable, statusCode)
	}

	switch statusCode {
	case http.StatusNotFound,
		http.StatusNoContent:
		return domain.Account{}, domain.ErrUsernameNotFound
	}

	var response mojangResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return domain.Account{}, fmt.Errorf("failed to parse mojang response: %w", err)
	}

	uuid, err := strutils.NormalizeUUID(response.UUID)
	if err != nil {
		return domain.Account{}, fmt.Errorf("failed to normalize UUID from mojang: %w", err)
	}

	return domain.Account{
		Username:  response.Username,
		UUID:      uuid,
		QueriedAt: queriedAt,
	}, nil
}

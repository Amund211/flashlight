package accountprovider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/Amund211/flashlight/internal/constants"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

const getAccountMaxOperationTime = ratelimiting.MaxOperationTime(2 * time.Second)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type RequestLimiter interface {
	Limit(ctx context.Context, maxOperationTime ratelimiting.MaxOperationTime, operation func()) error
}

type Mojang struct {
	httpClient HttpClient
	limiter    RequestLimiter
	nowFunc    func() time.Time
}

func NewMojang(httpClient HttpClient, limiter RequestLimiter, nowFunc func() time.Time) *Mojang {
	return &Mojang{
		httpClient: httpClient,
		limiter:    limiter,
		nowFunc:    nowFunc,
	}
}

func (m *Mojang) GetAccountByUUID(ctx context.Context, uuid string) (domain.Account, error) {
	var account domain.Account
	var err error
	waitErr := m.limiter.Limit(ctx, getAccountMaxOperationTime, func() {
		account, err = m.getProfile(ctx, fmt.Sprintf("https://api.minecraftservices.com/minecraft/profile/lookup/%s", uuid))
	})
	if errors.Is(waitErr, context.DeadlineExceeded) {
		return domain.Account{}, fmt.Errorf("%w: too many requests to mojang API", domain.ErrTemporarilyUnavailable)
	}
	if waitErr != nil {
		reporting.Report(ctx, waitErr)
		return domain.Account{}, fmt.Errorf("failed to wait for rate limiter: %w", waitErr)
	}

	if err != nil {
		return domain.Account{}, fmt.Errorf("failed to look up mojang profile by id: %w", err)
	}

	return account, nil
}

func (m *Mojang) GetAccountByUsername(ctx context.Context, username string) (domain.Account, error) {
	var account domain.Account
	var err error
	waitErr := m.limiter.Limit(ctx, getAccountMaxOperationTime, func() {
		account, err = m.getProfile(ctx, fmt.Sprintf("https://api.minecraftservices.com/minecraft/profile/lookup/name/%s", username))
	})
	if errors.Is(waitErr, context.DeadlineExceeded) {
		return domain.Account{}, fmt.Errorf("%w: too many requests to mojang API", domain.ErrTemporarilyUnavailable)
	}
	if waitErr != nil {
		reporting.Report(ctx, waitErr)
		return domain.Account{}, fmt.Errorf("failed to wait for rate limiter: %w", waitErr)
	}

	if err != nil {
		return domain.Account{}, fmt.Errorf("failed to look up mojang profile by name: %w", err)
	}

	return account, nil
}

func (m *Mojang) getProfile(ctx context.Context, url string) (domain.Account, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		err := fmt.Errorf("failed to create request: %w", err)
		reporting.Report(ctx, err)
		return domain.Account{}, err
	}

	req.Header.Set("User-Agent", constants.USER_AGENT)

	resp, err := m.httpClient.Do(req)
	if err != nil {
		err := fmt.Errorf("failed to send request: %w", err)
		reporting.Report(ctx, err)
		return domain.Account{}, err
	}

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		err := fmt.Errorf("failed to read response body: %w", err)
		reporting.Report(ctx, err)
		return domain.Account{}, err
	}

	identity, err := accountFromMojangResponse(resp.StatusCode, data, m.nowFunc())
	if err != nil {
		if errors.Is(err, domain.ErrUsernameNotFound) {
			// Pass through error but don't report
			return domain.Account{}, err
		}

		err := fmt.Errorf("failed to get identity from mojang response: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"data":   string(data),
			"status": strconv.Itoa(resp.StatusCode),
		})
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

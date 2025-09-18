package playerprovider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Amund211/flashlight/internal/config"
	"github.com/Amund211/flashlight/internal/constants"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/reporting"
)

type HttpClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type HypixelAPI interface {
	GetPlayerData(ctx context.Context, uuid string) ([]byte, int, time.Time, error)
}

type mockedHypixelAPI struct{}

func (hypixelAPI *mockedHypixelAPI) GetPlayerData(ctx context.Context, uuid string) ([]byte, int, time.Time, error) {
	return []byte(fmt.Sprintf(`{"success":true,"player":{"uuid":"%s"}}`, uuid)), 200, time.Now(), nil
}

type hypixelAPIImpl struct {
	httpClient HttpClient
	apiKey     string
}

func (hypixelAPI hypixelAPIImpl) GetPlayerData(ctx context.Context, uuid string) ([]byte, int, time.Time, error) {
	logger := logging.FromContext(ctx)
	url := fmt.Sprintf("https://api.hypixel.net/player?uuid=%s", uuid)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		err := fmt.Errorf("failed to create request: %w", err)
		logger.Error(err.Error())
		reporting.Report(ctx, err)
		return []byte{}, -1, time.Time{}, err
	}

	req.Header.Set("User-Agent", constants.USER_AGENT)
	req.Header.Set("API-Key", hypixelAPI.apiKey)

	start := time.Now()
	resp, err := hypixelAPI.httpClient.Do(req)
	if err != nil {
		err := fmt.Errorf("failed to send request: %w", err)
		logger.Error(err.Error())
		reporting.Report(ctx, err)
		return []byte{}, -1, time.Time{}, err
	}

	queriedAt := time.Now()

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		err := fmt.Errorf("failed to read response body: %w", err)
		logger.Error(err.Error())
		reporting.Report(ctx, err)
		return []byte{}, -1, time.Time{}, err
	}
	logging.FromContext(ctx).Info("hypixel request completed", "url", url, "status", resp.StatusCode, "duration", time.Since(start).String())

	return data, resp.StatusCode, queriedAt, nil
}

func NewHypixelAPI(httpClient HttpClient, apiKey string) HypixelAPI {
	return hypixelAPIImpl{
		httpClient: httpClient,
		apiKey:     apiKey,
	}
}

func NewHypixelAPIOrMock(config config.Config, httpClient HttpClient) (HypixelAPI, error) {
	if config.HypixelAPIKey() != "" {
		return NewHypixelAPI(httpClient, config.HypixelAPIKey()), nil
	}
	if config.IsDevelopment() {
		return &mockedHypixelAPI{}, nil
	}
	return nil, fmt.Errorf("Missing Hypixel API key in non-development environment")
}

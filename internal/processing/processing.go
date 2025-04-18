package processing

import (
	"context"
	"fmt"
	"net/http"

	e "github.com/Amund211/flashlight/internal/errors"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/reporting"
)

func checkForHypixelError(ctx context.Context, statusCode int, playerData []byte) error {
	// Only support 200 OK
	if statusCode == 200 {
		// Check for HTML response
		if len(playerData) > 0 && playerData[0] == '<' {
			return fmt.Errorf("%w: Hypixel API returned HTML %w", e.APIServerError, e.RetriableError)
		}

		return nil
	}

	// Error for unknown status code
	err := fmt.Errorf("%w: Hypixel API returned unsupported status code: %d", e.APIServerError, statusCode)

	// Errors for known status codes
	switch statusCode {
	case 429:
		err = fmt.Errorf("%w: Hypixel ratelimit exceeded %w", e.RatelimitExceededError, e.RetriableError)
	case 500, 502, 503, 504, 520, 521, 522, 523, 524, 525, 526, 527, 530:
		err = fmt.Errorf("%w: Hypixel returned status code %d (%s) %w", e.APIServerError, statusCode, http.StatusText(statusCode), e.RetriableError)
	}

	return err
}

func ParseHypixelAPIResponse(ctx context.Context, playerData []byte, statusCode int) (*hypixelAPIResponse, int, error) {
	err := checkForHypixelError(ctx, statusCode, playerData)
	if err != nil {
		reporting.Report(
			ctx,
			err,
			map[string]string{
				"statusCode": fmt.Sprint(statusCode),
				"data":       string(playerData),
			},
		)
		logging.FromContext(ctx).Error(
			"Got response from hypixel",
			"status", "error",
			"error", err.Error(),
			"data", string(playerData),
			"statusCode", statusCode,
			"contentLength", len(playerData),
		)
		return nil, -1, err
	}

	logging.FromContext(ctx).Info(
		"Got response from hypixel",
		"status", "success",
		"statusCode", statusCode,
		"contentLength", len(playerData),
	)

	parsedPlayerData, err := ParsePlayerData(ctx, playerData)
	if err != nil {
		err = fmt.Errorf("%w: failed to parse player data: %w", e.APIServerError, err)
		reporting.Report(
			ctx,
			err,
			map[string]string{
				"statusCode": fmt.Sprint(statusCode),
				"data":       string(playerData),
			},
		)
		return nil, -1, err
	}

	processedStatusCode := 200
	if parsedPlayerData.Success && parsedPlayerData.Player == nil {
		processedStatusCode = 404
	}

	return parsedPlayerData, processedStatusCode, nil
}

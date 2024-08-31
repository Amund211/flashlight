package processing

import (
	"context"
	"fmt"

	e "github.com/Amund211/flashlight/internal/errors"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/reporting"
)

func checkForHypixelError(ctx context.Context, statusCode int, playerData []byte) error {
	// Non-error status codes - check for HTML
	if statusCode <= 400 || statusCode == 404 {
		if len(playerData) > 0 && playerData[0] == '<' {
			return fmt.Errorf("%w: Hypixel API returned HTML", e.APIServerError)
		}

		return nil
	}

	err := fmt.Errorf("%w: Hypixel API failed (status code: %d)", e.APIServerError, statusCode)

	// Pass through certain status codes
	switch statusCode {
	case 429:
		err = fmt.Errorf("%w: Hypixel ratelimit exceeded", e.RatelimitExceededError)
	case 502:
		err = fmt.Errorf("%w: Hypixel returned 502 Bad Gateway", e.BadGateway)
	case 503:
		err = fmt.Errorf("%w: Hypixel returned 503 Service Unavailable", e.ServiceUnavailable)
	case 504:
		err = fmt.Errorf("%w: Hypixel returned 504 Gateway Timeout", e.GatewayTimeout)
	}

	return err
}

func ProcessPlayerData(ctx context.Context, playerData []byte, statusCode int) ([]byte, int, error) {
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
		return []byte{}, -1, err
	}

	logging.FromContext(ctx).Error(
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
		return []byte{}, -1, err
	}

	minifiedPlayerData, err := MarshalPlayerData(ctx, parsedPlayerData)
	if err != nil {
		err = fmt.Errorf("%w: failed to marshal player data: %w", e.APIServerError, err)
		reporting.Report(
			ctx,
			err,
			map[string]string{
				"statusCode": fmt.Sprint(statusCode),
				"data":       string(playerData),
			},
		)
		return []byte{}, -1, err
	}

	return minifiedPlayerData, statusCode, nil
}

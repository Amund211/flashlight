package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Amund211/flashlight/internal/adapters/cache"
	"github.com/Amund211/flashlight/internal/adapters/uuidprovider"
	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
)

type GetUUID func(ctx context.Context, username string) (string, error)

type usernameRepository interface {
	StoreUsername(ctx context.Context, uuid string, queriedAt time.Time, username string) error
	RemoveUsername(ctx context.Context, username string) error
	GetUUID(ctx context.Context, username string) (string, time.Time, error)
}

func buildGetUUIDWithoutCache(
	provider uuidprovider.UUIDProvider,
	repo usernameRepository,
	nowFunc func() time.Time,
) func(ctx context.Context, username string) (string, error) {
	return func(ctx context.Context, username string) (string, error) {
		repoUUID, queriedAt, repoGetErr := repo.GetUUID(ctx, username)
		if errors.Is(repoGetErr, domain.ErrUsernameNotFound) {
			// No entry in the repo - try to query the provider
		} else if repoGetErr != nil {
			// Failed to get UUID from repository - can still try to query the provider
			// NOTE: usernameRepository implementations handle their own error reporting
		} else {
			// time.Since(queriedAt) implemented using nowFunc()
			repoUUIDAge := nowFunc().Sub(queriedAt)
			if repoUUIDAge < 10*24*time.Hour {
				if !strutils.UUIDIsNormalized(repoUUID) {
					err := fmt.Errorf("UUID from repo is not normalized")
					reporting.Report(ctx, err, map[string]string{
						"uuid": repoUUID,
					})
					// We can still try to query the provider
				} else {
					// We have a valid, recent UUID from the repository, return it
					return repoUUID, nil
				}
			}
		}

		identity, err := provider.GetUUID(ctx, username)
		if errors.Is(err, domain.ErrUsernameNotFound) {
			removeUsernameErr := repo.RemoveUsername(ctx, username)
			if removeUsernameErr != nil {
				// NOTE: usernameRepository implementations handle their own error reporting
				// Still fall through to return the ErrUsernameNotFound
			}

			// Pass through ErrUsernameNotFound to the caller
			return "", err
		} else if err != nil {
			// NOTE: UUIDProvider implementations handle their own error reporting

			// Try to fall back to the repository result, if available
			if repoGetErr == nil {
				// time.Since(queriedAt) implemented using nowFunc()
				repoUUIDAge := nowFunc().Sub(queriedAt)
				// 30 day name change interval + 7 days grace period of reclaiming your name
				if repoUUIDAge < 37*24*time.Hour {
					if !strutils.UUIDIsNormalized(repoUUID) {
						err := fmt.Errorf("UUID from repo is not normalized")
						reporting.Report(ctx, err, map[string]string{
							"uuid": repoUUID,
						})
					} else {
						// We have a valid, recent-ish UUID from the repository, return it
						return repoUUID, nil
					}
				}

			}
			return "", fmt.Errorf("could not get uuid for username: %w", err)
		}

		if !strutils.UUIDIsNormalized(identity.UUID) {
			err := fmt.Errorf("UUID is not normalized")
			reporting.Report(ctx, err, map[string]string{
				"uuid": identity.UUID,
			})
			return "", err
		}

		err = repo.StoreUsername(ctx, identity.UUID, nowFunc(), identity.Username)
		if err != nil {
			// NOTE: This error is not critical, we can still return the UUID
		}

		return identity.UUID, nil
	}
}

func BuildGetUUIDWithCache(
	uuidCache cache.Cache[string],
	provider uuidprovider.UUIDProvider,
	repo usernameRepository,
	nowFunc func() time.Time,
) GetUUID {
	getUUIDWithoutCache := buildGetUUIDWithoutCache(provider, repo, nowFunc)

	return func(ctx context.Context, username string) (string, error) {
		usernameLength := len(username)
		if usernameLength == 0 || usernameLength > 100 {
			err := fmt.Errorf("invalid username length")
			reporting.Report(ctx, err, map[string]string{
				"username": username,
				"length":   strconv.Itoa(usernameLength),
			})
			return "", err
		}

		uuid, err := cache.GetOrCreate(ctx, uuidCache, username, func() (string, error) {
			return getUUIDWithoutCache(ctx, username)
		})
		if err != nil {
			// NOTE: GetOrCreate only returns an error if create() fails.
			// getUUIDWithoutCache handles its own error reporting
			return "", fmt.Errorf("failed to cache.GetOrCreate uuid for username: %w", err)
		}

		return uuid, nil
	}
}

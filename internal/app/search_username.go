package app

import (
	"context"
	"fmt"
	"time"
)

type SearchUsername func(ctx context.Context, searchTerm string, top int) ([]string, error)

type usernameSearcher interface {
	SearchUsername(ctx context.Context, searchTerm string, top int) ([]string, error)
}

func BuildSearchUsername(repository usernameSearcher) SearchUsername {
	return func(ctx context.Context, searchTerm string, top int) ([]string, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		uuids, err := repository.SearchUsername(ctx, searchTerm, top)
		if err != nil {
			return nil, fmt.Errorf("failed to search username: %w", err)
		}

		return uuids, nil
	}
}

package app

import (
	"context"
	"fmt"
	"time"
)

type SearchUsername func(ctx context.Context, search string, top int) ([]string, error)

type usernameSearcher interface {
	SearchUsername(ctx context.Context, search string, top int) ([]string, error)
}

func BuildSearchUsername(searcher usernameSearcher) SearchUsername {
	return func(ctx context.Context, search string, top int) ([]string, error) {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		uuids, err := searcher.SearchUsername(ctx, search, top)
		if err != nil {
			return nil, fmt.Errorf("could not search username: %w", err)
		}

		return uuids, nil
	}
}

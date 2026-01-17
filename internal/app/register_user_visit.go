package app

import (
	"context"

	"github.com/Amund211/flashlight/internal/domain"
)

type userRepository interface {
	RegisterVisit(ctx context.Context, userID string) (domain.User, error)
}

type RegisterUserVisit func(ctx context.Context, userID string) (domain.User, error)

func BuildRegisterUserVisit(repo userRepository) RegisterUserVisit {
	return func(ctx context.Context, userID string) (domain.User, error) {
		return repo.RegisterVisit(ctx, userID)
	}
}

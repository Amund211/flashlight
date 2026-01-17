package userrepository

import (
	"context"

	"github.com/Amund211/flashlight/internal/domain"
)

type UserRepository interface {
	RegisterVisit(ctx context.Context, userID string) (domain.User, error)
}

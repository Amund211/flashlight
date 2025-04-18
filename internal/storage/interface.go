package storage

import (
	"context"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
)

type StatsPersistor interface {
	StoreStats(ctx context.Context, player *domain.PlayerPIT) error
	GetHistory(ctx context.Context, playerUUID string, start, end time.Time, limit int) ([]domain.PlayerPIT, error)
	GetSessions(ctx context.Context, playerUUID string, start, end time.Time) ([]domain.Session, error)
}

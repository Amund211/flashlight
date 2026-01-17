package userrepository

import (
	"context"
	"fmt"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Postgres struct {
	db     *sqlx.DB
	schema string
	tracer trace.Tracer
}

func NewPostgres(db *sqlx.DB, schema string) *Postgres {
	tracer := otel.Tracer("flashlight/userrepository/postgres")
	return &Postgres{
		db:     db,
		schema: schema,
		tracer: tracer,
	}
}

type dbUser struct {
	UserID      string    `db:"user_id"`
	FirstSeenAt time.Time `db:"first_seen_at"`
	LastSeenAt  time.Time `db:"last_seen_at"`
	SeenCount   int64     `db:"seen_count"`
}

func (p *Postgres) RegisterVisit(ctx context.Context, userID string) (domain.User, error) {
	ctx, span := p.tracer.Start(ctx, "Postgres.RegisterVisit")
	defer span.End()

	if userID == "" {
		err := fmt.Errorf("userID is empty")
		reporting.Report(ctx, err)
		return domain.User{}, err
	}

	now := time.Now()

	var user dbUser
	err := p.db.QueryRowxContext(
		ctx,
		fmt.Sprintf(`INSERT INTO %s.users
		(user_id, first_seen_at, last_seen_at, seen_count)
		VALUES ($1, $2, $2, 1)
		ON CONFLICT (user_id)
		DO UPDATE SET
			last_seen_at = EXCLUDED.last_seen_at,
			seen_count = users.seen_count + 1
		RETURNING user_id, first_seen_at, last_seen_at, seen_count`,
			pq.QuoteIdentifier(p.schema)),
		userID,
		now,
	).Scan(&user.UserID, &user.FirstSeenAt, &user.LastSeenAt, &user.SeenCount)
	if err != nil {
		err := fmt.Errorf("failed to insert or update user: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"userID": userID,
		})
		return domain.User{}, err
	}

	return domain.User{
		UserID:      user.UserID,
		FirstSeenAt: user.FirstSeenAt,
		LastSeenAt:  user.LastSeenAt,
		SeenCount:   user.SeenCount,
	}, nil
}

type StubUserRepository struct{}

func (s *StubUserRepository) RegisterVisit(ctx context.Context, userID string) (domain.User, error) {
	return domain.User{
		UserID:      userID,
		FirstSeenAt: time.Now(),
		LastSeenAt:  time.Now(),
		SeenCount:   1,
	}, nil
}

func NewStubUserRepository() *StubUserRepository {
	return &StubUserRepository{}
}

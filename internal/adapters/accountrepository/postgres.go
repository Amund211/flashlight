package accountrepository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/reporting"
	"github.com/Amund211/flashlight/internal/strutils"
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
	tracer := otel.Tracer("flashlight/accountrepository/postgres")

	return &Postgres{
		db:     db,
		schema: schema,

		tracer: tracer,
	}
}

type dbUsernamesEntry struct {
	PlayerUUID string    `db:"player_uuid"`
	Username   string    `db:"username"`
	QueriedAt  time.Time `db:"queried_at"`
}

type dbUsernameQueriesEntry struct {
	PlayerUUID    string    `db:"player_uuid"`
	Username      string    `db:"username"`
	LastQueriedAt time.Time `db:"last_queried_at"`
}

func (p *Postgres) StoreAccount(ctx context.Context, account domain.Account) error {
	ctx, span := p.tracer.Start(ctx, "Postgres.StoreAccount")
	defer span.End()

	if !strutils.UUIDIsNormalized(account.UUID) {
		err := fmt.Errorf("uuid is not normalized")
		reporting.Report(ctx, err, map[string]string{
			"uuid": account.UUID,
		})
		return err
	}

	txx, err := p.db.BeginTxx(ctx, nil)
	if err != nil {
		err := fmt.Errorf("failed to start transaction: %w", err)
		reporting.Report(ctx, err)
		return err
	}
	defer txx.Rollback()

	_, err = txx.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", pq.QuoteIdentifier(p.schema)))
	if err != nil {
		err := fmt.Errorf("failed to set search path: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"schema": p.schema,
		})
		return err
	}

	// Insert or update entry in username_queries table
	_, err = txx.ExecContext(
		ctx,
		`INSERT INTO username_queries
		(player_uuid, username, last_queried_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (player_uuid, username)
		DO UPDATE SET
			last_queried_at = EXCLUDED.last_queried_at`,
		account.UUID,
		account.Username,
		account.QueriedAt,
	)
	if err != nil {
		err := fmt.Errorf("failed to insert or update username_queries entry: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"uuid":          account.UUID,
			"username":      account.Username,
			"lastQueriedAt": account.QueriedAt.Format(time.RFC3339),
		})
		return err
	}

	// Ensure exclusive access to the usernames table to prevent concurrent writes
	// leading to unique constraint violations
	//
	// NOTE: Using level EXCLUSIVE allows concurrent reads. This can result in misses if a
	// read occurs between the delete and upsert operations.
	// This will only occur if an already existing username is being assigned to a different UUID,
	// which is hopefully an uncommon case.
	//
	// https://www.postgresql.org/docs/current/explicit-locking.html#LOCKING-TABLES
	_, err = txx.ExecContext(ctx, "LOCK TABLE usernames IN EXCLUSIVE MODE")
	if err != nil {
		err := fmt.Errorf("failed to lock usernames table: %w", err)
		reporting.Report(ctx, err)
		return err
	}

	// Remove existing entries with same username case insensitively
	// NOTE: Not deleting entries with the same UUID, as we will update it later if it exists
	_, err = txx.ExecContext(ctx, "DELETE FROM usernames WHERE lower(username) = lower($1) AND player_uuid != $2", account.Username, account.UUID)
	if err != nil {
		err := fmt.Errorf("failed to delete entries with given username (case insensitive): %w", err)
		reporting.Report(ctx, err, map[string]string{
			"username": account.Username,
			"uuid":     account.UUID,
		})
		return err
	}

	// Insert new entry in usernames table
	_, err = txx.ExecContext(
		ctx,
		`INSERT INTO usernames
		(player_uuid, username, queried_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (player_uuid)
		DO UPDATE SET
			username = EXCLUDED.username,
			queried_at = EXCLUDED.queried_at`,
		account.UUID,
		account.Username,
		account.QueriedAt,
	)
	if err != nil {
		err := fmt.Errorf("failed to upsert new usernames entry: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"uuid":      account.UUID,
			"username":  account.Username,
			"queriedAt": account.QueriedAt.Format(time.RFC3339),
		})
		return err
	}

	err = txx.Commit()
	if err != nil {
		err := fmt.Errorf("failed to commit transaction: %w", err)
		reporting.Report(ctx, err)
		return err
	}

	return nil
}

func (p *Postgres) RemoveUsername(ctx context.Context, username string) error {
	ctx, span := p.tracer.Start(ctx, "Postgres.RemoveUsername")
	defer span.End()

	_, err := p.db.ExecContext(ctx, fmt.Sprintf(`
			DELETE FROM %s.usernames
			WHERE lower(username) = lower($1)`,
		pq.QuoteIdentifier(p.schema),
	),
		username,
	)
	if err != nil {
		err := fmt.Errorf("failed to delete username (case insensitive): %w", err)
		reporting.Report(ctx, err, map[string]string{
			"username": username,
		})
		return err
	}

	return nil
}

func (p *Postgres) GetAccountByUUID(ctx context.Context, uuid string) (domain.Account, error) {
	ctx, span := p.tracer.Start(ctx, "Postgres.GetAccountByUUID")
	defer span.End()

	if !strutils.UUIDIsNormalized(uuid) {
		err := fmt.Errorf("uuid is not normalized")
		reporting.Report(ctx, err, map[string]string{
			"uuid": uuid,
		})
		return domain.Account{}, err
	}

	var entry dbUsernamesEntry
	err := p.db.GetContext(ctx, &entry, fmt.Sprintf(`SELECT
		player_uuid, username, queried_at
		FROM %s.usernames
		WHERE player_uuid = $1`,
		pq.QuoteIdentifier(p.schema),
	),
		uuid,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No entry found
			return domain.Account{}, domain.ErrUsernameNotFound
		}
		err := fmt.Errorf("failed to select usernames entry: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"uuid": uuid,
		})
		return domain.Account{}, err
	}

	return domain.Account{
		UUID:      entry.PlayerUUID,
		Username:  entry.Username,
		QueriedAt: entry.QueriedAt,
	}, nil
}

func (p *Postgres) GetAccountByUsername(ctx context.Context, username string) (domain.Account, error) {
	ctx, span := p.tracer.Start(ctx, "Postgres.GetAccountByUsername")
	defer span.End()

	var entry dbUsernamesEntry
	err := p.db.GetContext(ctx, &entry, fmt.Sprintf(`SELECT
		player_uuid, username, queried_at
		FROM %s.usernames
		WHERE lower(username) = lower($1)`,
		pq.QuoteIdentifier(p.schema),
	),
		username,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// No entry found
			return domain.Account{}, domain.ErrUsernameNotFound
		}
		err := fmt.Errorf("failed to select usernames entry: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"username": username,
		})
		return domain.Account{}, err
	}

	if !strutils.UUIDIsNormalized(entry.PlayerUUID) {
		err := fmt.Errorf("uuid is not normalized")
		reporting.Report(ctx, err, map[string]string{
			"uuid": entry.PlayerUUID,
		})
		return domain.Account{}, err
	}

	return domain.Account{
		UUID:      entry.PlayerUUID,
		Username:  entry.Username,
		QueriedAt: entry.QueriedAt,
	}, nil
}

func (p *Postgres) SearchUsername(ctx context.Context, search string, top int) ([]string, error) {
	ctx, span := p.tracer.Start(ctx, "Postgres.SearchUsername")
	defer span.End()

	if top < 1 || top > 100 {
		err := fmt.Errorf("top must be between 1 and 100")
		reporting.Report(ctx, err, map[string]string{
			"top": fmt.Sprintf("%d", top),
		})
		return nil, err
	}

	type result struct {
		PlayerUUID string `db:"player_uuid"`
	}

	var results []result
	err := p.db.SelectContext(ctx, &results, fmt.Sprintf(`
		WITH ranked_matches AS (
			SELECT 
				player_uuid,
				MAX(similarity(username, $1)) as max_similarity,
				MAX(last_queried_at) as max_last_queried_at
			FROM %s.username_queries
			WHERE similarity(username, $1) > 0
			GROUP BY player_uuid
		)
		SELECT player_uuid
		FROM ranked_matches
		ORDER BY max_similarity DESC, max_last_queried_at DESC
		LIMIT $2`,
		pq.QuoteIdentifier(p.schema),
	),
		search,
		top,
	)
	if err != nil {
		err := fmt.Errorf("failed to search username: %w", err)
		reporting.Report(ctx, err, map[string]string{
			"search": search,
			"top":    fmt.Sprintf("%d", top),
		})
		return nil, err
	}

	uuids := make([]string, 0, len(results))
	for _, r := range results {
		if !strutils.UUIDIsNormalized(r.PlayerUUID) {
			err := fmt.Errorf("uuid is not normalized")
			reporting.Report(ctx, err, map[string]string{
				"uuid": r.PlayerUUID,
			})
			return nil, err
		}
		uuids = append(uuids, r.PlayerUUID)
	}

	return uuids, nil
}

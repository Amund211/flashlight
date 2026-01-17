package userrepository

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/Amund211/flashlight/internal/adapters/database"
)

func newPostgres(t *testing.T, db *sqlx.DB, schemaSuffix string) *Postgres {
	require.NotEmpty(t, schemaSuffix, "schemaSuffix must not be empty")
	schema := fmt.Sprintf("users_repo_test_%s", schemaSuffix)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db.MustExec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", pq.QuoteIdentifier(schema)))

	migrator := database.NewDatabaseMigrator(db, logger)

	err := migrator.Migrate(t.Context(), schema)
	require.NoError(t, err)

	return NewPostgres(db, schema)
}

func TestPostgresRegisterVisit(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping db tests in short mode.")
	}
	t.Parallel()

	ctx := t.Context()
	db, err := database.NewPostgresDatabase(database.LOCAL_CONNECTION_STRING)
	require.NoError(t, err)

	getStoredUser := func(t *testing.T, p *Postgres, userID string) *dbUser {
		t.Helper()

		txx, err := db.Beginx()
		require.NoError(t, err)
		defer txx.Rollback()

		_, err = txx.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", pq.QuoteIdentifier(p.schema)))
		require.NoError(t, err)

		var user dbUser
		err = txx.QueryRowxContext(
			ctx,
			"SELECT user_id, first_seen_at, last_seen_at, seen_count FROM users WHERE user_id = $1",
			userID,
		).Scan(&user.UserID, &user.FirstSeenAt, &user.LastSeenAt, &user.SeenCount)
		if err != nil {
			return nil
		}

		return &user
	}

	t.Run("First visit creates new user", func(t *testing.T) {
		t.Parallel()

		p := newPostgres(t, db, "first_visit")
		userID := "test-user-1"

		beforeTime := time.Now()
		user, err := p.RegisterVisit(ctx, userID)
		afterTime := time.Now()

		require.NoError(t, err)
		require.Equal(t, userID, user.UserID)
		require.Equal(t, int64(1), user.SeenCount)
		require.True(t, user.FirstSeenAt.After(beforeTime) || user.FirstSeenAt.Equal(beforeTime))
		require.True(t, user.FirstSeenAt.Before(afterTime) || user.FirstSeenAt.Equal(afterTime))
		require.True(t, user.LastSeenAt.After(beforeTime) || user.LastSeenAt.Equal(beforeTime))
		require.True(t, user.LastSeenAt.Before(afterTime) || user.LastSeenAt.Equal(afterTime))
		require.Equal(t, user.FirstSeenAt, user.LastSeenAt)

		// Verify in database
		stored := getStoredUser(t, p, userID)
		require.NotNil(t, stored)
		require.Equal(t, userID, stored.UserID)
		require.Equal(t, int64(1), stored.SeenCount)
	})

	t.Run("Second visit updates last_seen_at and increments count", func(t *testing.T) {
		t.Parallel()

		p := newPostgres(t, db, "second_visit")
		userID := "test-user-2"

		// First visit
		user1, err := p.RegisterVisit(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, int64(1), user1.SeenCount)

		// Wait a bit to ensure timestamp difference
		time.Sleep(10 * time.Millisecond)

		// Second visit
		user2, err := p.RegisterVisit(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, userID, user2.UserID)
		require.Equal(t, int64(2), user2.SeenCount)
		require.Equal(t, user1.FirstSeenAt, user2.FirstSeenAt) // First seen should not change
		require.True(t, user2.LastSeenAt.After(user1.LastSeenAt)) // Last seen should be updated

		// Verify in database
		stored := getStoredUser(t, p, userID)
		require.NotNil(t, stored)
		require.Equal(t, int64(2), stored.SeenCount)
	})

	t.Run("Multiple visits increment count correctly", func(t *testing.T) {
		t.Parallel()

		p := newPostgres(t, db, "multiple_visits")
		userID := "test-user-3"

		for i := 1; i <= 5; i++ {
			user, err := p.RegisterVisit(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, int64(i), user.SeenCount)
		}

		// Final verification
		stored := getStoredUser(t, p, userID)
		require.NotNil(t, stored)
		require.Equal(t, int64(5), stored.SeenCount)
	})

	t.Run("Different users are tracked independently", func(t *testing.T) {
		t.Parallel()

		p := newPostgres(t, db, "multiple_users")
		user1ID := "test-user-4"
		user2ID := "test-user-5"

		// User 1 visits twice
		u1v1, err := p.RegisterVisit(ctx, user1ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), u1v1.SeenCount)

		u1v2, err := p.RegisterVisit(ctx, user1ID)
		require.NoError(t, err)
		require.Equal(t, int64(2), u1v2.SeenCount)

		// User 2 visits once
		u2v1, err := p.RegisterVisit(ctx, user2ID)
		require.NoError(t, err)
		require.Equal(t, int64(1), u2v1.SeenCount)

		// Verify both users in database
		stored1 := getStoredUser(t, p, user1ID)
		require.NotNil(t, stored1)
		require.Equal(t, int64(2), stored1.SeenCount)

		stored2 := getStoredUser(t, p, user2ID)
		require.NotNil(t, stored2)
		require.Equal(t, int64(1), stored2.SeenCount)
	})

	t.Run("Empty userID returns error", func(t *testing.T) {
		t.Parallel()

		p := newPostgres(t, db, "empty_userid")

		_, err := p.RegisterVisit(ctx, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "userID is empty")
	})

	t.Run("Special characters in userID are handled correctly", func(t *testing.T) {
		t.Parallel()

		p := newPostgres(t, db, "special_chars")
		userID := "user@example.com"

		user, err := p.RegisterVisit(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, userID, user.UserID)
		require.Equal(t, int64(1), user.SeenCount)

		stored := getStoredUser(t, p, userID)
		require.NotNil(t, stored)
		require.Equal(t, userID, stored.UserID)
	})
}

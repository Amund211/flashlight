package userrepository

import (
	"fmt"
	"log/slog"
	"os"
	"testing"
	"testing/synctest"
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

	getStoredUser := func(t *testing.T, db *sqlx.DB, schema string, userID string) *dbUser {
		t.Helper()

		ctx := t.Context()

		txx, err := db.Beginx()
		require.NoError(t, err)
		defer txx.Rollback()

		_, err = txx.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s", pq.QuoteIdentifier(schema)))
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
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()

			db, err := database.NewPostgresDatabase(database.LOCAL_CONNECTION_STRING)
			require.NoError(t, err)
			schema := "first_visit"
			p := newPostgres(t, db, schema)
			userID := "test-user-1"

			start := time.Now()

			user, err := p.RegisterVisit(ctx, userID)
			require.NoError(t, err)

			require.Equal(t, userID, user.UserID)
			require.Equal(t, int64(1), user.SeenCount)
			require.WithinDuration(t, start, user.FirstSeenAt, time.Millisecond)
			require.WithinDuration(t, start, user.LastSeenAt, time.Millisecond)
			require.Equal(t, user.FirstSeenAt, user.LastSeenAt)

			// Verify in database
			stored := getStoredUser(t, db, schema, userID)
			require.NotNil(t, stored)
			require.Equal(t, userID, stored.UserID)
			require.Equal(t, int64(1), stored.SeenCount)
			require.WithinDuration(t, start, stored.FirstSeenAt, time.Millisecond)
			require.WithinDuration(t, start, stored.LastSeenAt, time.Millisecond)
			require.Equal(t, stored.FirstSeenAt, stored.LastSeenAt)
		})
	})

	t.Run("Second visit updates last_seen_at and increments count", func(t *testing.T) {
		t.Parallel()
		t.Skip("flaky test, needs investigation")
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()

			db, err := database.NewPostgresDatabase(database.LOCAL_CONNECTION_STRING)
			require.NoError(t, err)
			schema := "second_visit"
			p := newPostgres(t, db, schema)
			userID := "test-user-2"

			first := time.Now()

			// First visit
			user1, err := p.RegisterVisit(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, int64(1), user1.SeenCount)
			require.WithinDuration(t, first, user1.FirstSeenAt, time.Millisecond)
			require.WithinDuration(t, first, user1.LastSeenAt, time.Millisecond)

			time.Sleep(1 * time.Minute)

			second := time.Now()

			// Second visit
			user2, err := p.RegisterVisit(ctx, userID)
			require.NoError(t, err)
			require.Equal(t, userID, user2.UserID)
			require.Equal(t, int64(2), user2.SeenCount)
			require.Equal(t, user1.FirstSeenAt, user2.FirstSeenAt) // First seen should not change
			require.WithinDuration(t, second, user2.FirstSeenAt, time.Millisecond)

			// Verify in database
			stored := getStoredUser(t, db, schema, userID)
			require.NotNil(t, stored)
			require.Equal(t, userID, stored.UserID)
			require.Equal(t, int64(2), stored.SeenCount)
			require.Equal(t, first, stored.FirstSeenAt)
			require.WithinDuration(t, second, stored.LastSeenAt, time.Millisecond)
		})
	})

	t.Run("Multiple visits increment count correctly", func(t *testing.T) {
		t.Parallel()
		t.Skip("flaky test, needs investigation")
		synctest.Test(t, func(t *testing.T) {
			ctx := t.Context()

			db, err := database.NewPostgresDatabase(database.LOCAL_CONNECTION_STRING)
			require.NoError(t, err)
			schema := "multiple_visits"
			p := newPostgres(t, db, schema)
			userID := "test-user-3"

			start := time.Now()

			for i := range 5 {
				now := time.Now()
				user, err := p.RegisterVisit(ctx, userID)
				require.NoError(t, err)

				require.Equal(t, int64(i+1), user.SeenCount)
				require.WithinDuration(t, start, user.FirstSeenAt, time.Millisecond)
				require.WithinDuration(t, now, user.LastSeenAt, time.Millisecond)

				time.Sleep(1 * time.Hour)
			}

			// Final verification
			stored := getStoredUser(t, db, schema, userID)
			require.NotNil(t, stored)
			require.Equal(t, int64(5), stored.SeenCount)
		})
	})

	t.Run("Different users are tracked independently", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		db, err := database.NewPostgresDatabase(database.LOCAL_CONNECTION_STRING)
		require.NoError(t, err)
		schema := "multiple_users"
		p := newPostgres(t, db, schema)
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
		stored1 := getStoredUser(t, db, schema, user1ID)
		require.NotNil(t, stored1)
		require.Equal(t, int64(2), stored1.SeenCount)

		stored2 := getStoredUser(t, db, schema, user2ID)
		require.NotNil(t, stored2)
		require.Equal(t, int64(1), stored2.SeenCount)
	})

	t.Run("Empty userID returns error", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		db, err := database.NewPostgresDatabase(database.LOCAL_CONNECTION_STRING)
		require.NoError(t, err)
		p := newPostgres(t, db, "empty_userid")

		_, err = p.RegisterVisit(ctx, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "userID is empty")
	})

	t.Run("Special characters in userID are handled correctly", func(t *testing.T) {
		t.Parallel()
		ctx := t.Context()

		db, err := database.NewPostgresDatabase(database.LOCAL_CONNECTION_STRING)
		require.NoError(t, err)
		schema := "special_chars"
		p := newPostgres(t, db, schema)
		userID := "user@example.com"

		user, err := p.RegisterVisit(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, userID, user.UserID)
		require.Equal(t, int64(1), user.SeenCount)

		stored := getStoredUser(t, db, schema, userID)
		require.NotNil(t, stored)
		require.Equal(t, userID, stored.UserID)
	})
}

package database

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestMigrator(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping migrator tests in short mode.")
	}
	t.Parallel()

	t.Run("migrate up and down", func(t *testing.T) {
		t.Parallel()

		ctx := t.Context()
		schemaName := "migrate_up_down"

		db, err := NewPostgresDatabase(LocalConnectionString)
		require.NoError(t, err)

		db.MustExec(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", pgx.Identifier{schemaName}.Sanitize()))

		logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		migrator := NewDatabaseMigrator(LocalConnectionString, logger)

		err = migrator.migrate(ctx, schemaName)
		require.NoError(t, err, "error migrating up")

		// Migrate down manually
		schemaDB, err := openSchemaScopedDB(LocalConnectionString, schemaName)
		require.NoError(t, err)
		defer schemaDB.Close()

		migrationSource, err := iofs.New(embeddedMigrations, "migrations")
		require.NoError(t, err)
		defer migrationSource.Close()

		dbDriver, err := pgxmigrate.WithInstance(schemaDB, &pgxmigrate.Config{
			DatabaseName: DBName,
			SchemaName:   schemaName,
		})
		require.NoError(t, err)

		migratorInstance, err := migrate.NewWithInstance("iofs", migrationSource, "pgx5", dbDriver)
		require.NoError(t, err)
		defer migratorInstance.Close()

		err = migratorInstance.Down()
		require.NoError(t, err, "error migrating down") // Should not even be ErrNoChange
	})
}

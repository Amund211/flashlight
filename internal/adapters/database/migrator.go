package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

type migrator struct {
	connectionString string

	logger *slog.Logger
}

func NewDatabaseMigrator(connectionString string, logger *slog.Logger) *migrator {
	return &migrator{
		connectionString: connectionString,
		logger:           logger,
	}
}

func (m *migrator) Migrate(ctx context.Context, schemaName string) error {
	return m.migrate(ctx, schemaName)
}

func (m *migrator) migrate(ctx context.Context, schemaName string) error {
	db, err := openSchemaScopedDB(m.connectionString, schemaName)
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pgx.Identifier{schemaName}.Sanitize()))
	if err != nil {
		return fmt.Errorf("migrate: failed to create schema: %w", err)
	}

	migrationSource, err := iofs.New(embeddedMigrations, "migrations")
	if err != nil {
		return fmt.Errorf("migrate: failed to create driver from embedded migrations: %w", err)
	}
	defer migrationSource.Close()

	dbDriver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{
		DatabaseName: DBName,
		SchemaName:   schemaName,
	})
	if err != nil {
		return fmt.Errorf("migrate: failed to create pgx driver: %w", err)
	}

	migratorInstance, err := migrate.NewWithInstance("iofs", migrationSource, "pgx5", dbDriver)
	if err != nil {
		return fmt.Errorf("migrate: failed to create migration instance: %w", err)
	}
	defer migratorInstance.Close()

	m.logger.InfoContext(ctx, "Starting migrations...")
	if err := migratorInstance.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.InfoContext(ctx, "No migrations to run.")
		} else {
			return fmt.Errorf("migrate: failed to migrate: %w", err)
		}
	}
	m.logger.InfoContext(ctx, "Migrations completed successfully.")

	return nil
}

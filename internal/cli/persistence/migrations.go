package persistence

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/lewisedginton/general_purpose_chatbot/pkg/logger"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// MigrationManager demonstrates youcolour migration patterns
type MigrationManager struct {
	db     *sql.DB
	logger logger.Logger
}

// NewMigrationManager creates a migration manager from pgxpool
func NewMigrationManager(pool *pgxpool.Pool, logger logger.Logger) *MigrationManager {
	db := stdlib.OpenDBFromPool(pool)
	return &MigrationManager{
		db:     db,
		logger: logger,
	}
}

// RunMigrations executes pending migrations
func (m *MigrationManager) RunMigrations() error {
	migrator, err := m.createMigrator()
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		_, _ = migrator.Close()
	}()

	m.logger.Info("Starting database migrations")

	err = migrator.Up()
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			m.logger.Info("No new migrations to apply")
			return nil
		}
		m.logger.Error("Failed to run migrations", logger.ErrorField(err))
		return fmt.Errorf("run migrations: %w", err)
	}

	m.logger.Info("Successfully applied migrations")
	return nil
}

func (m *MigrationManager) createMigrator() (*migrate.Migrate, error) {
	sourceDriver, err := iofs.New(migrationFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("create embedded migration source: %w", err)
	}

	driver, err := postgres.WithInstance(m.db, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("create postgres driver: %w", err)
	}

	migrator, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("create migrator: %w", err)
	}

	return migrator, nil
}

// Close closes the database connection
func (m *MigrationManager) Close() error {
	return m.db.Close()
}

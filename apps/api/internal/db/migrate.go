package db

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func Run(databaseURL, schema string, files fs.FS, direction string) error {
	src, err := iofs.New(files, ".")
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toMigrateURL(databaseURL, schema))
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()

	switch {
	case direction == "up":
		err = m.Up()
	case direction == "down":
		err = m.Steps(-1)
	case strings.HasPrefix(direction, "force "):
		var version int
		if _, scanErr := fmt.Sscanf(direction, "force %d", &version); scanErr != nil {
			return fmt.Errorf("force version: %w", scanErr)
		}
		err = m.Force(version)
	default:
		return fmt.Errorf("unknown migrate direction %q", direction)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func toMigrateURL(databaseURL, schema string) string {
	raw := databaseURL
	switch {
	case strings.HasPrefix(databaseURL, "postgres://"):
		raw = "pgx5://" + strings.TrimPrefix(databaseURL, "postgres://")
	case strings.HasPrefix(databaseURL, "postgresql://"):
		raw = "pgx5://" + strings.TrimPrefix(databaseURL, "postgresql://")
	}
	if schema == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	q.Set("search_path", schema)
	q.Set("x-migrations-table", `"`+schema+`"."schema_migrations"`)
	q.Set("x-migrations-table-quoted", "true")
	u.RawQuery = q.Encode()
	return u.String()
}

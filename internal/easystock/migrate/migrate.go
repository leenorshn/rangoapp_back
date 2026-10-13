package migrate

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var Files embed.FS

// Up applies pending *.up.sql scripts in lexical order.
func Up(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version text PRIMARY KEY,
			applied_at timestamptz NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(Files, "sql")
	if err != nil {
		return err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".up.sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	for _, name := range names {
		version := strings.TrimSuffix(name, ".up.sql")
		var applied bool
		if err := pool.QueryRow(ctx, `SELECT true FROM schema_migrations WHERE version = $1`, version).Scan(&applied); err != nil && err != pgx.ErrNoRows {
			return err
		}
		if applied {
			continue
		}
		body, err := Files.ReadFile("sql/" + name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if err := execScript(ctx, tx.Conn(), string(body)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			_ = tx.Rollback(ctx)
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return setRolePasswords(ctx, pool)
}

// setRolePasswords applies APP_DB_PASSWORD, IDENTITY_DB_PASSWORD and
// JOB_DB_PASSWORD when SET_ROLE_PASSWORDS=true. Deployments use it after the
// roles are created without a password. Local tests leave the flag unset.
func setRolePasswords(ctx context.Context, pool *pgxpool.Pool) error {
	if os.Getenv("SET_ROLE_PASSWORDS") != "true" {
		return nil
	}
	roles := []struct {
		name string
		env  string
	}{
		{"easystock_app", "APP_DB_PASSWORD"},
		{"easystock_identity", "IDENTITY_DB_PASSWORD"},
		{"easystock_job", "JOB_DB_PASSWORD"},
	}
	for _, role := range roles {
		password := os.Getenv(role.env)
		if password == "" {
			return fmt.Errorf("%s is required when SET_ROLE_PASSWORDS=true", role.env)
		}
		var stmt string
		if err := pool.QueryRow(ctx,
			`SELECT format('ALTER ROLE %I PASSWORD %L', $1::text, $2::text)`,
			role.name, password,
		).Scan(&stmt); err != nil {
			return fmt.Errorf("role %s: %w", role.name, err)
		}
		if _, err := pool.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("role %s: %w", role.name, err)
		}
		if _, err := fmt.Fprintf(os.Stderr, "password set for role %s\n", role.name); err != nil {
			return err
		}
	}
	return nil
}

func execScript(ctx context.Context, conn *pgx.Conn, sql string) error {
	reader := conn.PgConn().Exec(ctx, sql)
	for reader.NextResult() {
		if _, err := reader.ResultReader().Close(); err != nil {
			_ = reader.Close()
			return err
		}
	}
	return reader.Close()
}

package testutil

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"rangoapp/internal/easystock/migrate"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	superUser = "postgres"
	superPass = "postgres"
	dbName    = "easystock"
	appPass   = "app"
)

var (
	once     sync.Once
	startErr error
	port     uint32
)

// Start boots one PostgreSQL 16 for the process and applies migrations.
func Start() (uint32, error) {
	once.Do(func() {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			startErr = err
			return
		}
		port = uint32(listener.Addr().(*net.TCPAddr).Port)
		_ = listener.Close()

		root, err := os.MkdirTemp("", "easystock-pg-")
		if err != nil {
			startErr = err
			return
		}
		_, file, _, _ := runtime.Caller(0)
		cache := filepath.Join(filepath.Dir(file), "..", "..", "..", ".cache", "postgres")
		if err := os.MkdirAll(cache, 0o755); err != nil {
			startErr = err
			return
		}
		cfg := embeddedpostgres.DefaultConfig().
			Version(embeddedpostgres.V16).
			Port(port).
			Database(dbName).
			Username(superUser).
			Password(superPass).
			Locale("C").
			Encoding("UTF8").
			CachePath(cache).
			RuntimePath(filepath.Join(root, "run")).
			DataPath(filepath.Join(root, "data")).
			StartTimeout(2 * time.Minute)
		db := embeddedpostgres.NewDatabase(cfg)
		if err := db.Start(); err != nil {
			startErr = fmt.Errorf("start postgres: %w", err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		admin, err := pgxpool.New(ctx, dsn(superUser, superPass))
		if err != nil {
			startErr = err
			return
		}
		if err := migrate.Up(ctx, admin); err != nil {
			startErr = err
			admin.Close()
			return
		}
		for _, stmt := range []string{
			`ALTER ROLE easystock_app PASSWORD 'app'`,
			`ALTER ROLE easystock_identity PASSWORD 'identity'`,
			`ALTER ROLE easystock_job PASSWORD 'job'`,
		} {
			if _, err := admin.Exec(ctx, stmt); err != nil {
				startErr = err
				break
			}
		}
		admin.Close()
	})
	return port, startErr
}

func dsn(user, password string) string {
	return fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable", user, password, port, dbName)
}

// Pools returns superuser, application, identity and job pools.
func Pools(t *testing.T) (admin, app, identity, job *pgxpool.Pool) {
	t.Helper()
	if _, err := Start(); err != nil {
		t.Fatalf("postgres: %v", err)
	}
	ctx := context.Background()
	open := func(user, password string) *pgxpool.Pool {
		t.Helper()
		pool, err := pgxpool.New(ctx, dsn(user, password))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(pool.Close)
		return pool
	}
	return open(superUser, superPass), open("easystock_app", appPass), open("easystock_identity", "identity"), open("easystock_job", "job")
}

package dbtest

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver for goose
)

// SetupTestDB starts a PostgreSQL testcontainer, runs goose migrations,
// and returns a *pgx.Conn ready for use. Cleanup happens on test teardown.
func SetupTestDB(t *testing.T) *pgx.Conn {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgis/postgis:16-3.4-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	// Run goose migrations
	migrationsDir := migrationsPath()
	gooseDB, err := goose.OpenDBWithDriver("pgx", connStr)
	if err != nil {
		t.Fatalf("failed to open db for goose: %v", err)
	}
	defer gooseDB.Close()

	if err := goose.Up(gooseDB, migrationsDir); err != nil {
		t.Fatalf("failed to run goose migrations: %v", err)
	}

	// Connect with pgx for tests
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to connect with pgx: %v", err)
	}
	t.Cleanup(func() {
		conn.Close(ctx)
	})

	return conn
}

// migrationsPath returns the absolute path to the migrations directory.
func migrationsPath() string {
	_, filename, _, _ := runtime.Caller(0)
	// Go up from internal/db/dbtest/ to project root
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	dir := filepath.Join(root, "migrations")

	// Verify the directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		panic(fmt.Sprintf("migrations directory not found: %s", dir))
	}
	return dir
}

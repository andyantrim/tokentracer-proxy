package db

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB defines the interface for database operations, compatible with pgxpool.Pool
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	Close()
}

var Pool DB
var Repo Repository

func InitDB() error {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return fmt.Errorf("DATABASE_URL environment variable is required")
	}

	if !strings.HasSuffix(dbURL, "/tokentracer") {
		dbURL = dbURL + "/tokentracer"
	}

	var err error
	var realPool *pgxpool.Pool
	realPool, err = pgxpool.New(context.Background(), dbURL)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %v", err)
	}
	Pool = realPool
	Repo = NewPostgresRepository(Pool)

	return nil
}

func CloseDB() {
	if Pool != nil {
		Pool.Close()
	}
}

package connect

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

// MustDatabase connects to a database and fails if a connection can't
// be established.
func MustDatabase(url string) *pgxpool.Pool {
	db, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}

	if err = db.Ping(context.Background()); err != nil {
		log.Fatalf("error pinging database: %v", err)
	}

	return db
}

// DatabaseOption allows the caller of MustDatabaseWithConfig to override
// fields on the *pgxpool.Config struct.
type DatabaseOption func(*pgxpool.Config)

// MustDatatabaseWithConfig connects to a database and fails if a connection can't
// be established.
func MustDatatabaseWithConfig(url string, opts ...DatabaseOption) *pgxpool.Pool {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		log.Fatalf("error parsing db config: %v", err)
	}

	for _, opt := range opts {
		opt(cfg)
	}

	db, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	defer db.Close()

	if err = db.Ping(context.Background()); err != nil {
		log.Fatalf("error pinging database: %v", err)
	}

	return db
}

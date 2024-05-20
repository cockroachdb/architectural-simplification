package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	if err := generateOrders(db); err != nil {
		log.Fatalf("error generating orders: %v", err)
	}
}

func generateOrders(db *pgxpool.Pool) error {
	const stmt = `INSERT INTO orders (customer_id, total, ts) VALUES ($1, $2, $3)`
	for range time.NewTicker(time.Millisecond * 10).C {
		total := rand.Float64() * 100
		if _, err := db.Exec(context.Background(), stmt, uuid.NewString(), total, randomDate()); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}
	}

	return fmt.Errorf("finished generateOrders unexpectedly")
}

func randomDate() time.Time {
	start := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Now()

	diff := end.Sub(start).Seconds()
	randomSeconds := rand.Float64() * diff
	randomDuration := time.Duration(randomSeconds) * time.Second

	return start.Add(randomDuration)
}

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	url, ok := os.LookupEnv("CONNECTION_STRING")
	if !ok {
		log.Fatalf("missing CONNECTION_STRING env var")
	}

	db := connect.MustDatabase(url)
	defer db.Close()

	work(db)
}

func work(db *pgxpool.Pool) {
	for range time.NewTicker(time.Second).C {
		id := uuid.NewString()

		// Insert purchase.
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		defer cancel()

		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			stmt := `INSERT INTO purchase (id, basket_id, member_id, amount) VALUES ($1, $2, $3, $4)`
			if _, err := db.Exec(context.Background(), stmt, id, uuid.NewString(), uuid.NewString(), rand.Float64()*100); err != nil {
				return fmt.Errorf("inserting purchase: %w", err)
			}

			// Select purchase.
			stmt = `SELECT amount FROM purchase WHERE id = $1`
			row := db.QueryRow(ctx, stmt, id)

			var value float64
			if err := row.Scan(&value); err != nil {
				return fmt.Errorf("selecting purchase: %w", err)
			}

			// Feedback.
			log.Println("ok")
			return nil
		})

		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
	}
}

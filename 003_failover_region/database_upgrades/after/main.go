package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	db := connect.MustDatabase("postgres://root@cockroachdb-public.crdb.svc.cluster.local:26257/defaultdb?sslmode=disable")
	defer db.Close()

	work(db)
}

func work(db *pgxpool.Pool) {
	for range time.NewTicker(time.Second).C {
		id := uuid.NewString()

		ctx, _ := context.WithTimeout(context.Background(), time.Second*3)

		err := crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			// Insert purchase.
			stmt := `INSERT INTO purchase (id, basket_id, member_id, amount) VALUES ($1, $2, $3, $4)`
			if _, err := db.Exec(context.Background(), stmt, id, uuid.NewString(), uuid.NewString(), rand.Float64()*100); err != nil {
				return fmt.Errorf("inserting purchase: %w", err)
			}

			// Select purchase.
			stmt = `SELECT amount FROM purchase WHERE id = $1`
			row := db.QueryRow(context.Background(), stmt, id)

			var value float64
			if err := row.Scan(&value); err != nil {
				return fmt.Errorf("selecting purchase: %w", err)
			}

			// Select database version.
			stmt = `SELECT version()`
			row = db.QueryRow(context.Background(), stmt)

			var version string
			if err := row.Scan(&version); err != nil {
				return fmt.Errorf("selecting version: %w", err)
			}

			// Feedback.
			log.Printf("ok (%s)", strings.Split(version, "(")[0])

			return nil
		})

		if err != nil {
			log.Println(err)
		}
	}
}

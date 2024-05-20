package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const letters = "abcdefghijklmnopqrstuvwxyz"

func main() {
	url, ok := os.LookupEnv("CONNECTION_STRING")
	if !ok {
		log.Fatalf("missing CONNECTION_STRING env var")
	}

	db := connect.MustDatabase(url)
	defer db.Close()

	readWrites(db)
}

func readWrites(db *pgxpool.Pool) {
	for range time.NewTicker(time.Millisecond * 50).C {
		if err := readWrite(db); err != nil {
			log.Printf("error: %v", err)
		}
	}
}

func readWrite(db *pgxpool.Pool) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	return crdbpgx.ExecuteTx(ctx, db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		// Write.
		const writeStmt = `INSERT INTO product (name, price) VALUES ($1, $2)
												 RETURNING id`

		name := mustRandomString(10)
		price := round(rand.Float64()*100, 2)

		row := db.QueryRow(ctx, writeStmt, name, price)

		var id string
		if err := row.Scan(&id); err != nil {
			return fmt.Errorf("inserting product: %w", err)
		}

		// Read.
		const readStmt = `SELECT name, price FROM product WHERE id = $1`

		ctx, cancel = context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		row = db.QueryRow(context.Background(), readStmt, id)

		var dbName string
		var dbPrice float64
		if err := row.Scan(&dbName, &dbPrice); err != nil {
			return fmt.Errorf("reading product: %w", err)
		}

		// Print
		fmt.Printf("inserted %s\n", id)
		return nil
	})
}

func mustRandomString(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func round(val float64, precision int) float64 {
	return math.Round(val*(math.Pow10(precision))) / math.Pow10(precision)
}

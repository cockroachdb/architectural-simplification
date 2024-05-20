package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	url := flag.String("url", "", "database connection string")
	flag.Parse()

	if *url == "" {
		flag.Usage()
		os.Exit(2)
	}

	db := connect.MustDatabase(*url)
	defer db.Close()

	work(db)
}

func work(db *pgxpool.Pool) {
	for range time.NewTicker(time.Second * 1).C {
		const stmt = `SELECT COUNT(*) FROM customer`

		start := time.Now()
		row := db.QueryRow(context.Background(), stmt)

		var count int
		if err := row.Scan(&count); err != nil {
			log.Printf("error getting row count: %v", err)
		}

		log.Printf("customers: %d (took %s)", count, time.Since(start))
	}
}

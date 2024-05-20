package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	products = []string{
		"93410c29-1609-484d-8662-ae2d0aa93cc4",
		"47b0472d-708c-4377-aab4-acf8752f0ecb",
		"a1a879d8-58c0-4357-a570-a57c3b1fe059",
		"5ded80d3-fb55-4a2f-b339-43fc9c89894a",
		"b6afe0c5-9cab-4971-8c61-127fe5b4acd1",
		"7098227b-4883-4992-bc32-e12335efbc8c",
	}

	reads  uint64
	writes uint64
)

func main() {
	readInterval := flag.Duration("r", time.Millisecond*100, "interval between reads")
	writeInterval := flag.Duration("w", time.Millisecond*1000, "interval between writes")
	flag.Parse()

	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	go simulateReads(db, *readInterval)
	go simulateWrites(db, *writeInterval)
	printLoop()
}

func simulateReads(db *pgxpool.Pool, rate time.Duration) error {
	for range time.NewTicker(rate).C {
		if err := simulateRead(db); err != nil {
			log.Printf("error simulating read: %v", err)
		}

		atomic.AddUint64(&reads, 1)
	}

	return fmt.Errorf("finished simulateReads unexectedly")
}

func simulateRead(db *pgxpool.Pool) error {
	// Pick a product.
	productID := products[rand.Intn(len(products))]

	// Read stock from database.
	_, err := readFromDB(db, productID)
	if err != nil {
		return fmt.Errorf("reading from db: %w", err)
	}

	return nil
}

func simulateWrites(db *pgxpool.Pool, rate time.Duration) error {
	stock := 0
	for range time.NewTicker(rate).C {
		stock++

		if err := simulateWrite(db, stock); err != nil {
			log.Printf("error simulating write: %v", err)
		}

		atomic.AddUint64(&writes, 1)
	}

	return fmt.Errorf("finished simulateWrites unexectedly")
}

func simulateWrite(db *pgxpool.Pool, stock int) error {
	// Pick a product.
	productID := products[rand.Intn(len(products))]

	// Update stock in database but "fail" 1% of the time.
	if rand.Intn(100) < 99 {
		const stmt = `UPDATE stock SET quantity = $1 WHERE product_id = $2`
		if _, err := db.Exec(context.Background(), stmt, stock, productID); err != nil {
			return fmt.Errorf("updating database stock: %w", err)
		}
	}

	return nil
}

func readFromDB(db *pgxpool.Pool, productID string) (int, error) {
	const stmt = `SELECT quantity FROM stock WHERE product_id = $1`

	row := db.QueryRow(context.Background(), stmt, productID)

	var quantity int
	if err := row.Scan(&quantity); err != nil {
		return 0, fmt.Errorf("getting stock from database: %w", err)
	}

	return quantity, nil
}

func printLoop() {
	for range time.NewTicker(time.Millisecond * 100).C {
		fmt.Println("\033[H\033[2J")
		fmt.Printf("reads:  %d\n", atomic.LoadUint64(&reads))
		fmt.Printf("writes: %d\n", atomic.LoadUint64(&writes))
	}
}

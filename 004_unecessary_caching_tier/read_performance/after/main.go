package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	products = mustLoadIDs("004_unecessary_caching_tier/read_performance/ids.json")
)

func main() {
	writeInterval := flag.Duration("w", time.Millisecond*100, "interval between writes")
	flag.Parse()

	db := connect.MustDatatabaseWithConfig(
		"postgres://root@localhost:26257/defaultdb?sslmode=disable",
		func(cfg *pgxpool.Config) {
			cfg.MaxConns = 50
		})

	router := fiber.New()
	router.Get("/products/:id/stock", handleGetProduct(db))
	go func() {
		log.Fatal(router.Listen(":3000"))
	}()

	simulateWrites(db, *writeInterval)
}

func mustLoadIDs(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		log.Fatalf("error opening ids file for reading: %v", err)
	}

	var ids []string
	if err = json.NewDecoder(f).Decode(&ids); err != nil {
		log.Fatalf("error parsing ids: %v", err)
	}

	return ids
}

func handleGetProduct(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		productID := c.Params("id")
		if productID == "" {
			return fiber.NewError(fiber.StatusUnprocessableEntity, "missing product id")
		}

		dbQuantity, err := readFromDB(db, productID)
		if err != nil {
			return fmt.Errorf("reading from db: %w", err)
		}

		return c.JSON(fiber.Map{
			"stock": dbQuantity,
		})
	}
}

func simulateWrites(db *pgxpool.Pool, rate time.Duration) error {
	writesMade := 0
	for range time.NewTicker(rate).C {
		if err := simulateWrite(db); err != nil {
			log.Printf("error simulating write: %v", err)
		}

		writesMade++
		fmt.Printf("Writes made: %d\r", writesMade)

	}

	return fmt.Errorf("finished simulateWrites unexectedly")
}

func simulateWrite(db *pgxpool.Pool) error {
	// Pick a product.
	productID := products[rand.Intn(len(products))]
	stock := rand.Intn(1000)

	// Update stock in database.
	const stmt = `UPDATE stock SET quantity = $1 WHERE product_id = $2`
	if _, err := db.Exec(context.Background(), stmt, stock, productID); err != nil {
		return fmt.Errorf("updating database stock: %w", err)
	}

	return nil
}

func readFromDB(db *pgxpool.Pool, productID string) (int, error) {
	const stmt = `SELECT quantity FROM stock AS OF SYSTEM TIME follower_read_timestamp()
								WHERE product_id = $1`

	row := db.QueryRow(context.Background(), stmt, productID)

	var quantity int
	if err := row.Scan(&quantity); err != nil {
		return 0, fmt.Errorf("getting stock from database: %w", err)
	}

	return quantity, nil
}

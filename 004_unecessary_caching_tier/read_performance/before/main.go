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
	"github.com/redis/go-redis/v9"
)

var (
	products = mustLoadIDs("004_unecessary_caching_tier/read_performance/ids.json")
)

func main() {
	writeInterval := flag.Duration("w", time.Millisecond*100, "interval between writes")
	flag.Parse()

	cache := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})
	defer cache.Close()

	if err := cache.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("error pinging redis: %v", err)
	}

	db := connect.MustDatabase("postgres://root@localhost:26257/defaultdb?sslmode=disable")

	router := fiber.New()
	router.Get("/products/:id/stock", handleGetProduct(db, cache))
	go func() {
		log.Fatal(router.Listen(":3000"))
	}()

	simulateWrites(db, cache, *writeInterval)
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

func handleGetProduct(db *pgxpool.Pool, cache *redis.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		productID := c.Params("id")
		if productID == "" {
			return fiber.NewError(fiber.StatusUnprocessableEntity, "missing product id")
		}

		// Read stock from cache.
		cmd := cache.Get(context.Background(), productID)
		if err := cmd.Err(); err != nil {
			if err == redis.Nil {
				// No stock in cache, read from DB.
				dbQuantity, err := readFromDB(db, productID)
				if err != nil {
					return fmt.Errorf("reading from db: %w", err)
				}

				// Set stock in cache.
				if err = cache.Set(context.Background(), productID, dbQuantity, 0).Err(); err != nil {
					return fmt.Errorf("setting cache stock: %w", err)
				}

				return c.JSON(fiber.Map{
					"stock": dbQuantity,
				})
			}

			return fmt.Errorf("error getting stock from cache: %w", err)
		}

		cacheQuantity, err := cmd.Int()
		if err != nil {
			return fmt.Errorf("converting cache quantity to integer: %w", err)
		}

		return c.JSON(fiber.Map{
			"stock": cacheQuantity,
		})
	}
}

func simulateWrites(db *pgxpool.Pool, cache *redis.Client, rate time.Duration) error {
	writesMade := 0
	for range time.NewTicker(rate).C {
		if err := simulateWrite(db, cache); err != nil {
			log.Printf("error simulating write: %v", err)
		}

		writesMade++
		fmt.Printf("Writes made: %d\r", writesMade)
	}

	return fmt.Errorf("finished simulateWrites unexectedly")
}

func simulateWrite(db *pgxpool.Pool, cache *redis.Client) error {
	// Pick a product.
	productID := products[rand.Intn(len(products))]
	stock := rand.Intn(1000)

	// Update stock in database.
	const stmt = `UPDATE stock SET quantity = $1 WHERE product_id = $2`
	if _, err := db.Exec(context.Background(), stmt, stock, productID); err != nil {
		return fmt.Errorf("updating database stock: %w", err)
	}

	// Invalidate cache.
	if err := cache.Del(context.Background(), productID).Err(); err != nil {
		return fmt.Errorf("invalidating cache: %w", err)
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

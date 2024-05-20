package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

var (
	globalMU sync.RWMutex

	products = []string{
		"93410c29-1609-484d-8662-ae2d0aa93cc4",
		"47b0472d-708c-4377-aab4-acf8752f0ecb",
		"a1a879d8-58c0-4357-a570-a57c3b1fe059",
		"5ded80d3-fb55-4a2f-b339-43fc9c89894a",
		"b6afe0c5-9cab-4971-8c61-127fe5b4acd1",
		"7098227b-4883-4992-bc32-e12335efbc8c",
	}

	reads          uint64
	writes         uint64
	cacheCorrect   uint64
	cacheMiss      uint64
	cacheHit       uint64
	cacheIncorrect uint64
)

func main() {
	readInterval := flag.Duration("r", time.Millisecond*100, "interval between reads")
	writeInterval := flag.Duration("w", time.Millisecond*1000, "interval between writes")
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

	db := connect.MustDatabase("postgres://postgres:password@localhost:5432/postgres?sslmode=disable")

	go simulateReads(db, cache, *readInterval)
	go simulateWrites(db, cache, *writeInterval)
	printLoop(db, cache)
}

type product struct {
	id    string
	stock int
}

func simulateReads(db *pgxpool.Pool, cache *redis.Client, rate time.Duration) error {
	for range time.NewTicker(rate).C {
		globalMU.RLock()
		if err := simulateRead(db, cache); err != nil {
			log.Printf("error simulating read: %v", err)
		}
		globalMU.RUnlock()

		atomic.AddUint64(&reads, 1)
	}

	return fmt.Errorf("finished simulateReads unexectedly")
}

func simulateRead(db *pgxpool.Pool, cache *redis.Client) error {
	// Pick a product.
	productID := products[rand.Intn(len(products))]

	// Read stock from cache.
	cmd := cache.Get(context.Background(), productID)
	if err := cmd.Err(); err != nil {
		if err == redis.Nil {
			atomic.AddUint64(&cacheMiss, 1)

			// No stock in cache, read from DB.
			dbQuantity, err := readFromDB(db, productID)
			if err != nil {
				return fmt.Errorf("reading from db: %w", err)
			}

			// Set stock in cache but "fail" 1% of the time.
			if rand.Intn(100) < 99 {
				if err = cache.Set(context.Background(), productID, dbQuantity, 0).Err(); err != nil {
					return fmt.Errorf("setting cache stock: %w", err)
				}
			}
		}
		return nil
	}

	atomic.AddUint64(&cacheHit, 1)
	return nil
}

func simulateWrites(db *pgxpool.Pool, cache *redis.Client, rate time.Duration) error {
	stock := 0
	for range time.NewTicker(rate).C {
		stock++

		globalMU.Lock()
		if err := simulateWrite(db, cache, stock); err != nil {
			log.Printf("error simulating write: %v", err)
		}
		globalMU.Unlock()

		atomic.AddUint64(&writes, 1)
	}

	return fmt.Errorf("finished simulateWrites unexectedly")
}

func simulateWrite(db *pgxpool.Pool, cache *redis.Client, stock int) error {
	// Pick a product.
	productID := products[rand.Intn(len(products))]

	// Update stock in database but "fail" 1% of the time.
	if rand.Intn(100) < 99 {
		const stmt = `UPDATE stock SET quantity = $1 WHERE product_id = $2`
		if _, err := db.Exec(context.Background(), stmt, stock, productID); err != nil {
			return fmt.Errorf("updating database stock: %w", err)
		}
	}

	// Invalidate cache but "fail" 1% of the time.
	if rand.Intn(100) < 99 {
		if err := cache.Del(context.Background(), productID).Err(); err != nil {
			return fmt.Errorf("invalidating cache: %w", err)
		}
	}

	return nil
}

func printLoop(db *pgxpool.Pool, cache *redis.Client) {
	for range time.NewTicker(time.Millisecond * 100).C {
		if err := fetchAndCheck(db, cache); err != nil {
			log.Printf("error during check: %v", err)
			continue
		}

		fmt.Println("\033[H\033[2J")
		fmt.Printf("reads:           %d (hits: %d / misses: %d)\n",
			atomic.LoadUint64(&reads),
			atomic.LoadUint64(&cacheHit),
			atomic.LoadUint64(&cacheMiss),
		)
		fmt.Printf("writes:          %d\n", atomic.LoadUint64(&writes))
		fmt.Println()
		fmt.Println("Snapshot sample")
		fmt.Println("---------------")
		fmt.Println()
		fmt.Printf("cache correct:   %d\n", atomic.LoadUint64(&cacheCorrect))
		fmt.Printf("cache incorrect: %d\n", atomic.LoadUint64(&cacheIncorrect))
	}
}

func fetchAndCheck(db *pgxpool.Pool, cache *redis.Client) error {
	globalMU.Lock()
	defer globalMU.Unlock()

	// Fetch database values.
	dbProducts := map[string]int{}

	rows, err := db.Query(context.Background(), "SELECT product_id, quantity FROM stock")
	if err != nil {
		return fmt.Errorf("error getting db values: %w", err)
	}

	for rows.Next() {
		var p product
		if err = rows.Scan(&p.id, &p.stock); err != nil {
			return fmt.Errorf("error scanning db value: %w", err)
		}
		dbProducts[p.id] = p.stock
	}

	// Fetch cache values.
	cacheProducts := map[string]int{}
	cmd := cache.MGet(context.Background(), products...)
	result, err := cmd.Result()
	if err != nil {
		return fmt.Errorf("error getting cache values: %w", err)
	}

	for i, r := range result {
		stockString, ok := r.(string)
		if !ok {
			continue
		}

		stock, err := strconv.Atoi(stockString)
		if err != nil {
			continue
		}

		cacheProducts[products[i]] = stock
	}

	// Compare.
	success := true
	for dbk, dbv := range dbProducts {
		cv, ok := cacheProducts[dbk]
		if ok && cv != dbv {
			atomic.AddUint64(&cacheIncorrect, 1)
			success = false
		}
	}

	if success {
		atomic.AddUint64(&cacheCorrect, 1)
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

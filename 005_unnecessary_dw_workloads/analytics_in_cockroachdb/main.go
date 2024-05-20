package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	browseProductsLatencyWindow = rollingAverage(1000)
	browseProductsLatencyMu     sync.RWMutex
	browseProductsLatency       time.Duration

	createOrderLatencyWindow = rollingAverage(1000)
	createOrderLatencyMu     sync.RWMutex
	createOrderLatency       time.Duration
)

func main() {
	db := connect.MustDatatabaseWithConfig(
		"postgres://root@localhost:26257/defaultdb?sslmode=disable",
		func(cfg *pgxpool.Config) {
			cfg.MaxConns = 50
		})

	router := fiber.New()
	router.Post("/customers", createCustomer(db))
	router.Get("/products", getProducts(db))
	router.Post("/orders", createOrder(db))

	go printLoop()

	log.Fatal(router.Listen(":3000"))
}

func createCustomer(db *pgxpool.Pool) fiber.Handler {
	type request struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	}

	return func(c *fiber.Ctx) error {
		const stmt = `INSERT INTO customers (id, email)
									VALUES ($1, $2)`

		var r request
		if err := c.BodyParser(&r); err != nil {
			log.Printf("error parsing request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "error parsing request")
		}

		if _, err := db.Exec(c.Context(), stmt, r.ID, r.Email); err != nil {
			return fmt.Errorf("inserting customer: %w", err)
		}

		return nil
	}
}

type product struct {
	ID    string `json:"id"`
	Price string `json:"price"`
}

func getProducts(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		const stmt = `SELECT id, price
									FROM products
									ORDER BY random()
									LIMIT 10`

		rows, err := db.Query(c.Context(), stmt)
		if err != nil {
			log.Printf("error getting products: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error getting products")
		}

		var products []product
		var p product
		for rows.Next() {
			if err = rows.Scan(&p.ID, &p.Price); err != nil {
				log.Printf("error scanning product: %v", err)
				return fiber.NewError(fiber.StatusInternalServerError, "error scanning product")
			}
			products = append(products, p)
		}

		browseProductsLatencyMu.Lock()
		browseProductsLatency = browseProductsLatencyWindow(time.Since(start))
		browseProductsLatencyMu.Unlock()
		return c.JSON(products)
	}
}

func createOrder(db *pgxpool.Pool) fiber.Handler {
	type request struct {
		ID         string `json:"id"`
		CustomerID string `json:"customer_id"`
		Items      []struct {
			ID       string `json:"id"`
			Quantity int    `json:"quantity"`
		} `json:"items"`
		Total float64 `json:"total"`
	}

	return func(c *fiber.Ctx) error {
		start := time.Now()

		const orderStmt = `INSERT INTO orders (id, customer_id, total)
											 VALUES ($1, $2, $3)`

		const orderItemStmt = `INSERT INTO order_items (order_id, product_id, quantity)
													 VALUES ($1, $2, $3)`

		var o request
		if err := c.BodyParser(&o); err != nil {
			log.Printf("error parsing request: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "error parsing request")
		}

		err := crdbpgx.ExecuteTx(c.Context(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
			// Insert order.
			if _, err := db.Exec(c.Context(), orderStmt, o.ID, o.CustomerID, o.Total); err != nil {
				return fmt.Errorf("inserting order: %w", err)
			}

			// Insert order items.
			for _, item := range o.Items {
				if _, err := db.Exec(c.Context(), orderItemStmt, o.ID, item.ID, item.Quantity); err != nil {
					return fmt.Errorf("inserting order: %w", err)
				}
			}

			return nil
		})

		if err != nil {
			log.Printf("error creating order: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error creating order")
		}

		createOrderLatencyMu.Lock()
		createOrderLatency = createOrderLatencyWindow(time.Since(start))
		createOrderLatencyMu.Unlock()
		return nil
	}
}

func rollingAverage(period int) func(time.Duration) time.Duration {
	var i int
	var sum time.Duration
	var storage = make([]time.Duration, 0, period)

	return func(input time.Duration) (avrg time.Duration) {
		if len(storage) < period {
			sum += input
			storage = append(storage, input)
		}

		sum += input - storage[i]
		storage[i], i = input, (i+1)%period
		avrg = sum / time.Duration(len(storage))

		return
	}
}

func printLoop() {
	for range time.NewTicker(time.Second).C {
		fmt.Println("\033[H\033[2J")

		browseProductsLatencyMu.RLock()
		createOrderLatencyMu.RLock()

		fmt.Printf("avg browse latency: %dms\n", browseProductsLatency.Milliseconds())
		fmt.Printf("avg order latency:  %dms\n", createOrderLatency.Milliseconds())

		browseProductsLatencyMu.RUnlock()
		createOrderLatencyMu.RUnlock()
	}
}

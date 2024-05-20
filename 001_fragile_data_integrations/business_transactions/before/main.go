package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	router := fiber.New()
	router.Post("/orders", handleCreateOrder(db))

	log.Fatal(router.Listen(":3000"))
}

func handleCreateOrder(db *pgxpool.Pool) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		log.Println("[order] create")

		var o order
		if err := ctx.BodyParser(&o); err != nil {
			log.Printf("invalid order: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid order")
		}

		if err := json.Unmarshal(o.Products, &o.productsParsed); err != nil {
			log.Printf("parsing products: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "parsing products")
		}

		if err := json.Unmarshal(o.Failures, &o.failuresParsed); err != nil {
			log.Printf("parsing failures: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "parsing failures")
		}

		// Build creation/cancellation workflows.
		functions := []createCancel{
			{
				create: createOrder,
			},
			{
				create:  createPayment,
				cancels: []dbFunc{cancelOrder},
			},
			{
				create:  createReservation,
				cancels: []dbFunc{cancelPayment, cancelOrder},
			},
			{
				create:  createShipment,
				cancels: []dbFunc{cancelReservation, cancelPayment, cancelOrder},
			},
		}

		for _, f := range functions {
			// Attempt to create.
			if err := f.create(db, o); err != nil {
				log.Printf("creating order: %v", err)

				// Attempt to unwind.
				for _, cf := range f.cancels {
					if err := cf(db, o); err != nil {
						log.Printf("cancelling order: %v", err)
						return fiber.NewError(fiber.StatusUnprocessableEntity, "cancelling order")
					}
				}
				return fiber.NewError(fiber.StatusUnprocessableEntity, "creating order")
			}
		}

		return nil
	}
}

// **********
// Structs **
// **********

type order struct {
	OrderID  string          `json:"order_id"`
	Payment  float64         `json:"payment"`
	Products json.RawMessage `json:"products"`
	Failures json.RawMessage `json:"failures"`

	productsParsed []struct {
		ID       string `json:"id"`
		Quantity int    `json:"quantity"`
	}
	failuresParsed failures
}

type failures struct {
	Orders       bool `json:"orders"`
	Payments     bool `json:"payments"`
	Reservations bool `json:"reservations"`
	Shipments    bool `json:"shipments"`
}

// *********************
// Database functions **
// *********************

type createCancel struct {
	create  dbFunc
	cancels []dbFunc
}

type dbFunc func(*pgxpool.Pool, order) error

func createOrder(db *pgxpool.Pool, o order) error {
	log.Println("creating order")

	if o.failuresParsed.Orders {
		return fmt.Errorf("mocked error creating order")
	}

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO orders (id) VALUES ($1)`

		if _, err := tx.Exec(context.Background(), stmt, o.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		return nil
	})
}

func cancelOrder(db *pgxpool.Pool, o order) error {
	log.Println("cancelling order")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `UPDATE orders SET status = 'cancelled' WHERE id = $1`

		if _, err := tx.Exec(context.Background(), stmt, o.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		return nil
	})
}

func createPayment(db *pgxpool.Pool, o order) error {
	log.Println("creating payment")

	if o.failuresParsed.Payments {
		return fmt.Errorf("mocked error creating payment")
	}

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO payments (order_id, amount) VALUES ($1, $2)`

		if _, err := tx.Exec(context.Background(), stmt, o.OrderID, o.Payment); err != nil {
			return fmt.Errorf("inserting payment: %w", err)
		}

		return nil
	})
}

func cancelPayment(db *pgxpool.Pool, o order) error {
	log.Println("cancelling payment")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `UPDATE payments SET status = 'cancelled' WHERE order_id = $1`

		if _, err := tx.Exec(context.Background(), stmt, o.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		return nil
	})
}

func createReservation(db *pgxpool.Pool, o order) error {
	log.Println("creating reservation")

	if o.failuresParsed.Reservations {
		return fmt.Errorf("mocked error creating reservations")
	}

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO reservations (order_id, product_id, quantity) VALUES ($1, $2, $3)`

		for _, r := range o.productsParsed {
			if _, err := tx.Exec(context.Background(), stmt, o.OrderID, r.ID, r.Quantity); err != nil {
				return fmt.Errorf("inserting reservation: %w", err)
			}
		}

		return nil
	})
}

func cancelReservation(db *pgxpool.Pool, o order) error {
	log.Println("cancelling reservation")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `DELETE FROM reservations WHERE order_id = $1`

		if _, err := tx.Exec(context.Background(), stmt, o.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		return nil
	})
}

func createShipment(db *pgxpool.Pool, o order) error {
	log.Println("creating shipment")

	if o.failuresParsed.Shipments {
		return fmt.Errorf("mocked error creating shipment")
	}

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO shipments (order_id) VALUES ($1)`

		if _, err := tx.Exec(context.Background(), stmt, o.OrderID); err != nil {
			return fmt.Errorf("inserting payment: %w", err)
		}

		return nil
	})
}

func cancelShipment(db *pgxpool.Pool, o order) error {
	log.Println("cancelling shipment")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `UPDATE shipments SET status = 'cancelled' WHERE order_id = $1`

		if _, err := tx.Exec(context.Background(), stmt, "cancelled", o.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		return nil
	})
}

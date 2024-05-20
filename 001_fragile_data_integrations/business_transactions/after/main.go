package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	crdbpgx "github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgxv5"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

func main() {
	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	router := fiber.New()
	router.Post("/sagas", createSaga(db))

	go handleSagaMessages(db, newReader("localhost:9092", "sagas"))

	log.Fatal(router.Listen(":3000"))
}

func handleSagaMessages(db *pgxpool.Pool, reader *kafka.Reader) {
	defer reader.Close()

	for {
		var s saga
		commit, err := readAndParse(reader, &s)
		if err != nil {
			log.Printf("error fetching event: %v", err)
			continue
		}

		// Nothing to do.
		if s.Status == sagaStatusCancelled || s.Status == sagaStatusFinished {
			continue
		}

		switch s.Step {
		case sagaStepOrder:
			if err := processOrder(db, s); err != nil {
				log.Printf("error processing order: %v", err)
				continue
			}
		case sagaStepPayment:
			if err := processPayment(db, s); err != nil {
				log.Printf("error processing payment: %v", err)
				continue
			}
		case sagaStepReservation:
			if err := processReservation(db, s); err != nil {
				log.Printf("error processing reservations: %v", err)
				continue
			}
		case sagaStepShipment:
			if err := processShipment(db, s); err != nil {
				log.Printf("error processing shipment: %v", err)
				continue
			}
		default:
			log.Printf("invalid saga step: %v", err)
			continue
		}

		if err := commit(); err != nil {
			log.Printf("error committing event: %v", err)
			continue
		}
	}
}

func processOrder(db *pgxpool.Pool, s saga) error {
	if s.failuresParsed.Orders {
		return cancelSaga(db, s)
	}

	switch s.Status {
	case sagaStatusInProgress:
		return createOrder(db, s)
	case sagaStatusCancelling:
		return cancelOrder(db, s)
	default:
		return fmt.Errorf("invalid status: %q", s.Status)
	}
}

func processPayment(db *pgxpool.Pool, s saga) error {
	if s.failuresParsed.Payments {
		return cancelOrder(db, s)
	}

	switch s.Status {
	case sagaStatusInProgress:
		return createPayment(db, s)
	case sagaStatusCancelling:
		return cancelPayment(db, s)
	default:
		return fmt.Errorf("invalid status: %q", s.Status)
	}
}

func processReservation(db *pgxpool.Pool, s saga) error {
	if s.failuresParsed.Reservations {
		return cancelPayment(db, s)
	}

	switch s.Status {
	case sagaStatusInProgress:
		return createReservation(db, s)
	case sagaStatusCancelling:
		return cancelReservation(db, s)
	default:
		return fmt.Errorf("invalid status: %q", s.Status)
	}
}

func processShipment(db *pgxpool.Pool, s saga) error {
	if s.failuresParsed.Shipments {
		return cancelReservation(db, s)
	}

	switch s.Status {
	case sagaStatusInProgress:
		return createShipment(db, s)
	case sagaStatusCancelling:
		return cancelShipment(db, s)
	default:
		return fmt.Errorf("invalid status: %q", s.Status)
	}
}

// **********
// Structs **
// **********

type sagaStep string

const (
	sagaStepOrder       sagaStep = "order"
	sagaStepPayment     sagaStep = "payment"
	sagaStepReservation sagaStep = "reservation"
	sagaStepShipment    sagaStep = "shipment"
)

type sagaStatus string

const (
	sagaStatusInProgress sagaStatus = "in_progress"
	sagaStatusFinished   sagaStatus = "finished"
	sagaStatusCancelling sagaStatus = "cancelling"
	sagaStatusCancelled  sagaStatus = "cancelled"
)

type saga struct {
	OrderID  string          `json:"order_id"`
	Payment  float64         `json:"payment"`
	Products json.RawMessage `json:"products"`
	Step     sagaStep        `json:"step"`
	Status   sagaStatus      `json:"status"`
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

func createSaga(db *pgxpool.Pool) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		log.Println("[saga] create")

		var s saga
		if err := ctx.BodyParser(&s); err != nil {
			log.Printf("invalid saga: %v", err)
			return fiber.NewError(fiber.StatusUnprocessableEntity, "invalid saga")
		}

		const stmt = `INSERT INTO sagas (order_id, payment, products, failures) VALUES ($1, $2, $3, $4)`
		if _, err := db.Exec(ctx.Context(), stmt, s.OrderID, s.Payment, s.Products, s.Failures); err != nil {
			log.Printf("inserting saga: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "inserting saga")
		}

		return nil
	}
}

func cancelSaga(db *pgxpool.Pool, s saga) error {
	log.Println("cancelling order")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `UPDATE sagas SET status = 'cancelled' WHERE order_id = $1`

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepOrder, sagaStatusCancelled); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func createOrder(db *pgxpool.Pool, s saga) error {
	log.Println("creating order")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO orders (id) VALUES ($1)`

		// Insert order (if it fails, don't update saga just yet, we might be able to try
		// again when CockroachDB republishes the unacked message).
		if _, err := tx.Exec(context.Background(), stmt, s.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepPayment, sagaStatusInProgress); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func cancelOrder(db *pgxpool.Pool, s saga) error {
	log.Println("cancelling order")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `UPDATE orders SET status = 'cancelled' WHERE id = $1`

		// Insert order (if it fails, don't update saga just yet, we might be able to try
		// again when CockroachDB republishes the unacked message).
		if _, err := tx.Exec(context.Background(), stmt, s.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepOrder, sagaStatusCancelled); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func createPayment(db *pgxpool.Pool, s saga) error {
	log.Println("creating payment")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO payments (order_id, amount) VALUES ($1, $2)`

		// Insert payment (if it fails, don't update saga just yet, we might be able to try
		// again when CockroachDB republishes the unacked message).
		if _, err := tx.Exec(context.Background(), stmt, s.OrderID, s.Payment); err != nil {
			return fmt.Errorf("inserting payment: %w", err)
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepReservation, sagaStatusInProgress); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func cancelPayment(db *pgxpool.Pool, s saga) error {
	log.Println("cancelling payment")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `UPDATE payments SET status = 'cancelled' WHERE order_id = $1`

		// Update payment (if it fails, don't update saga just yet, we might be able to try
		// again when CockroachDB republishes the unacked message).
		if _, err := tx.Exec(context.Background(), stmt, s.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepOrder, sagaStatusCancelling); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func createReservation(db *pgxpool.Pool, s saga) error {
	log.Println("creating reservation")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO reservations (order_id, product_id, quantity) VALUES ($1, $2, $3)`

		// Insert reservation (if it fails, don't update saga just yet, we might be able to try
		// again when CockroachDB republishes the unacked message).
		for _, r := range s.productsParsed {
			if _, err := tx.Exec(context.Background(), stmt, s.OrderID, r.ID, r.Quantity); err != nil {
				return fmt.Errorf("inserting reservation: %w", err)
			}
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepShipment, sagaStatusInProgress); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func cancelReservation(db *pgxpool.Pool, s saga) error {
	log.Println("cancelling reservation")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `DELETE FROM reservations WHERE order_id = $1`

		// Delete reserved quantities (if it fails, don't update saga just yet, we might
		// be able to try again when CockroachDB republishes the unacked message).
		if _, err := tx.Exec(context.Background(), stmt, s.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepPayment, sagaStatusCancelling); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func createShipment(db *pgxpool.Pool, s saga) error {
	log.Println("creating shipment")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `INSERT INTO shipments (order_id) VALUES ($1)`

		// Insert shipment (if it fails, don't update saga just yet, we might be able to try
		// again when CockroachDB republishes the unacked message).
		if _, err := tx.Exec(context.Background(), stmt, s.OrderID); err != nil {
			return fmt.Errorf("inserting payment: %w", err)
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepShipment, sagaStatusFinished); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

func cancelShipment(db *pgxpool.Pool, s saga) error {
	log.Println("cancelling shipment")

	return crdbpgx.ExecuteTx(context.Background(), db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		const stmt = `UPDATE shipments SET status = 'cancelled' WHERE order_id = $1`

		// Update payment (if it fails, don't update saga just yet, we might be able to try
		// again when CockroachDB republishes the unacked message).
		if _, err := tx.Exec(context.Background(), stmt, "cancelled", s.OrderID); err != nil {
			return fmt.Errorf("inserting order: %w", err)
		}

		// Update saga.
		if err := updateSaga(tx, s.OrderID, sagaStepReservation, sagaStatusCancelling); err != nil {
			return fmt.Errorf("updating saga: %w", err)
		}

		return nil
	})
}

// **********
// Helpers **
// **********

func newReader(url, topic string) *kafka.Reader {
	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{url},
		GroupID:     uuid.NewString(),
		Topic:       topic,
		StartOffset: kafka.LastOffset,
	})

	return kafkaReader
}

type commitFunc func() error

func readAndParse(reader *kafka.Reader, s *saga) (commitFunc, error) {
	msg, err := reader.FetchMessage(context.Background())
	if err != nil {
		return nil, fmt.Errorf("reading message: %w", err)
	}

	if err = json.Unmarshal(msg.Value, &s); err != nil {
		return nil, fmt.Errorf("parsing message: %w", err)
	}

	if err = json.Unmarshal(s.Products, &s.productsParsed); err != nil {
		return nil, fmt.Errorf("parsing products: %w", err)
	}

	if err = json.Unmarshal(s.Failures, &s.failuresParsed); err != nil {
		return nil, fmt.Errorf("parsing failures: %w", err)
	}

	commit := func() error {
		return reader.CommitMessages(context.Background(), msg)
	}

	return commit, nil
}

func updateSaga(tx pgx.Tx, orderID string, step sagaStep, status sagaStatus) error {
	const stmt = `UPDATE sagas SET
									step = $1,
									status = $2
								WHERE order_id = $3`

	if _, err := tx.Exec(context.Background(), stmt, step, status, orderID); err != nil {
		return fmt.Errorf("updating saga: %w", err)
	}

	return nil
}

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

func main() {
	log.SetFlags(0)

	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	transformedReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{"localhost:9092"},
		GroupID:     uuid.NewString(),
		Topic:       "transformed_2",
		StartOffset: kafka.LastOffset,
	})

	go simulateProducer(db)
	simulateConsumer(transformedReader)
}

func simulateProducer(db *pgxpool.Pool) error {
	const stmt = `INSERT INTO order_line_item (order_id, product_id, customer_id, quantity, price, ts) VALUES
								(gen_random_uuid(), gen_random_uuid(), gen_random_uuid(), $1, $2, $3)`

	for {
		time.Sleep(time.Millisecond * time.Duration(rand.Intn(3000)))

		if _, err := db.Exec(context.Background(), stmt, rand.Intn(10), rand.Float64()*100, time.Now()); err != nil {
			return fmt.Errorf("inserting event: %w", err)
		}
	}
}

type after struct {
	OrderID   string `json:"-"`
	Quantity  int    `json:"quantity"`
	Price     int64  `json:"price"` // Integer
	Timestamp int64  `json:"ts"`    // Epoch
}

func simulateConsumer(reader *kafka.Reader) error {
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("error reading message: %v", err)
			continue
		}

		// Parse message to determine total flight time.
		var a after
		if err = json.Unmarshal(m.Value, &a); err != nil {
			log.Printf("error parsing message: %v", err)
			continue
		}

		ts := time.Unix(a.Timestamp, 0)
		log.Printf("\n[transformed %s] %s", time.Since(ts), string(m.Value))
	}
}

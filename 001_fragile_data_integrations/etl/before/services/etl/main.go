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

	rawReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{"localhost:9092"},
		GroupID:     uuid.NewString(),
		Topic:       "raw",
		StartOffset: kafka.LastOffset,
	})

	transformedWriter := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:9092"),
		Topic:                  "transformed",
		AllowAutoTopicCreation: true,
	}
	defer transformedWriter.Close()

	go simulateProducer(db)
	simulateETL(rawReader, transformedWriter)
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

type before struct {
	After struct {
		OrderID    string    `json:"order_id"`
		ProductID  string    `json:"product_id"`
		CustomerID string    `json:"customer_id"`
		Quantity   int       `json:"quantity"`
		Price      float64   `json:"price"`
		Timestamp  time.Time `json:"ts"`
	} `json:"after"`
}

type after struct {
	OrderID   string `json:"-"`
	Quantity  int    `json:"quantity"`
	Price     int64  `json:"price"` // Integer
	Timestamp int64  `json:"ts"`    // Epoch
}

func simulateETL(reader *kafka.Reader, writer *kafka.Writer) error {
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("error reading message: %v", err)
			continue
		}

		a, err := transform(m)
		if err != nil {
			log.Printf("error transforming message: %v", err)
			continue
		}

		abytes, err := json.Marshal(a)
		if err != nil {
			log.Printf("error marshalling transformed message: %v", err)
			continue
		}

		out := kafka.Message{
			Key:   []byte(a.OrderID),
			Value: abytes,
		}
		if err = writer.WriteMessages(context.Background(), out); err != nil {
			log.Printf("writing transformed message: %v", err)
			continue
		}
	}
}

func transform(m kafka.Message) (after, error) {
	var b before
	if err := json.Unmarshal(m.Value, &b); err != nil {
		return after{}, fmt.Errorf("parsing before")
	}

	return after{
		Quantity:  b.After.Quantity,
		Price:     int64(b.After.Price * 100),
		Timestamp: b.After.Timestamp.Unix(),
	}, nil
}

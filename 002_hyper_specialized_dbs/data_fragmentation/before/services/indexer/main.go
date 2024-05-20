package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

func main() {
	kafkaURL, ok := os.LookupEnv("KAFKA_URL")
	if !ok {
		log.Fatal("missing KAFKA_URL env var")
	}

	indexURL, ok := os.LookupEnv("INDEX_URL")
	if !ok {
		log.Fatalf("missing INDEX_URL env var")
	}

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{kafkaURL},
		GroupID:     uuid.NewString(),
		Topic:       "products.store.product",
		StartOffset: kafka.LastOffset,
	})

	db := connect.MustDatabase(indexURL)
	defer db.Close()

	consume(kafkaReader, db)
}

type cdcEvent struct {
	Payload struct {
		Op    string `json:"op"`
		After struct {
			ID struct {
				Value      string `json:"value"`
				DeletionTs any    `json:"deletion_ts"`
				Set        bool   `json:"set"`
			} `json:"id"`
			Ts struct {
				Value      string `json:"value"`
				DeletionTs any    `json:"deletion_ts"`
				Set        bool   `json:"set"`
			} `json:"ts"`
			Description struct {
				Value      string `json:"value"`
				DeletionTs any    `json:"deletion_ts"`
				Set        bool   `json:"set"`
			} `json:"description"`
			Name struct {
				Value      string `json:"value"`
				DeletionTs any    `json:"deletion_ts"`
				Set        bool   `json:"set"`
			} `json:"name"`
		} `json:"after"`
	} `json:"payload"`
}

func consume(reader *kafka.Reader, db *pgxpool.Pool) {
	avgDelay := rollingAverage(50)

	for {
		msg, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("error reading event: %v", err)
			continue
		}

		var event cdcEvent
		if err = json.Unmarshal(msg.Value, &event); err != nil {
			log.Printf("error parsing event: %v", err)
			continue
		}

		ts, err := updateIndex(db, event)
		if err != nil {
			log.Printf("error updating index: %v", err)
			continue
		}

		fmt.Printf("average delay: %s\r", avgDelay(time.Since(ts)))
	}
}

func updateIndex(db *pgxpool.Pool, e cdcEvent) (time.Time, error) {
	const stmt = `INSERT INTO product (id, name, description, ts) VALUES ($1, $2, $3, $4)
								ON CONFLICT (id)
								DO UPDATE SET
									name = EXCLUDED.name,
									description = EXCLUDED.description`

	a := e.Payload.After

	uuidTime, err := uuid.Parse(a.Ts.Value)
	if err != nil {
		return time.Time{}, fmt.Errorf("error parsing timeuuid: %w", err)
	}

	sec, nsec := uuidTime.Time().UnixTime()
	ts := time.Unix(sec, nsec)

	if _, err := db.Exec(context.Background(), stmt, a.ID.Value, a.Name.Value, a.Description.Value, ts); err != nil {
		return time.Time{}, fmt.Errorf("upserting product: %w", err)
	}

	return ts, nil
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

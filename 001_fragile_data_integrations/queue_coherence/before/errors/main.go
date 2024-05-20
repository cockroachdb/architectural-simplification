package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

var (
	productID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	exptectedCount uint64
	actualCount    uint64
)

func main() {
	writeInterval := flag.Duration("w", time.Millisecond*1000, "interval between writes")
	flag.Parse()

	kafkaWriter := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:9092"),
		Topic:                  "stock",
		AllowAutoTopicCreation: true,
		BatchSize:              1,
	}
	defer kafkaWriter.Close()

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{"localhost:9092"},
		GroupID:     uuid.NewString(),
		Topic:       "stock",
		StartOffset: kafka.LastOffset,
	})

	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	go simulateQueueConsumer(kafkaReader)
	go simulateProducer(db, kafkaWriter, *writeInterval)
	printLoop()
}

func simulateQueueConsumer(reader *kafka.Reader) error {
	for {
		_, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("error reading message: %v", err)
		}

		atomic.AddUint64(&actualCount, 1)
	}
}

func simulateProducer(db *pgxpool.Pool, kafka *kafka.Writer, rate time.Duration) error {
	for range time.NewTicker(rate).C {
		if err := simulateWrite(db, kafka); err != nil {
			log.Printf("error simulating write: %v", err)
		}
	}

	return fmt.Errorf("finished simulateWrites unexectedly")
}

func simulateWrite(db *pgxpool.Pool, writer *kafka.Writer) error {
	// Update stock in database.
	const stmt = `UPDATE stock SET quantity = 1000 WHERE product_id = $1`
	if _, err := db.Exec(context.Background(), stmt, productID); err != nil {
		return fmt.Errorf("updating database stock: %w", err)
	}

	atomic.AddUint64(&exptectedCount, 1)

	if rand.Intn(100) == 99 {
		return nil
	}

	// Publish message.
	err := writer.WriteMessages(
		context.Background(),
		kafka.Message{
			Key:   []byte(productID),
			Value: []byte(strconv.Itoa(1000)),
		},
	)

	if err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	return nil
}

func printLoop() {
	for range time.NewTicker(time.Second).C {
		fmt.Println("\033[H\033[2J")

		fmt.Printf("db count:    %d\n", atomic.LoadUint64(&exptectedCount))
		fmt.Printf("queue count: %d\n", atomic.LoadUint64(&actualCount))
	}
}

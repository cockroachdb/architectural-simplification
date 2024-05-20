package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
)

var (
	productID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"

	dbQuantities    *quantities
	queueQuantities *quantities
)

func main() {
	readInterval := flag.Duration("r", time.Millisecond*100, "interval between reads")
	writeInterval := flag.Duration("w", time.Millisecond*1000, "interval between writes")
	flag.Parse()

	kafkaWriter := &kafka.Writer{
		Addr:                   kafka.TCP("localhost:9092"),
		Topic:                  "stock",
		AllowAutoTopicCreation: true,
	}
	defer kafkaWriter.Close()

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{"localhost:9092"},
		GroupID: "cg-a",
		Topic:   "stock",
	})

	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	dbQuantities = &quantities{
		name:     "db quantities",
		products: map[string]int{},
	}
	queueQuantities = &quantities{
		name:     "cache quantities",
		products: map[string]int{},
	}

	go simulatePollingConsumer(db, *readInterval)
	go simulateQueueConsumer(kafkaReader)
	go simulateProducer(db, kafkaWriter, *writeInterval)
	printLoop()
}

type quantities struct {
	name string

	productsMu sync.RWMutex
	products   map[string]int
}

type product struct {
	id    string
	stock int
}

func (q *quantities) set(product string, stock int) {
	q.productsMu.Lock()
	defer q.productsMu.Unlock()

	q.products[product] = stock
}

func simulatePollingConsumer(db *pgxpool.Pool, rate time.Duration) error {
	for range time.NewTicker(rate).C {
		if err := simulateRead(db); err != nil {
			log.Printf("error simulating read: %v", err)
		}
	}

	return fmt.Errorf("finished simulateReads unexectedly")
}

func simulateRead(db *pgxpool.Pool) error {
	dbQuantity, err := readFromDB(db, productID)
	if err != nil {
		return fmt.Errorf("reading from db: %w", err)
	}

	dbQuantities.set(productID, dbQuantity)
	return nil
}

func simulateQueueConsumer(reader *kafka.Reader) error {
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("error reading message: %v", err)
		}

		stock, err := strconv.Atoi(string(m.Value))
		if err != nil {
			log.Printf("stock value is not an integer: %v", err)
		}

		queueQuantities.set(string(m.Key), stock)
	}
}

func simulateProducer(db *pgxpool.Pool, kafka *kafka.Writer, rate time.Duration) error {
	i := 0
	for range time.NewTicker(rate).C {
		i++
		if err := simulateWrite(db, kafka, i); err != nil {
			log.Printf("error simulating write: %v", err)
		}
	}

	return fmt.Errorf("finished simulateWrites unexectedly")
}

func simulateWrite(db *pgxpool.Pool, writer *kafka.Writer, stock int) error {
	// Update stock in database.
	const stmt = `UPDATE stock SET quantity = $1 WHERE product_id = $2`
	if _, err := db.Exec(context.Background(), stmt, stock, productID); err != nil {
		return fmt.Errorf("updating database stock: %w", err)
	}

	// Publish message.
	err := writer.WriteMessages(
		context.Background(),
		kafka.Message{
			Key:   []byte(productID),
			Value: []byte(strconv.Itoa(stock)),
		},
	)

	if err != nil {
		return fmt.Errorf("writing message: %w", err)
	}

	return nil
}

func printLoop() {
	for range time.NewTicker(time.Second).C {
		dbQuantities.productsMu.RLock()
		queueQuantities.productsMu.RLock()

		lines := []string{}

		for dbk, dbv := range dbQuantities.products {
			cv := queueQuantities.products[dbk]

			if cv != dbv {
				lines = append(lines, fmt.Sprintf("%s (db: %d vs queue: %d)\n", dbk, dbv, cv))
			}
		}

		dbQuantities.productsMu.RUnlock()
		queueQuantities.productsMu.RUnlock()

		fmt.Println("\033[H\033[2J")
		if len(lines) > 0 {
			fmt.Println(strings.Join(lines, "\n"))
		} else {
			fmt.Println("queue and database match")
		}
	}
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

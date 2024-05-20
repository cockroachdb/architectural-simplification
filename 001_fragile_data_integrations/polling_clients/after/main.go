package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/segmentio/kafka-go"
	"golang.org/x/sync/errgroup"
)

var (
	requestsMade uint64
	rowsRead     uint64

	rowsWrittenMu       sync.RWMutex
	rowsWritten         = map[string]time.Time{}
	queueDelayAvg       float64
	queueDelayAvgWindow = rollingAverage(10)
)

func main() {
	writeInterval := flag.Duration("w", time.Millisecond*10, "interval between writes")
	concurrency := flag.Int("c", 1, "number of users to simulate")
	flag.Parse()

	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{"localhost:9092"},
		GroupID:     uuid.NewString(),
		Topic:       "events",
		StartOffset: kafka.LastOffset,
	})

	go simulateProducer(db, *writeInterval)
	go printLoop()

	var eg errgroup.Group
	for i := 0; i < *concurrency; i++ {
		eg.Go(func() error {
			return simulateQueueConsumer(kafkaReader)
		})
	}
	if err := eg.Wait(); err != nil {
		log.Fatalf("error in worker: %v", err)
	}
}

func simulateProducer(db *pgxpool.Pool, rate time.Duration) error {
	const stmt = `INSERT INTO events (id, value, ts) VALUES ($1, $2, $3)`

	for range time.NewTicker(rate).C {
		id := uuid.NewString()
		ts := time.Now()

		if _, err := db.Exec(context.Background(), stmt, id, rand.Float64()*100, ts); err != nil {
			return fmt.Errorf("inserting event: %w", err)
		}

		rowsWrittenMu.Lock()
		rowsWritten[id] = ts
		rowsWrittenMu.Unlock()
	}

	return fmt.Errorf("finished simulateProducer unexectedly")
}

func simulateQueueConsumer(reader *kafka.Reader) error {
	for {
		m, err := reader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("error reading message: %v", err)
			continue
		}
		applyPollLag(strings.Trim(string(m.Key), `["]`))

		atomic.AddUint64(&requestsMade, 1)
		atomic.AddUint64(&rowsRead, 1)
	}
}

func applyPollLag(id string) {
	// If rowsWritten contains the id, we've not tested it yet. If it doesn't,
	// we've already processed and can ignore.
	rowsWrittenMu.Lock()
	defer rowsWrittenMu.Unlock()

	rwts, ok := rowsWritten[id]
	if !ok {
		return
	}

	queueDelayAvg = queueDelayAvgWindow(time.Since(rwts).Seconds())

	delete(rowsWritten, id)
}

func rollingAverage(period int) func(float64) float64 {
	var i int
	var sum float64
	var storage = make([]float64, 0, period)

	return func(input float64) (avrg float64) {
		if len(storage) < period {
			sum += input
			storage = append(storage, input)
		}

		sum += input - storage[i]
		storage[i], i = input, (i+1)%period
		avrg = sum / float64(len(storage))

		return
	}
}

func printLoop() {
	for range time.NewTicker(time.Second).C {
		fmt.Println("\033[H\033[2J")
		fmt.Printf("requests made: %d\n", atomic.LoadUint64(&requestsMade))
		fmt.Printf("rows read:     %d\n", atomic.LoadUint64(&rowsRead))

		rowsWrittenMu.RLock()
		fmt.Printf("average delay: %0.2fs\n", queueDelayAvg)
		rowsWrittenMu.RUnlock()
	}
}

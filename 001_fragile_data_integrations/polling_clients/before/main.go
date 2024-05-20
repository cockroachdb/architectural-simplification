package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
)

var (
	requestsMade uint64
	rowsRead     uint64

	rowsWrittenMu sync.RWMutex
	rowsWritten   = map[string]time.Time{}
	averageLagMS  int64
)

func main() {
	writeInterval := flag.Duration("w", time.Millisecond*10, "interval between writes")
	readInterval := flag.Duration("r", time.Millisecond*100, "interval between reads")
	concurrency := flag.Int("c", 1, "number of users to simulate")
	flag.Parse()

	db := connect.MustDatabase("postgres://root@localhost:26257/?sslmode=disable")
	defer db.Close()

	go simulateProducer(db, *writeInterval)
	go printLoop()

	var eg errgroup.Group
	for i := 0; i < *concurrency; i++ {
		eg.Go(func() error {
			return simulatePollingConsumer(db, *readInterval)
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

func simulatePollingConsumer(db *pgxpool.Pool, rate time.Duration) error {
	for range time.NewTicker(rate).C {
		rowCount, err := readFromDB(db)
		if err != nil {
			log.Printf("error simulating read: %v", err)
		}

		atomic.AddUint64(&requestsMade, 1)
		atomic.AddUint64(&rowsRead, rowCount)
	}

	return fmt.Errorf("finished simulateReads unexectedly")
}

func readFromDB(db *pgxpool.Pool) (uint64, error) {
	const stmt = `SELECT id FROM events
								WHERE ts > now() - INTERVAL '10s'`

	rows, err := db.Query(context.Background(), stmt)
	if err != nil {
		return 0, fmt.Errorf("running query: %w", err)
	}

	count := 0
	var ids []string
	for rows.Next() {
		count++

		var id string
		if err = rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scanning row: %w", err)
		}
		ids = append(ids, id)
	}

	applyPollLag(ids)

	return uint64(count), nil
}

func applyPollLag(ids []string) {
	// If rowsWritten contains the id, we've not tested it yet. If it doesn't,
	// we've already processed and can ignore.
	rowsWrittenMu.Lock()
	defer rowsWrittenMu.Unlock()

	var totalDelays time.Duration
	var averagePool int
	for _, id := range ids {
		publishTS, ok := rowsWritten[id]
		if !ok {
			continue
		}

		totalDelays += time.Since(publishTS)
		averagePool++
		delete(rowsWritten, id)
	}

	if totalDelays == 0 {
		return
	}

	averageLagMS = totalDelays.Milliseconds() / int64(averagePool)
}

func printLoop() {
	for range time.NewTicker(time.Second).C {
		fmt.Println("\033[H\033[2J")
		fmt.Printf("requests made: %d\n", atomic.LoadUint64(&requestsMade))
		fmt.Printf("rows read:     %d\n", atomic.LoadUint64(&rowsRead))

		rowsWrittenMu.RLock()
		fmt.Printf("average delay: %dms\n", averageLagMS)
		rowsWrittenMu.RUnlock()
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/gocql/gocql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/api/option"
)

func main() {
	log.SetFlags(0)

	postgres := connect.MustDatabase("postgres://postgres:password@localhost:5432/postgres?sslmode=disable")
	defer postgres.Close()

	cassandra := mustConnectCassandra("localhost:9042")
	defer cassandra.Close()

	bigquery := mustConnectBigQuery("http://localhost:9050")
	defer bigquery.Close()

	if err := simulateWriter(postgres, cassandra, bigquery); err != nil {
		log.Fatalf("error running application: %v", err)
	}
}

type order struct {
	ID     string    `json:"id"`
	UserID string    `json:"user_id"`
	Total  float64   `json:"total"`
	TS     time.Time `json:"ts"`
}

func simulateWriter(pg *pgxpool.Pool, cs *gocql.Session, bq *bigquery.Client) error {
	i := 0
	for range time.NewTicker(time.Millisecond * 100).C {
		o := order{
			ID:     uuid.NewString(),
			UserID: uuid.NewString(),
			Total:  rand.Float64() * 100,
			TS:     time.Now(),
		}

		var wg sync.WaitGroup

		if err := writePostgres(pg, o, &wg); err != nil {
			log.Printf("writing to postgres: %v", err)
			continue
		}

		if err := writeCassandra(cs, o, &wg); err != nil {
			log.Printf("writing to cassandra: %v", err)
			continue
		}

		if err := writeBigQuery(bq, o, &wg); err != nil {
			log.Printf("writing to bigquery: %v", err)
			continue
		}

		wg.Wait()
		i++
		fmt.Printf("saved %d orders\r", i)
	}

	return fmt.Errorf("finished work unexpectedly")
}

func writePostgres(pg *pgxpool.Pool, o order, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()

	tt := timeEvent("writePostgres")
	defer tt()

	const stmt = `INSERT INTO orders (id, user_id, total, ts) VALUES ($1, $2, $3, $4)`

	// Insert row into Postgres but "fail" 1% of the time.
	if rand.Intn(100) == 99 {
		return fmt.Errorf("simulated error in postgres")
	}

	if _, err := pg.Exec(context.Background(), stmt, o.ID, o.UserID, o.Total, o.TS); err != nil {
		return fmt.Errorf("inserting row into postgres: %w", err)
	}

	return nil
}

func writeCassandra(cs *gocql.Session, o order, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()

	tt := timeEvent("writeCassandra")
	defer tt()

	const stmt = `INSERT INTO example.orders (id, user_id, total, ts) VALUES (?, ?, ?, ?)`

	// Insert row into Cassandra but "fail" 1% of the time.
	if rand.Intn(100) == 99 {
		return fmt.Errorf("simulated error in cassandra")
	}

	if err := cs.Query(stmt, o.ID, o.UserID, o.Total, o.TS).Exec(); err != nil {
		return fmt.Errorf("inserting row into cassandra: %w", err)
	}

	return nil
}

func (o order) Save() (map[string]bigquery.Value, string, error) {
	m := map[string]bigquery.Value{
		"id":      o.ID,
		"user_id": o.UserID,
		"total":   o.Total,
		"ts":      o.TS,
	}

	return m, o.ID, nil
}

func writeBigQuery(bq *bigquery.Client, o order, wg *sync.WaitGroup) error {
	wg.Add(1)
	defer wg.Done()

	tt := timeEvent("writeBigQuery")
	defer tt()

	inserter := bq.Dataset("example").Table("orders").Inserter()

	// Insert row into BigQuery but "fail" 10% of the time.
	if rand.Intn(100) >= 90 {
		return fmt.Errorf("simulated error in bigquery")
	}

	if err := inserter.Put(context.Background(), o); err != nil {
		return fmt.Errorf("inserting row into bigquery: %w", err)
	}

	return nil
}

func mustConnectCassandra(url string) *gocql.Session {
	cluster := gocql.NewCluster(url)

	// The connection to Cassandra fails once in a while, try in a back-off loop
	// until a connection attempt is successful but otherwise fail.
	for i := 1; i <= 10; i++ {
		cassandra, err := cluster.CreateSession()
		if err != nil {
			nextTry := time.Duration(100 * i)
			log.Printf("failed to connect to cassandra, trying again in %sms", nextTry)
			time.Sleep(nextTry)
			continue
		}

		return cassandra
	}

	log.Fatalf("unable to establish connection to cassandra")
	return nil
}

func mustConnectBigQuery(url string) *bigquery.Client {
	client, err := bigquery.NewClient(
		context.Background(),
		"local",
		option.WithEndpoint(url),
	)
	if err != nil {
		log.Fatalf("error connecting to big query: %v", err)
	}

	return client
}

func timeEvent(eventName string) func() {
	start := time.Now()
	return func() {
		log.Printf("%s took: %s", eventName, time.Since(start))
	}
}

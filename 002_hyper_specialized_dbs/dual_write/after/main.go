package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/api/option"
)

func main() {
	log.SetFlags(0)

	cockroach := connect.MustDatabase("postgres://root@localhost:26257/defaultdb?sslmode=disable")
	defer cockroach.Close()

	bigquery := mustConnectBigQuery("http://localhost:9050")
	defer bigquery.Close()

	go bigQueryAdapter(bigquery)

	if err := simulateWriter(cockroach, bigquery); err != nil {
		log.Fatalf("error running application: %v", err)
	}
}

type order struct {
	ID     string    `json:"id"`
	UserID string    `json:"user_id"`
	Total  float64   `json:"total"`
	TS     time.Time `json:"ts"`
}

func simulateWriter(pg *pgxpool.Pool, bq *bigquery.Client) error {
	i := 0
	for range time.NewTicker(time.Millisecond * 10).C {
		o := order{
			ID:     uuid.NewString(),
			UserID: uuid.NewString(),
			Total:  rand.Float64() * 100,
			TS:     time.Now(),
		}

		if err := writeCockroach(pg, o); err != nil {
			log.Printf("writing to cockroach: %v", err)
			continue
		}

		i++
		fmt.Printf("saved %d orders\r", i)
	}

	return fmt.Errorf("finished work unexpectedly")
}

func writeCockroach(pg *pgxpool.Pool, o order) error {
	const stmt = `INSERT INTO orders (id, user_id, total, ts) VALUES ($1, $2, $3, $4)`

	// Insert row into CockroachDB but "fail" 0.1% of the time.
	if rand.Intn(1000) == 42 {
		return fmt.Errorf("simulated error in cockroachdb")
	}

	if _, err := pg.Exec(context.Background(), stmt, o.ID, o.UserID, o.Total, o.TS); err != nil {
		return fmt.Errorf("inserting row into cockroach: %w", err)
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

type bigQueryCDCMessage struct {
	Payload []order `json:"payload"`
}

func bigQueryAdapter(bq *bigquery.Client) {
	router := fiber.New()
	router.Post("/bigquery", func(c *fiber.Ctx) error {
		var msg bigQueryCDCMessage
		if err := c.BodyParser(&msg); err != nil {
			return fiber.NewError(fiber.StatusUnprocessableEntity, err.Error())
		}

		// Insert row into BigQuery but "fail" 0.1% of the time.
		if rand.Intn(1000) == 42 {
			log.Println("simulated error in bigquery")
			return fiber.NewError(fiber.StatusInternalServerError, "simulated error in bigquery")
		}

		for _, o := range msg.Payload {
			if err := writeBigQuery(bq, o); err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, err.Error())
			}
		}

		return nil
	})

	log.Fatal(router.ListenTLS(":3000", "./cert.pem", "./key.pem"))
}

func writeBigQuery(bq *bigquery.Client, o order) error {
	inserter := bq.Dataset("example").Table("orders").Inserter()

	if err := inserter.Put(context.Background(), o); err != nil {
		return fmt.Errorf("inserting row into bigquery: %w", err)
	}

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

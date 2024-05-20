package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/gofiber/fiber/v2"
	"google.golang.org/api/option"
)

func main() {
	log.SetFlags(0)

	bigquery := mustConnectBigQuery("http://localhost:9050")
	defer bigquery.Close()

	bigQueryAdapter(bigquery)
}

type order struct {
	ID     string    `json:"id"`
	UserID string    `json:"user_id"`
	Total  float64   `json:"total"`
	TS     time.Time `json:"ts"`
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

func writeBigQuery(bq *bigquery.Client, o order) error {
	inserter := bq.Dataset("example").Table("orders").Inserter()

	if err := inserter.Put(context.Background(), o); err != nil {
		return fmt.Errorf("inserting row into bigquery: %w", err)
	}

	return nil
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

		for _, o := range msg.Payload {
			if err := writeBigQuery(bq, o); err != nil {
				return fiber.NewError(fiber.StatusInternalServerError, err.Error())
			}
		}

		return nil
	})

	log.Fatal(router.ListenTLS(":3000", "./cert.pem", "./key.pem"))
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

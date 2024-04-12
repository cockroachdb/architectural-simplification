package main

import (
	"context"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	url, ok := os.LookupEnv("CONNECTION_STRING")
	if !ok {
		log.Fatalf("missing CONNECTION_STRING env var")
	}
	log.Println(url)

	db, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	defer db.Close()

	router := fiber.New()
	router.Get("/products/", handleGetProducts(db))

	log.Fatal(router.Listen(":3000"))
}

type product struct {
	SKU  string `json:"sku"`
	Name string `json:"name"`
}

func handleGetProducts(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		lang := c.Query("lang")
		if lang == "" {
			return fiber.NewError(fiber.StatusUnprocessableEntity, "missing lang query arg")
		}

		const stmt = `SELECT
										p.sku,
										i.translation AS name
									FROM products p
									JOIN i18n i ON p.name = i.word
									WHERE i.lang = $1`

		rows, err := db.Query(c.Context(), stmt, lang)
		if err != nil {
			log.Printf("error fetching products: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching products")
		}

		var products []product
		var p product
		for rows.Next() {
			if err = rows.Scan(&p.SKU, &p.Name); err != nil {
				log.Printf("error scanning product columns: %v", err)
				return fiber.NewError(fiber.StatusInternalServerError, "error scanning product columns")
			}

			products = append(products, p)
		}

		return c.JSON(products)
	}
}

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

	db, err := pgxpool.New(context.Background(), url)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	defer db.Close()

	router := fiber.New()
	router.Get("/products/:market", handleGetProducts(db))

	log.Fatal(router.Listen(":3000"))
}

type product struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	SKU   string  `json:"sku"`
	Price float64 `json:"price"`
}

func handleGetProducts(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		market := c.Params("market")

		lang := c.Query("lang")
		if lang == "" {
			return fiber.NewError(fiber.StatusUnprocessableEntity, "missing lang query arg")
		}

		const stmt = `SELECT
										p.id,
										i.translation AS name,
										pm.sku,
										pm.price
									FROM product_markets pm
									JOIN products p ON pm.product_id = p.id
									JOIN i18n i ON p.name = i.word
									WHERE pm.market = $1
									AND i.lang = $2;`

		rows, err := db.Query(c.Context(), stmt, market, lang)
		if err != nil {
			log.Printf("error fetching products: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error fetching products")
		}

		var products []product
		var p product
		for rows.Next() {
			if err = rows.Scan(&p.ID, &p.Name, &p.SKU, &p.Price); err != nil {
				log.Printf("error scanning product columns: %v", err)
				return fiber.NewError(fiber.StatusInternalServerError, "error scanning product columns")
			}

			products = append(products, p)
		}

		return c.JSON(products)
	}
}

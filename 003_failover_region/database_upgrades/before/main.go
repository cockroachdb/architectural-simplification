package main

import (
	"database/sql"
	"log"
	"math/rand"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
)

func main() {
	db, err := sql.Open("mysql", "root:password@tcp(mysql:3306)/defaultdb")
	if err != nil {
		log.Fatalf("error connecting to mysql: %v", err)
	}
	defer db.Close()

	work(db)
}

func work(db *sql.DB) {
	for range time.NewTicker(time.Second).C {
		id := uuid.NewString()

		// Insert purchase.
		stmt := `INSERT INTO purchase (id, basket_id, member_id, amount) VALUES (?, ?, ?, ?)`
		if _, err := db.Exec(stmt, id, uuid.NewString(), uuid.NewString(), rand.Float64()*100); err != nil {
			log.Printf("inserting purchase: %v", err)
			continue
		}

		// Select purchase.
		stmt = `SELECT amount FROM purchase WHERE id = ?`
		row := db.QueryRow(stmt, id)

		var value float64
		if err := row.Scan(&value); err != nil {
			log.Printf("selecting purchase: %v", err)
			continue
		}

		// Select database version.
		stmt = `SELECT version()`
		row = db.QueryRow(stmt)

		var version string
		if err := row.Scan(&version); err != nil {
			log.Printf("selecting version: %v", err)
			continue
		}

		// Feedback.
		log.Printf("ok (%s)", strings.Split(version, "(")[0])
	}

	log.Fatal("application unexpectedly exited")
}

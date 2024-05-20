package main

import (
	"fmt"
	"log"
	"time"

	"github.com/brianvoe/gofakeit/v6"
	"github.com/gocql/gocql"
)

func main() {
	cluster := gocql.NewCluster("localhost:9042")
	cluster.Keyspace = "store"

	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("error connecting to cassandra: %v", err)
	}
	defer session.Close()

	insert(session)
}

func insert(session *gocql.Session) {
	const stmt = `INSERT INTO product (id, name, description, ts)
								VALUES (?, ?, ?, ?);`

	i := 0
	for range time.NewTicker(time.Millisecond * 100).C {
		id := gocql.MustRandomUUID()
		name := gofakeit.ProductName()
		description := gofakeit.ProductDescription()
		ts := gocql.TimeUUID()

		if err := session.Query(stmt, id, name, description, ts).Exec(); err != nil {
			log.Printf("error inserting product: %v", err)
			continue
		}

		i++

		fmt.Printf("%d rows inserted\r", i)
	}
}

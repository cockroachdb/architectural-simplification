package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/cockroachdb/architectural-simplification/pkg/connect"
	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/api/option"
)

func main() {
	log.SetFlags(0)

	postgresURL := flag.String("postgres", "", "postgres connection string")
	cassandraURL := flag.String("cassandra", "", "cassandra connection string")
	bigQueryURL := flag.String("bigquery", "", "bigquery connection string")
	flag.Parse()

	postgres := connect.MustDatabase(*postgresURL)
	defer postgres.Close()

	var cassandra *gocql.Session
	if *cassandraURL != "" {
		cassandra = mustConnectCassandra(*cassandraURL)
		defer cassandra.Close()
	}

	bigquery := mustConnectBigQuery(*bigQueryURL)
	defer bigquery.Close()

	if err := simulateWriter(postgres, cassandra, bigquery); err != nil {
		log.Fatalf("error running application: %v", err)
	}
}

func simulateWriter(pg *pgxpool.Pool, cs *gocql.Session, bq *bigquery.Client) error {
	for range time.NewTicker(time.Second).C {
		postgresEOD, err := readPostgres(pg)
		if err != nil {
			log.Printf("reading eod from postgres: %v", err)
			continue
		}

		var cassandraEOD float64
		if cs != nil {
			cassandraEOD, err = readCassandra(cs)
			if err != nil {
				log.Printf("reading eod from cassandra: %v", err)
				continue
			}
		}

		bigQueryEOD, err := readBigQuery(bq)
		if err != nil {
			log.Printf("reading eod from bigquery: %v", err)
			continue
		}

		fmt.Println("\033[H\033[2J")
		fmt.Printf("postgres eod:  %2.f\n", postgresEOD)
		if cs != nil {
			fmt.Printf("cassandra eod: %2.f\n", cassandraEOD)
		}
		fmt.Printf("bigquery eod:  %2.f\n", bigQueryEOD)
	}

	return fmt.Errorf("finished work unexpectedly")
}

func readPostgres(pg *pgxpool.Pool) (float64, error) {
	const stmt = `SELECT SUM(total) FROM orders`

	var total float64
	row := pg.QueryRow(context.Background(), stmt)

	if err := row.Scan(&total); err != nil {
		return 0, nil
	}

	return total, nil
}

func readCassandra(cs *gocql.Session) (float64, error) {
	const stmt = `SELECT SUM(total) FROM example.orders;`

	var total float64
	if err := cs.Query(stmt).Scan(&total); err != nil {
		return 0, fmt.Errorf("reading eod from cassandra: %w", err)
	}

	return total, nil
}

func readBigQuery(bq *bigquery.Client) (float64, error) {
	ctx := context.Background()

	job, err := bq.Query("SELECT SUM(total) FROM example.orders").Run(context.Background())
	if err != nil {
		return 0, fmt.Errorf("reading eod from bigquery: %w", err)
	}

	status, err := job.Wait(ctx)
	if err != nil {
		return 0, fmt.Errorf("waiting for bigquery job: %w", err)
	}
	if err := status.Err(); err != nil {
		return 0, fmt.Errorf("waiting in bigquery job: %w", err)
	}

	it, err := job.Read(ctx)
	if err != nil {
		return 0, fmt.Errorf("error reading bigquery response: %w", err)
	}

	var row []bigquery.Value
	if err = it.Next(&row); err != nil {
		return 0, fmt.Errorf("scanning bigquery row: %w", err)
	}

	f, ok := row[0].(float64)
	if !ok {
		return 0, nil
	}

	return f, nil
}

func mustConnectCassandra(url string) *gocql.Session {
	// The connection to Cassandra fails once in a while, try in a back-off loop
	// until a connection attempt is successful but otherwise fail.
	for i := 1; i <= 10; i++ {
		cassandra, err := gocql.NewCluster(url).CreateSession()
		if err != nil {
			log.Printf("error connecting to cassandra: %v", err)
			time.Sleep(time.Second * time.Duration(i+1))
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

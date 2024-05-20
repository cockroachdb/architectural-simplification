# Before

**3 terminal windows (vertical)**

### Introduction

* With Cassandra (and by virtue, Scylla), data is modelled around queries:
  * If you have varying query requirements, you might need to duplicate data to achieve that
  * This is why we're using an indexer database (in this case, Postgres)

### Infra

``` sh
cp go.* 002_hyper_specialized_dbs/data_fragmentation/before/services/indexer

(
  export DEBEZIUM_VERSION=2.5.0.CR1 && \
  cd 002_hyper_specialized_dbs/data_fragmentation/before && \
  docker-compose -f compose.yaml up --build -d
)
```

Kafka topic and consumer

``` sh
kafkactl create topic products.store.product
```

### Run

Create index table

``` sh
psql "postgres://postgres:password@localhost/?sslmode=disable" \
  -c 'CREATE TABLE product (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        name TEXT NOT NULL,
        description TEXT NOT NULL,
        ts TIMESTAMPTZ NOT NULL DEFAULT now()
      );'
```

Watch index for updates

``` sh
see psql "postgres://postgres:password@localhost/?sslmode=disable" \
  -c 'SELECT COUNT(*) FROM product;'
```

Run local indexer for testing

``` sh
KAFKA_URL="localhost:9092" \
INDEX_URL="postgres://postgres:password@localhost/?sslmode=disable" \
  go run 002_hyper_specialized_dbs/data_fragmentation/before/services/indexer/main.go
```

Create keyspace and table (wait for a short while before attempting to connect)

``` sh
clear && cqlsh
```

Create keyspace and table

``` sql
CREATE KEYSPACE IF NOT EXISTS store
  WITH REPLICATION = {
    'class' : 'SimpleStrategy', 'replication_factor': 1
  }
  AND durable_writes = true;

USE store;

CREATE TABLE product (
  id uuid,
  , name text
  , description text
  , ts timeuuid
  , PRIMARY KEY (id, ts)
) WITH cdc=true;
```

Generate load

``` sh
go run 002_hyper_specialized_dbs/data_fragmentation/before/services/load/main.go
```

### Debugging

Check cdc_raw is being drained

``` sh
docker exec -it cassandra bash

watch du -sh /var/lib/cassandra/cdc_raw
```

### Summary

* Some of our customers have performed this exact migration:
  * Their query requirements outgrew the database they were writing to
  * ...which necessitated a read-specialized database

* With CockroachDB, you can scale for both reads and writes:
  * Meaning one database
  * Less infrastructure
  * Less to manage
  * ...and less to go wrong

# Before

### Infra

Containers

``` sh
(cd 001_fragile_data_integrations/cdc/before && docker compose up -d)
```

> Mention that the Debezium container sets up logical replication and host-based-authentication (more to manage)

Kafka topic and consumer

``` sh
kafkactl create topic events.public.payment
```

Table

``` sh
psql "postgres://postgres:password@localhost/?sslmode=disable" \
  -c 'CREATE TABLE payment (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        amount DECIMAL NOT NULL,
        ts TIMESTAMPTZ NOT NULL DEFAULT now()
      );'
```

Debezium connector

``` sh
curl "localhost:8083/connectors" \
  -H 'Content-Type: application/json' \
  -d '{
        "name": "db-connector",
        "config": {
          "connector.class": "io.debezium.connector.postgresql.PostgresConnector",
          "database.hostname": "postgres",
          "database.port": "5432",
          "database.user": "postgres",
          "database.password": "password",
          "database.dbname" : "postgres",
          "topic.prefix": "events",
          "tasks.max": 1,
          "decimal.handling.mode": "double",
          "include.schema.changes": "false"
        }
      }'
```

### Run

Listen for changes

``` sh
kafkactl consume events.public.payment
```

Run application

``` sh
go run 001_fragile_data_integrations/cdc/main.go \
  --url "postgres://postgres:password@localhost/?sslmode=disable"
```

# After

### Infra

``` sh
make teardown

(cd 001_fragile_data_integrations/cdc/after && docker compose up -d)
```

Convert to enterprise

``` sh
enterprise --url "postgres://root@localhost:26257/?sslmode=disable"
```

Connect

``` sh
cockroach sql --insecure
```

Create table and changefeed 

``` sql
CREATE TABLE payment (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  amount DECIMAL NOT NULL,
  ts TIMESTAMPTZ NOT NULL DEFAULT now()
);

SET CLUSTER SETTING kv.rangefeed.enabled = true;

CREATE CHANGEFEED INTO 'kafka://redpanda:29092?topic_name=events.public.payment'
WITH
  envelope=wrapped,
  kafka_sink_config = '{"Flush": {"MaxMessages": 1, "Frequency": "100ms"}, "RequiredAcks": "ONE"}'
AS SELECT
  "id",
  "amount",
  "ts"
FROM payment;
```

### Run

Listen for changes

``` sh
kafkactl create topic events.public.payment
kafkactl consume events.public.payment
```

Run application

``` sh
go run 001_fragile_data_integrations/cdc/main.go \
  --url "postgres://root@localhost:26257/?sslmode=disable"
```

# Summary

* Thanks to CockroachDB's in-built CDC capabilities, we've removed:
  * A component from our architecture
  * ...along with network hops and latencies to and from it
  * ...and any maintenance it would have required
* CDC isn't new to CockroachDB
  * But having it built _into_ CockroachDB allows our customers to:
    * Drastically simplify their architecture
    * ...and integrate and maintain less infrastructure

# Cleanup

Delete Debezium connector

``` sh
curl -X DELETE "localhost:8083/connectors/db-connector"

psql "postgres://postgres:password@localhost/?sslmode=disable" \
  -c "select pg_drop_replication_slot('debezium');"
```

Delete replication slot


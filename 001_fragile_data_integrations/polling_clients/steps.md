# Before

### Create

Database

``` sh
cockroach demo --insecure --no-example-database
```

Table

``` sql
CREATE TABLE events (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "value" DECIMAL NOT NULL,
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Run

> Mention that with just one polling consumer, we're scanning thousands of rows.

``` sh
(cd 001_fragile_data_integrations/polling_clients/before && go run main.go -c 1 -r 1001ms -w 100ms)
```

> There's no guarantee it'll be just one polling consumer. Let's see how the situation worsens with five.

``` sh
# AND LEAVE RUNNING.
(cd 001_fragile_data_integrations/polling_clients/before && go run main.go -c 5 -r 1001ms -w 100ms)
```

# After

Kafka

``` sh
docker run -d \
  --name redpanda \
  -p 9092:9092 -p 29092:29092 \
  docker.redpanda.com/redpandadata/redpanda:v22.2.2 \
    start \
      --smp 1 \
      --kafka-addr PLAINTEXT://0.0.0.0:29092,OUTSIDE://0.0.0.0:9092 \
      --advertise-kafka-addr PLAINTEXT://redpanda:29092,OUTSIDE://localhost:9092
```

CDC

``` sql
SET CLUSTER SETTING kv.rangefeed.enabled = true;

CREATE CHANGEFEED INTO 'kafka://localhost:9092?topic_name=events'
WITH
  kafka_sink_config = '{"Flush": {"MaxMessages": 1, "Frequency": "100ms"}, "RequiredAcks": "ONE"}'
AS SELECT
  "id",
  "value",
  "ts"
FROM events;
```

### Run

``` sh
(cd 001_fragile_data_integrations/polling_clients/after && go run main.go -c 5 -w 100ms)
```

### Summary

* CDC can be just as fast as a consumer that is regularly polling for database changes, only a lot more efficient.

* After:
  * Kafka consumers are still waiting for messages, which adds a slight additional delay.

### Teardown

``` sh
make teardown
```
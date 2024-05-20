**Note from Kai**

> How about collapsing "Queue Incoherence" and "Multi producers"? Its essentially the same problem, that there's multiple transactional resources involved in a non-atomic transaction (no xa/2pc). It could involve a database, a cache, a queue or an aux service, each one using independent local txns / sessions.

> The after diagram could highlight cdc to a message passing system (aka outbox) which in turn feeds the downstream systems with at-least-once delivery guarantee (or effectively-once by dedup)

# Before

### Create

Infrastructure

``` sh
(cd 001_fragile_data_integrations/queue_coherence/before && docker compose up -d)
```

Connect to CockroachDB

``` sh
cockroach sql --insecure
```

Create table and populate

``` sql
CREATE TABLE stock (
  product_id VARCHAR(36) PRIMARY KEY,
  quantity INT NOT NULL
);

INSERT INTO stock (product_id, quantity) VALUES
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 1000);
```

### Run

Without errors

``` sh
(cd 001_fragile_data_integrations/queue_coherence/before && go run main.go -r 100ms -w 250ms)
```

With errors

> Let's assume your business is dependent on at least once or exactly once delivery semantics. Perhaps you're publishing payment notifications between systems.

``` sh
(cd 001_fragile_data_integrations/queue_coherence/before/errors && go run main.go -w 100ms)
```

> In a system with dual writes, any at least once or exactly once delivery semantics guaranteed by your queue are out the window.

> The only way to protect these guarantees is to drive events from the database itself

### Summary

* This demonstrates that even if writes are successful, they can arrive at different times, leading to different consumers having a different view of what is currently correct data.

* If a write to either the database or queue fails, this exascerbates the inconsistencies.

# After

### Create

Infrastructure

``` sh
make teardown

docker run -d \
  --name redpanda \
  -p 9092:9092 -p 29092:29092 \
  docker.redpanda.com/redpandadata/redpanda:v22.2.2 \
    start \
      --smp 1 \
      --kafka-addr PLAINTEXT://0.0.0.0:29092,OUTSIDE://0.0.0.0:9092 \
      --advertise-kafka-addr PLAINTEXT://redpanda:29092,OUTSIDE://localhost:9092

cockroach demo --insecure --no-example-database
```

Create table and populate

``` sql
CREATE TABLE stock (
  product_id VARCHAR(36) PRIMARY KEY,
  quantity INT NOT NULL
);

INSERT INTO stock (product_id, quantity) VALUES
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', 1000);

SET CLUSTER SETTING kv.rangefeed.enabled = true;

CREATE CHANGEFEED INTO 'kafka://localhost:9092?topic_name=stock'
WITH
  kafka_sink_config = '{"Flush": {"MaxMessages": 1, "Frequency": "100ms"}, "RequiredAcks": "ONE"}'
AS SELECT
  product_id,
  quantity
FROM stock;
```

### Run

``` sh
(cd 001_fragile_data_integrations/queue_coherence/after && go run main.go -r 100ms -w 1s)
```

### Summary

* There's still a chance of data being out-of-sync and semantics are still at-least once.

* But we've removed complexity in the application.

* And events are published in the correct order (on the same partition).

* Single responsiblity for producer app.

* Will never see inconsistent state in terms of lost updates due to failures in the application. Writes are atomic.

### Teardown

``` sh
make teardown
```
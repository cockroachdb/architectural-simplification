# Before

``` mermaid
sequenceDiagram
    participant app
    participant db
    participant kafka
    participant etl
    
    app->>db: Write value
    db-->>kafka: Publish (raw)
    kafka-->>etl: Consume (raw)
    etl->>etl: Transform
    etl->>kafka: Publish (transformed)
    kafka->>app: Consume (transformed)
```

### Create

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

Database

``` sh
cockroach demo --insecure --no-example-database
```

Table

``` sql
CREATE TABLE order_line_item (
  order_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  product_id UUID NOT NULL,
  customer_id UUID NOT NULL,
  quantity INT NOT NULL,
  price DECIMAL NOT NULL,
  ts TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

CDC

``` sql
SET CLUSTER SETTING kv.rangefeed.enabled = true;

CREATE CHANGEFEED FOR TABLE order_line_item INTO 'kafka://localhost:9092?topic_name=raw'
WITH
  kafka_sink_config = '{"Flush": {"MaxMessages": 1, "Frequency": "100ms"}, "RequiredAcks": "ONE"}';
```

### Run

Consumer

``` sh
(cd 001_fragile_data_integrations/etl/before/services/consumer && go run main.go)
```

ETL

``` sh
(cd 001_fragile_data_integrations/etl/before/services/etl && go run main.go)
```

# After

**DON'T TEAR ANYTHING DOWN**

``` mermaid
sequenceDiagram
    participant app
    participant db
    participant kafka
    
    app->>db: Write value
    db-->>kafka: Publish (transformed)
    kafka->>app: Consume (transformed)
```

### Create

CDC

``` sql
CREATE CHANGEFEED INTO 'kafka://localhost:9092?topic_name=transformed_2'
WITH
  kafka_sink_config = '{"Flush": {"MaxMessages": 1, "Frequency": "100ms"}, "RequiredAcks": "ONE"}'
AS
  SELECT
    quantity,
    (price * 100)::INT AS price,
    ts::INT
  FROM order_line_item;
```

### Run

``` sh
(cd 001_fragile_data_integrations/etl/after && go run main.go)
```

### Summary

* CDC can be just as fast as a consumer that is regularly polling for database changes, only a lot more efficient.

* Less moving parts.

* No breach of bounded context (only one service owns the data).

### Teardown

``` sh
make teardown
```
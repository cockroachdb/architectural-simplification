# Before

**1 terminal window**

### Create

Infrastructure

``` sh
(cd 004_unecessary_caching_tier/cache_coherence/before && docker compose up -d)
```

Connect to Postgres

``` sh
dw "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
psql "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"
```

Create table and populate

``` sql
CREATE TABLE stock (
  product_id VARCHAR(36) PRIMARY KEY,
  quantity INT NOT NULL
);

INSERT INTO stock (product_id, quantity) VALUES
  ('93410c29-1609-484d-8662-ae2d0aa93cc4', 1000),
  ('47b0472d-708c-4377-aab4-acf8752f0ecb', 1000),
  ('a1a879d8-58c0-4357-a570-a57c3b1fe059', 1000),
  ('5ded80d3-fb55-4a2f-b339-43fc9c89894a', 1000),
  ('b6afe0c5-9cab-4971-8c61-127fe5b4acd1', 1000),
  ('7098227b-4883-4992-bc32-e12335efbc8c', 1000);
```

### Run

``` sh
# Read/Write ratio of 2:1
(cd 004_unecessary_caching_tier/cache_coherence/before && go run main.go -r 10ms -w 20ms)
```

### Summary

* Writes to either system will eventually fail:
  * ...and when they do, customers see an inconsistent view of data
  * ...or - more realistically - over pay, under pay, don't receive medication
* In a dynamic environment with reads and writes, a cache's value is diminished
  * With a read/write ratio of 2:1, the cache is empty half the time
  * With a read/write ratio of 1:1 (50% reads and 50% writes), the cache is more or less empty

# After

**1 terminal window**

### Create

Infrastructure

``` sh
make teardown

(cd 004_unecessary_caching_tier/cache_coherence/after && docker compose -f compose.yaml up -d)
docker exec -it node1 cockroach init --insecure
docker exec -it node1 cockroach sql --insecure
```

Create table and populate

``` sql
CREATE TABLE stock (
  product_id UUID PRIMARY KEY,
  quantity INT NOT NULL
);

INSERT INTO stock (product_id, quantity) VALUES
  ('93410c29-1609-484d-8662-ae2d0aa93cc4', 1000),
  ('47b0472d-708c-4377-aab4-acf8752f0ecb', 1000),
  ('a1a879d8-58c0-4357-a570-a57c3b1fe059', 1000),
  ('5ded80d3-fb55-4a2f-b339-43fc9c89894a', 1000),
  ('b6afe0c5-9cab-4971-8c61-127fe5b4acd1', 1000),
  ('7098227b-4883-4992-bc32-e12335efbc8c', 1000);
```

### Run

``` sh
# Read/Write ratio of 2:1
(cd 004_unecessary_caching_tier/cache_coherence/after && go run main.go -r 10ms -w 20ms)
```

### After summary

* There's no comparison this time, because
  * There's nothing to compare to
  * There's one source of truth
  * ...and with SERIALIZABLE isolation, that one source of truth will always be correct

### Todos

* Figure out why after scenario read and writes drift

### Summary

* Adding a cache ruins your ACID compliance
* Having a db and a cache introduces the dual write problem
* Any comms issues to db or cache could result in cache incoherence
* Having just a db means there's no dual write problem

### Teardown

``` sh
make teardown
```
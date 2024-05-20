# Before

**5 console windows**

### Infra

``` sh
(
  cd 003_failover_region/predictable_failover_latency/before && \
  docker compose up --build --force-recreate -d
)
```

### Run

Connect to the primary node

``` sh
dw postgres://user:password@localhost:5432/postgres
psql postgres://user:password@localhost:5432/postgres
```

Create table and insert data

``` sql
CREATE TABLE product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" VARCHAR(255) NOT NULL,
  "price" DECIMAL NOT NULL
);

INSERT INTO product ("name", "price") VALUES
  ('a', 0.99),
  ('b', 1.99),
  ('c', 2.99),
  ('d', 3.99),
  ('e', 4.99);
```

Connect to the secondary node

``` sh
dw postgres://user:password@localhost:5433/postgres
psql postgres://user:password@localhost:5433/postgres
```

Query table

``` sql
SELECT count(*) FROM product;
```

Spin up load balancer and select the primary database

``` sh
dp \
  --server "localhost:5432" \
  --server "localhost:5433" \
  --port 5430
```

Run application

``` sh
CONNECTION_STRING=postgres://user:password@localhost:5430/postgres?sslmode=disable \
  go run 003_failover_region/predictable_failover_latency/before/main.go
```

Take down primary

``` sh
docker stop primary
```

Promote replica

``` sh
docker exec -it replica bash

pg_ctl promote
```

Switch load balancer to point to replica (new primary)

### Summary

* The failover to the replica was successfully. Now what?
  * How do you get back to the primary?
  * Does the primary now become the replica?
  * How much data was lost during the outage and how to we backfill?

* Why asynchronous and not synchronous?
  * Synchronous replication slows everything down
  * Sycnrhonous requires primary and secondary to be up at all times
  * Mention (or show data loss in stand-by after failover)

# After

**3 console windows**

### Infra

``` sh
(
  cd 003_failover_region/predictable_failover_latency/after && \
  docker compose up --build --force-recreate -d
)
```

### Run

Initialise the cluster

``` sh
docker exec -it node4 cockroach init --insecure
enterprise --url "postgres://root@localhost:26002/?sslmode=disable"
docker exec -it node4 cockroach sql --insecure 
```

Create table and insert data

> MENTION: Semantically, primary and secondary just refer to leaseholder locality preferences.

``` sql
CREATE DATABASE store
  PRIMARY REGION "us-east-1"
  REGIONS "us-west-2", "eu-central-1"
  SURVIVE REGION FAILURE;

ALTER DATABASE store SET SECONDARY REGION = "us-west-2";
SHOW REGIONS FROM DATABASE store;

USE store;

CREATE TABLE product (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "name" STRING NOT NULL,
  "price" DECIMAL NOT NULL
);

INSERT INTO product ("id", "name", "price") VALUES
  ('a4aebc20-0355-40fa-86f7-b2ba25907cf2', 'a', 0.99),
  ('ba7a5891-8d82-46f3-8232-00aa7813392b', 'b', 1.99),
  ('cd5069b7-d399-4d7f-a733-e96ff31671c9', 'c', 2.99),
  ('dd9e1e42-81a8-454f-afae-c5fb9fac27f3', 'd', 3.99),
  ('ec7c7142-4bbc-418a-99c5-1fe621d0aca4', 'e', 4.99);

SET CLUSTER SETTING sql.show_ranges_deprecated_behavior.enabled = 'false';
```

Run application

``` sh
CONNECTION_STRING=postgres://root@localhost:26257/store?sslmode=disable \
  go run 003_failover_region/predictable_failover_latency/after/main.go
```

Show the replica numbers

``` sql
SELECT DISTINCT
  split_part(split_part(unnest(replica_localities), ',', 1), '=', 2) region,
  split_part(split_part(unnest(replica_localities), ',', 2), '=', 2) az,
  unnest(replicas) replica
FROM [SHOW RANGES FROM TABLE product]
ORDER BY replica;
```

View leaseholder locality (show that it's in the **primary** region)

``` sql
SELECT DISTINCT
  split_part(unnest(replica_localities), ',', 1) replica_localities,
  unnest(replicas) replica,
  lease_holder,
  range_id
FROM [SHOW RANGE FROM TABLE product FOR ROW ('9369476a-03da-43c5-a1de-211a95c90b3b')];
```

Take down node in primary region

``` sh
docker stop node1 node2 node3
```

View leaseholder locality (show that it's in the **secondary** region)

``` sql
SELECT DISTINCT
  split_part(unnest(replica_localities), ',', 1) replica_localities,
  unnest(replicas) replica,
  lease_holder,
  range_id
FROM [SHOW RANGE FROM TABLE product FOR ROW ('9369476a-03da-43c5-a1de-211a95c90b3b')];
```

``` sh
docker start node1 node2 node3
```

View leaseholder locality (show that it's in the **primary** region)

> Might need to wait a couple of minutes.

``` sql
SELECT DISTINCT
  split_part(unnest(replica_localities), ',', 1) replica_localities,
  unnest(replicas) replica,
  lease_holder,
  range_id
FROM [SHOW RANGE FROM TABLE product FOR ROW ('9369476a-03da-43c5-a1de-211a95c90b3b')];
```

### Teardown

``` sh
make teardown
```

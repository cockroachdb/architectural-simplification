# Before

### Introduction

* 3 regions, all separate
* Duplicated code to cater for differences between regions
* Separate translations and supported languages in each region
* Whenever you're running a global business, it's expensive.

### Infra

Services

``` sh
cp go.* 006_app_silos/multi_instance_architecture/before/services/eu
cp go.* 006_app_silos/multi_instance_architecture/before/services/jp
cp go.* 006_app_silos/multi_instance_architecture/before/services/us

(
  cd 006_app_silos/multi_instance_architecture/before && \
  docker compose up --build --force-recreate -d
)
```

### Run

Populate the databases

``` sh
psql "postgres://postgres:password@localhost:5432/postgres?sslmode=disable" \
  -f 006_app_silos/multi_instance_architecture/before/services/us/create.sql

psql "postgres://postgres:password@localhost:5433/postgres?sslmode=disable" \
  -f 006_app_silos/multi_instance_architecture/before/services/eu/create.sql

psql "postgres://postgres:password@localhost:5434/postgres?sslmode=disable" \
  -f 006_app_silos/multi_instance_architecture/before/services/jp/create.sql
```

Test the services

``` sh
curl -s "http://localhost:3001/products?lang=en" | jq
curl -s "http://localhost:3002/products?lang=es" | jq
curl -s "http://localhost:3003/products" | jq
```

### Teardown

### Summary

* No way of getting a holistic view of the business without a data warehousing solution (unless all products, ids and SKUs remain consistent, there's no single consistent definition of a single product across all locations)
* Adding/updating a product or translation means performing as many operations as there are regions
* Changes to the database, requires separate downtime for each region
* Data, code, infrastructure, and effort are duplicated everywhere
* Enforcing global constraints/rules (business or techincal) across regions becomes challenging
* Goes against DRY (don't repeat yourself) principles

# After

### Introduction

* 3 regions, 1 database
* Same code to cater for differences between regions
* Translations and supported languages shared by all regions

### Infra

Services

``` sh
cp go.* 006_app_silos/multi_instance_architecture/after/services/global

(
  cd 006_app_silos/multi_instance_architecture/after && \
  docker compose up --build --force-recreate -d
)
```

### Run

Initialize the database

``` sh
cockroach init --host localhost:26001 --insecure
```

> Enable or request an Enterprise license or simulate using `cockroach demo`

Create tables

``` sh
cockroach sql \
  --url "postgres://root@localhost:26001/defaultdb?sslmode=disable" \
  < 006_app_silos/multi_instance_architecture/after/services/global/create.sql
```

Connect to cluster

``` sh
cockroach sql --url "postgres://root@localhost:26001/store?sslmode=disable"
```

Observe data localities

``` sql
SELECT DISTINCT
  split_part(unnest(replica_localities), ',', 1) replica_localities,
  unnest(replicas) replica,
  lease_holder,
  range_id
FROM [SHOW RANGE FROM TABLE product_markets FOR ROW ('eu-central-1', 'a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'de')];

SELECT DISTINCT
  split_part(unnest(replica_localities), ',', 1) replica_localities,
  unnest(replicas) replica,
  lease_holder,
  range_id
FROM [SHOW RANGE FROM TABLE product_markets FOR ROW ('us-east-1', 'a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'mx')];

SELECT DISTINCT
  split_part(unnest(replica_localities), ',', 1) replica_localities,
  unnest(replicas) replica,
  lease_holder,
  range_id
FROM [SHOW RANGE FROM TABLE product_markets FOR ROW ('ap-northeast-1', 'a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'jp')];
```

Test the services

``` sh
curl -s "http://localhost:3001/products/uk?lang=en" | jq
curl -s "http://localhost:3002/products/us?lang=es" | jq
curl -s "http://localhost:3003/products/jp?lang=ja" | jq
```

### Summary

* In the new architecture, there is one single database serving all regions
* Much lower operational complexity
* Regional data can be pinned to different locations, while global data can be shared amongst them
* A consistent approach for all regions, with zero config creep

# Teardown

``` sh
make teardown
```
### Introduction

* Anectode on my caching experience.

## Shared

Infra

``` sh
(cd 004_unecessary_caching_tier/read_performance && docker compose up -d)

docker exec -it node1 cockroach init --insecure
docker exec -it node1 cockroach sql --insecure
```

Create table and populate

``` sql
CREATE TABLE stock (
  product_id UUID PRIMARY KEY,
  quantity INT NOT NULL
);

INSERT INTO stock (product_id, quantity)
  SELECT
    gen_random_uuid(),
    1000
  FROM generate_series(1, 100);
```

Copy ids into a file

``` sh
cockroach sql \
  --insecure \
  -e "SELECT json_build_object('ids', array_agg(product_id)) FROM stock" \
  | sed -n 's/.*\[\([^]]*\)\].*/\1/p' \
  | sed 's/""/"/g' \
  | sed 's/^/[ /; s/$/ ]/' \
  > 004_unecessary_caching_tier/read_performance/ids.json
```

## Before

**2 terminal windows**

### Low write/read ratio

App

``` sh
go run 004_unecessary_caching_tier/read_performance/before/main.go \
  -w 100ms
```

Load

``` sh
k6 run 004_unecessary_caching_tier/read_performance/load.js \
  --summary-trend-stats="avg,p(99)"
```

> 99.9nth percentile latencies of around X

### High write/read ratio

App

``` sh
go run 004_unecessary_caching_tier/read_performance/before/main.go \
  -w 20ms
```

Load

``` sh
k6 run 004_unecessary_caching_tier/read_performance/load.js \
  --summary-trend-stats="p(99.9)"
```

> 99.9nth percentile latencies of around X

### Summary

* In an environment with balanced read/write behaviour, the value of a cache is diminished.
* By the time we come to read a value from the cache, it's already been invalidated from previous write.

## After

### Low write/read ratio

App

``` sh
go run 004_unecessary_caching_tier/read_performance/after/main.go \
  -w 100ms
```

Load

``` sh
k6 run 004_unecessary_caching_tier/read_performance/load.js \
  --summary-trend-stats="p(99.9)"
```

> 99.9nth percentile latencies of around X

### High write/read ratio

App

``` sh
go run 004_unecessary_caching_tier/read_performance/after/main.go \
  -w 20ms
```

Load

``` sh
k6 run 004_unecessary_caching_tier/read_performance/load.js \
  --summary-trend-stats="p(99.9)"
```

> 99.9nth percentile latencies of around X


### Summary

* Possible to run run workloads directly against a database without a cache.
* Try historical reads before caching; it offers a big performance boost at the cost of slightly stale data (which you'll see from caching anyway).
* Additional application complexity.
* Additional network latency.
* Could use local in-memory caches but with multiple service instances, you need to orchestrate cache consistency across services.
* Consider caching only after careful consideration and load testing. Your environment and workload will dictate your requirements on caching.
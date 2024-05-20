# Before

### Create

Database

``` sh
cockroach demo --insecure --no-example-database
```

Table

``` sql
CREATE TABLE orders (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "customer_id" UUID NOT NULL,
  "total" DECIMAL NOT NULL,
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Run

Orders service

``` sh
(cd 001_fragile_data_integrations/purging_data/before/services/orders && go run main.go)
```

Data purger service

``` sh
(cd 001_fragile_data_integrations/purging_data/before/services/purger && go run main.go)
```

Check number of expired orders

``` sh
see -n 1 cockroach sql --insecure -e "SELECT COUNT(*) FROM orders
WHERE ts < now() + INTERVAL '5 year'";
```

# After

**Without stopping anything**

Add TTL

``` sql
ALTER TABLE orders SET (
  ttl_expiration_expression = '((ts AT TIME ZONE ''UTC'') + INTERVAL ''5 year'') AT TIME ZONE ''UTC''',
  ttl_job_cron = '* * * * *'
);
```

Stop the data purger

### Teardown

``` sh
make teardown
```
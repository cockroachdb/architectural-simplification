# Before

**3 terminal windows**

> Mention that this is a single node before we've realised we need to scale.

Create first node

``` sh
docker run -d \
  --name eu_db_1 \
  --platform linux/amd64 \
  -p 5432:5432 \
  -v eu_db_1:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=password \
    postgres:16

dw "postgres://postgres:password@localhost:5432/?sslmode=disable"
psql "postgres://postgres:password@localhost:5432/?sslmode=disable"
```

Create table

``` sql
CREATE TABLE customer (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email VARCHAR(255) NOT NULL
);

INSERT INTO customer (id, email)
  SELECT
    gen_random_uuid(),
    CONCAT(gen_random_uuid(), '@gmail.com')
  FROM generate_series(1, 1000);
```

Start client

``` sh
go run 003_failover_region/horizontal_scaling/client.go \
  --url "postgres://postgres:password@localhost:5432/?sslmode=disable"
```

### Year 1 scale-up (to 2 nodes)

``` sh
docker run -d \
  --name eu_db_2 \
  --platform linux/amd64 \
  -p 5433:5432 \
  -v eu_db_2:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=password \
    postgres:16

dw "postgres://postgres:password@localhost:5433/?sslmode=disable"
```

Add the FDW extension on eu_db_1 and eu_db_2

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE EXTENSION IF NOT EXISTS postgres_fdw;"
  
psql "postgres://postgres:password@localhost:5433/?sslmode=disable" \
  -c "CREATE EXTENSION IF NOT EXISTS postgres_fdw;"
```

Create the customer table on eu_db_2

``` sh
psql "postgres://postgres:password@localhost:5433/?sslmode=disable" \
  -c "CREATE TABLE customer (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL
      );"
```

Make eu_db_1 aware of eu_db_2

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE SERVER eu_db_2 FOREIGN DATA WRAPPER postgres_fdw
        OPTIONS (
          host 'host.docker.internal',
          port '5433',
          dbname 'postgres'
        );"
```

Map a local user to the foreign user and grant access to the FDW.

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE USER MAPPING FOR postgres
        SERVER eu_db_2
        OPTIONS (
          user 'postgres',
          password 'password'
        );
      GRANT USAGE ON FOREIGN SERVER eu_db_2 TO postgres;"
```

Partition table (on eu_db_1).

``` sql
CREATE TABLE customer_partitioned
  (LIKE customer)
  PARTITION BY HASH (id);

CREATE TABLE customer_0 PARTITION OF customer_partitioned
  FOR VALUES WITH (MODULUS 2, REMAINDER 0);

CREATE FOREIGN TABLE customer_1
  PARTITION OF customer_partitioned
  FOR VALUES WITH (MODULUS 2, REMAINDER 1)
  SERVER eu_db_2
  OPTIONS (
    table_name 'customer'
  );
```

> Draw attention to latencies (they'll increase from now on).

Test
``` sql
-- Insert data into the partitioned table.
INSERT INTO customer_partitioned
  SELECT * FROM customer;

-- Drop original table and replace with partitioned.
-- MENTION PAUSE.
BEGIN;
DROP TABLE customer;
ALTER TABLE customer_partitioned RENAME TO customer;
COMMIT;

-- Test.
INSERT INTO customer (id, email)
  SELECT
    gen_random_uuid(),
    CONCAT(gen_random_uuid(), '@gmail.com')
  FROM generate_series(1, 1000);

-- Check.
SELECT COUNT(*) FROM customer;
SELECT COUNT(*) FROM customer_0;
SELECT COUNT(*) FROM customer_1;

SELECT
  table_name,
  pg_size_pretty(pg_total_relation_size(quote_ident(table_name))),
  pg_relation_size(quote_ident(table_name))
FROM information_schema.tables
WHERE table_schema = 'public'
ORDER BY table_name;
```

> Normally you'd have to add rules in the pg_hba.conf file but
> as this is all local, I can skip this.

### Year 2 scale-up (3 nodes)

Create third node

``` sh
docker run -d \
  --name eu_db_3 \
  --platform linux/amd64 \
  -p 5434:5432 \
  -v eu_db_3:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=password \
    postgres:16

dw "postgres://postgres:password@localhost:5434/?sslmode=disable"
```

Add the FDW extension on eu_db_3

``` sh
psql "postgres://postgres:password@localhost:5434/?sslmode=disable" \
  -c "CREATE EXTENSION IF NOT EXISTS postgres_fdw;"
```

Create the customer table on eu_db_3

``` sh
psql "postgres://postgres:password@localhost:5434/?sslmode=disable" \
  -c "CREATE TABLE customer (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL
      );"
```

Make eu_db_1 aware of eu_db_3

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE SERVER eu_db_3 FOREIGN DATA WRAPPER postgres_fdw
        OPTIONS (
          host 'host.docker.internal',
          port '5434',
          dbname 'postgres'
        );"
```

Map a local user to the foreign user and grant access to the FDW.

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE USER MAPPING FOR postgres
        SERVER eu_db_3
        OPTIONS (
          user 'postgres',
          password 'password'
        );
      GRANT USAGE ON FOREIGN SERVER eu_db_3 TO postgres;"
```

Create customer_new table on eu_db_2

``` sh
psql "postgres://postgres:password@localhost:5433/?sslmode=disable" \
  -c "CREATE TABLE customer_new (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL
      );"
```

Partition table (on eu_db_1).

``` sql
CREATE TABLE customer_partitioned
  (LIKE customer)
  PARTITION BY HASH (id);

CREATE TABLE customer_0_new PARTITION OF customer_partitioned
  FOR VALUES WITH (MODULUS 3, REMAINDER 0);

CREATE FOREIGN TABLE customer_1_new
  PARTITION OF customer_partitioned
  FOR VALUES WITH (MODULUS 3, REMAINDER 1)
  SERVER eu_db_2
  OPTIONS (
    table_name 'customer_new'
  );

CREATE FOREIGN TABLE customer_2_new
  PARTITION OF customer_partitioned
  FOR VALUES WITH (MODULUS 3, REMAINDER 2)
  SERVER eu_db_3
  OPTIONS (
    table_name 'customer'
  );

-- Insert data into the partitioned table.
INSERT INTO customer_partitioned
  SELECT * FROM customer;

-- Drop original table and replace with partitioned.
-- MENTION PAUSE.
BEGIN;
DROP TABLE customer;
ALTER TABLE customer_partitioned RENAME TO customer;
ALTER TABLE customer_0_new RENAME TO customer_0;
ALTER TABLE customer_1_new RENAME TO customer_1;
ALTER TABLE customer_2_new RENAME TO customer_2;
COMMIT;

-- Test.
-- MENTION: Increased latency again.
INSERT INTO customer (id, email)
  SELECT
    gen_random_uuid(),
    CONCAT(gen_random_uuid(), '@gmail.com')
  FROM generate_series(1, 1000);

-- Check.
SELECT COUNT(*) FROM customer;
SELECT COUNT(*) FROM customer_0;
SELECT COUNT(*) FROM customer_1;
SELECT COUNT(*) FROM customer_2;

SELECT
  table_name,
  pg_size_pretty(pg_total_relation_size(quote_ident(table_name))),
  pg_relation_size(quote_ident(table_name))
FROM information_schema.tables
WHERE table_schema = 'public'
ORDER BY table_name;
```

Rename customer_new to customer on eu_db_2.

> Mention how this will knock out queries until compleleted.
> (And it can't be done as a distributed transaction).

``` sh
psql "postgres://postgres:password@localhost:5433/?sslmode=disable" \
  -c "BEGIN;
      DROP TABLE customer;
      ALTER TABLE customer_new RENAME TO customer;
      COMMIT;"
```

Back on eu_db_1, alter the foreign table back to customer.

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "ALTER FOREIGN TABLE customer_1
      OPTIONS (
        SET table_name 'customer'
      );"
```

### Year 5 scale-up (multi-region)

``` sh
docker run -d \
  --name us_db_1 \
  --platform linux/amd64 \
  -p 5435:5432 \
  -v us_db_1:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=password \
    postgres:16

docker run -d \
  --name us_db_2 \
  --platform linux/amd64 \
  -p 5436:5432 \
  -v us_db_2:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=password \
    postgres:16

docker run -d \
  --name us_db_3 \
  --platform linux/amd64 \
  -p 5437:5432 \
  -v us_db_3:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=password \
    postgres:16

dw "postgres://postgres:password@localhost:5435/?sslmode=disable"
dw "postgres://postgres:password@localhost:5436/?sslmode=disable"
dw "postgres://postgres:password@localhost:5437/?sslmode=disable"
```

Add the FDW extension on us_db_1, us_db_2, and us_db_3

``` sh
psql "postgres://postgres:password@localhost:5435/?sslmode=disable" \
  -c "CREATE EXTENSION IF NOT EXISTS postgres_fdw;"

psql "postgres://postgres:password@localhost:5436/?sslmode=disable" \
  -c "CREATE EXTENSION IF NOT EXISTS postgres_fdw;"

psql "postgres://postgres:password@localhost:5437/?sslmode=disable" \
  -c "CREATE EXTENSION IF NOT EXISTS postgres_fdw;"
```

Create the customer table on us_db_1, us_db_2, and us_db_3.

``` sh
psql "postgres://postgres:password@localhost:5435/?sslmode=disable" \
  -c "CREATE TABLE customer (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL,
        region VARCHAR(255) NOT NULL
      );"

psql "postgres://postgres:password@localhost:5436/?sslmode=disable" \
  -c "CREATE TABLE customer (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL,
        region VARCHAR(255) NOT NULL
      );"

psql "postgres://postgres:password@localhost:5437/?sslmode=disable" \
  -c "CREATE TABLE customer (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL,
        region VARCHAR(255) NOT NULL
      );"
```

Create customer_new table on eu_db_2 and eu_db_3

``` sh
psql "postgres://postgres:password@localhost:5433/?sslmode=disable" \
  -c "CREATE TABLE customer_new (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL,
        region VARCHAR(255) NOT NULL
      );"

psql "postgres://postgres:password@localhost:5434/?sslmode=disable" \
  -c "CREATE TABLE customer_new (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        email VARCHAR(255) NOT NULL,
        region VARCHAR(255) NOT NULL
      );"
```

Create a foreign servers.

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE SERVER us_db_1 FOREIGN DATA WRAPPER postgres_fdw
        OPTIONS (
          host 'host.docker.internal',
          port '5435',
          dbname 'postgres'
        );"

psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE SERVER us_db_2 FOREIGN DATA WRAPPER postgres_fdw
        OPTIONS (
          host 'host.docker.internal',
          port '5436',
          dbname 'postgres'
        );"

psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE SERVER us_db_3 FOREIGN DATA WRAPPER postgres_fdw
        OPTIONS (
          host 'host.docker.internal',
          port '5437',
          dbname 'postgres'
        );"
```

Map a local user to the foreign user and grant access to the FDW.

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE USER MAPPING FOR postgres
        SERVER us_db_1
        OPTIONS (
          user 'postgres',
          password 'password'
        );
      GRANT USAGE ON FOREIGN SERVER us_db_1 TO postgres;"

psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE USER MAPPING FOR postgres
        SERVER us_db_2
        OPTIONS (
          user 'postgres',
          password 'password'
        );
      GRANT USAGE ON FOREIGN SERVER us_db_2 TO postgres;"

psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "CREATE USER MAPPING FOR postgres
        SERVER us_db_3
        OPTIONS (
          user 'postgres',
          password 'password'
        );
      GRANT USAGE ON FOREIGN SERVER us_db_3 TO postgres;"
```

Partition table (on eu_db_1).

``` sql
-- Create partitioned version of customer table with new region table.
-- (Can't add it to the original, as it's already partitioned)
CREATE TABLE customer_partitioned (
  LIKE customer,
  region VARCHAR(255) NOT NULL DEFAULT 'uk'
)
PARTITION BY LIST (region);

-- US (using "native partitioning").
CREATE TABLE customer_us PARTITION OF customer_partitioned
FOR VALUES IN ('us')
PARTITION BY HASH (id);

CREATE FOREIGN TABLE customer_us_partitioned_0
  PARTITION OF customer_us
  FOR VALUES WITH (MODULUS 3, REMAINDER 0)
  SERVER us_db_1
  OPTIONS (
    table_name 'customer'
  );

CREATE FOREIGN TABLE customer_us_partitioned_1
  PARTITION OF customer_us
  FOR VALUES WITH (MODULUS 3, REMAINDER 1)
  SERVER us_db_2
  OPTIONS (
    table_name 'customer'
  );

CREATE FOREIGN TABLE customer_us_partitioned_2
  PARTITION OF customer_us
  FOR VALUES WITH (MODULUS 3, REMAINDER 2)
  SERVER us_db_3
  OPTIONS (
    table_name 'customer'
  );

-- UK (using "native partitioning").
CREATE TABLE customer_uk PARTITION OF customer_partitioned
FOR VALUES IN ('uk')
PARTITION BY HASH (id);

CREATE TABLE customer_uk_partitioned_0 PARTITION OF customer_uk
  FOR VALUES WITH (MODULUS 3, REMAINDER 0);

CREATE FOREIGN TABLE customer_uk_partitioned_1
  PARTITION OF customer_uk
  FOR VALUES WITH (MODULUS 3, REMAINDER 1)
  SERVER eu_db_2
  OPTIONS (
    table_name 'customer_new'
  );

CREATE FOREIGN TABLE customer_uk_partitioned_2
  PARTITION OF customer_uk
  FOR VALUES WITH (MODULUS 3, REMAINDER 2)
  SERVER eu_db_3
  OPTIONS (
    table_name 'customer_new'
  );

-- Insert data into the partitioned table.
INSERT INTO customer_partitioned
  SELECT * FROM customer;

-- Drop original table and replace with partitioned.
-- MENTION downtime and further increased latency.
BEGIN;
DROP TABLE customer;
ALTER TABLE customer_partitioned RENAME TO customer;
COMMIT;

-- Test.
-- MENTION slow insert.
INSERT INTO customer (id, email, region)
  SELECT
    gen_random_uuid(),
    CONCAT(gen_random_uuid(), '@gmail.com'),
    ('{uk, us}'::TEXT[])[CEIL(RANDOM()*2)]
  FROM generate_series(1, 1000);

-- Check.
SELECT COUNT(*) FROM customer;
SELECT COUNT(*) FROM customer_uk;
SELECT COUNT(*) FROM customer_uk_partitioned_0;
SELECT COUNT(*) FROM customer_uk_partitioned_1;
SELECT COUNT(*) FROM customer_uk_partitioned_2;
SELECT COUNT(*) FROM customer_us;
SELECT COUNT(*) FROM customer_us_partitioned_0;
SELECT COUNT(*) FROM customer_us_partitioned_1;
SELECT COUNT(*) FROM customer_us_partitioned_2;

SELECT
  table_name,
  pg_size_pretty(pg_total_relation_size(quote_ident(table_name))),
  pg_relation_size(quote_ident(table_name))
FROM information_schema.tables
WHERE table_schema = 'public'
ORDER BY table_name;
```

Rename customer_new to customer on eu_db_2 and eu_db_3.

``` sh
psql "postgres://postgres:password@localhost:5433/?sslmode=disable" \
  -c "BEGIN;
      DROP TABLE customer;
      ALTER TABLE customer_new RENAME TO customer;
      COMMIT;"

psql "postgres://postgres:password@localhost:5434/?sslmode=disable" \
  -c "BEGIN;
      DROP TABLE customer;
      ALTER TABLE customer_new RENAME TO customer;
      COMMIT;"
```

Back on eu_db_1, alter the foreign table back to customer.

``` sh
psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "ALTER FOREIGN TABLE customer_uk_partitioned_1
      OPTIONS (
        SET table_name 'customer'
      );"

psql "postgres://postgres:password@localhost:5432/?sslmode=disable" \
  -c "ALTER FOREIGN TABLE customer_uk_partitioned_2
      OPTIONS (
        SET table_name 'customer'
      );"
```

### Scratchpad

``` sh
# Insert a row directly into a US node.
psql "postgres://postgres:password@localhost:5435/?sslmode=disable" \
  -c "INSERT INTO customer (email, region) VALUES ('us_1@gmail.com', 'us');"

psql "postgres://postgres:password@localhost:5435/?sslmode=disable" \
  -c "INSERT INTO customer (email, region) VALUES ('us_2@gmail.com', 'us');"

psql "postgres://postgres:password@localhost:5435/?sslmode=disable" \
  -c "INSERT INTO customer (email, region) VALUES ('us_3@gmail.com', 'us');"

# Insert an existing id.
psql "postgres://postgres:password@localhost:5435/?sslmode=disable" \
  -c "INSERT INTO customer (id, email, region) VALUES ('26c12e0a-ee43-467f-b678-518876338407', 'us_3@gmail.com', 'us');"
```

### Summary

* All of the complexity in this demo has been incurred for just one table.
  * The complexity will only grow once I need more tables.

* If my user base shrinks, my cluster is now much harder to shrink.

* If my user base grows, I need to repartition again.

* All main queries will still need to go through 1 node
  * Makes me realise why all of the CSP databases have a master/primary write node

* This satisfies the need to keep data pinned to geographies but doesn't allow us to achieve low-latency reads for all users ("GLOBAL TABLES" in CockroachDB). For that, we'd need replication, which will add further complexity.

* No data integrity, I can insert conflicting ids into each of the servers directly.

``` sh
# Insert an existing id.
psql "postgres://postgres:password@localhost:5435/?sslmode=disable" \
  -c "INSERT INTO customer (id, email, region) VALUES ('26c12e0a-ee43-467f-b678-518876338407', 'us_3@gmail.com', 'us');"
```

# After

**3 terminal windows**

Create UK cluster

``` sh
cockroach start \
  --insecure \
  --store=path=node1,size=1GB \
  --locality=region=eu-west-2 \
  --listen-addr=localhost:26257 \
  --http-addr=localhost:8080 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach start \
  --insecure \
  --store=path=node2,size=1GB \
  --locality=region=eu-west-2  \
  --listen-addr=localhost:26258 \
  --http-addr=localhost:8081 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach start \
  --insecure \
  --store=path=node3,size=1GB \
  --locality=region=eu-west-2  \
  --listen-addr=localhost:26259 \
  --http-addr=localhost:8082 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach init --host localhost:26257 --insecure
cockroach sql --insecure
```

Create table

``` sql
CREATE TABLE customer (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email TEXT NOT NULL
);

INSERT INTO customer (id, email)
  SELECT
    gen_random_uuid(),
    CONCAT(gen_random_uuid()::STRING, '@gmail.com')
  FROM generate_series(1, 1000);
```

Start client

``` sh
go run 003_failover_region/horizontal_scaling/client.go \
  --url "postgres://root@localhost:26257/?sslmode=disable"
```

### Year 5 scale-up (multi-region)

``` sh
cockroach start \
  --insecure \
  --store=path=node4,size=1GB \
  --locality=region=us-east-1 \
  --listen-addr=localhost:26260 \
  --http-addr=localhost:8083 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach start \
  --insecure \
  --store=path=node5,size=1GB \
  --locality=region=us-east-1  \
  --listen-addr=localhost:26261 \
  --http-addr=localhost:8084 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach start \
  --insecure \
  --store=path=node6,size=1GB \
  --locality=region=us-east-1  \
  --listen-addr=localhost:26262 \
  --http-addr=localhost:8085 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background
```

Enable enterprise (for geo-partitioning)

``` sh
enterprise --url "postgres://root@localhost:26257/?sslmode=disable"
```

All region column

> Notice how the customer queries aren't affected.

``` sql
ALTER DATABASE defaultdb
SET PRIMARY REGION 'eu-west-2';

ALTER DATABASE defaultdb
ADD REGION 'us-east-1';

ALTER TABLE customer
ADD REGION crdb_internal_region NOT NULL DEFAULT 'eu-west-2';

ALTER TABLE customer
SET LOCALITY REGIONAL BY ROW;
```

Add data for both UK and US customers

``` sql
INSERT INTO customer (id, email, region)
  SELECT
    gen_random_uuid(),
    CONCAT(gen_random_uuid()::TEXT, '@gmail.com'),
    (ARRAY['eu-west-2', 'us-east-1'])[1 + floor((random() * 2))::int]
  FROM generate_series(1, 1000);
```

### Teardown

``` sh
make teardown
```
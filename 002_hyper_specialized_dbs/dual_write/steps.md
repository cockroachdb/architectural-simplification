# Before

**2 terminal windows**

### Dependencies

* cqlsh CLI
* gcloud CLI

### Introduction

* Lots of data duplication
* Lots of application responsibility
* Multiple writes (easy for databases to fall out-of-sync)

### Infra

Databases

``` sh
(
  cd 002_hyper_specialized_dbs/dual_write/before && \
  docker compose up --build --force-recreate -d
)
```

Postgres

``` sh
dw "postgres://postgres:password@localhost:5432/postgres?sslmode=disable"

psql "postgres://postgres:password@localhost:5432/postgres?sslmode=disable" \
  -c "CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    total DECIMAL NOT NULL,
    ts TIMESTAMP NOT NULL DEFAULT now()
  )"
```

BigQuery

``` sh
bq mk \
  --api http://localhost:9050 \
  --project_id local \
  example

bq mk \
  --api http://localhost:9050 \
  --project_id local \
  --table example.orders id:STRING,user_id:STRING,total:FLOAT,ts:TIMESTAMP
```

Cassandra

``` sh
cqlsh -e "CREATE KEYSPACE example
  WITH REPLICATION = {
    'class' : 'SimpleStrategy',
    'replication_factor' : 1
  };"

cqlsh -e "CREATE TABLE example.orders (
  id UUID PRIMARY KEY,
  user_id UUID,
  total DOUBLE,
  ts TIMESTAMP
)"
```

### Check data

``` sh
go run 002_hyper_specialized_dbs/dual_write/eod/main.go \
  --postgres "postgres://postgres:password@localhost:5432/postgres?sslmode=disable" \
  --cassandra "localhost:9042" \
  --bigquery "http://localhost:9050"
```

### Run

``` sh
go run 002_hyper_specialized_dbs/dual_write/before/main.go
```

# After

**3 terminal windows**

### Dependencies

* gcloud CLI

### Infra

Databases

``` sh
(
  cd 002_hyper_specialized_dbs/dual_write/after && \
  docker compose up --build --force-recreate -d
)
```

Tables

``` sh
# CockroachDB
cockroach sql --insecure -e "CREATE TABLE orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  total DECIMAL NOT NULL,
  ts TIMESTAMP NOT NULL DEFAULT now()
)"

# BigQuery
bq mk \
  --api http://localhost:9050 \
  --project_id local \
  example

bq mk \
  --api http://localhost:9050 \
  --project_id local \
  --table example.orders id:STRING,user_id:STRING,total:FLOAT,ts:TIMESTAMP
```

### Check data

``` sh
go run 002_hyper_specialized_dbs/dual_write/eod/main.go \
  --postgres "postgres://root@localhost:26257/defaultdb?sslmode=disable" \
  --bigquery "http://localhost:9050"
```

### Run

Start server (with certificates)

``` sh
(
  cd 002_hyper_specialized_dbs/dual_write/after && \
  openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 3650 -nodes -subj "/C=XX/ST=StateName/L=CityName/O=CompanyName/OU=CompanySectionName/CN=CommonNameOrHostname" \
)

(cd 002_hyper_specialized_dbs/dual_write/after && go run main.go)
```

Convert to enterprise

``` sh
enterprise --url "postgres://root@localhost:26257/?sslmode=disable"
```

Get cert.pem base64

``` sh
base64 -i 002_hyper_specialized_dbs/dual_write/after/cert.pem | pbcopy
```

Connect to CockroachDB

``` sh
cockroach sql --insecure
```

Create product changefeed

``` sql
SET CLUSTER SETTING kv.rangefeed.enabled = true;
SET CLUSTER SETTING changefeed.new_webhook_sink_enabled = true;

CREATE CHANGEFEED INTO 'webhook-https://host.docker.internal:3000/bigquery?insecure_tls_skip_verify=true&ca_cert=LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUY3ekNDQTllZ0F3SUJBZ0lVRzhjMUhBQzVsSjZzYWhxL0hvazBCMUJmQXBrd0RRWUpLb1pJaHZjTkFRRUwKQlFBd2dZWXhDekFKQmdOVkJBWVRBbGhZTVJJd0VBWURWUVFJREFsVGRHRjBaVTVoYldVeEVUQVBCZ05WQkFjTQpDRU5wZEhsT1lXMWxNUlF3RWdZRFZRUUtEQXREYjIxd1lXNTVUbUZ0WlRFYk1Ca0dBMVVFQ3d3U1EyOXRjR0Z1CmVWTmxZM1JwYjI1T1lXMWxNUjB3R3dZRFZRUUREQlJEYjIxdGIyNU9ZVzFsVDNKSWIzTjBibUZ0WlRBZUZ3MHkKTkRBeU1Ua3hOVEEyTURoYUZ3MHpOREF5TVRZeE5UQTJNRGhhTUlHR01Rc3dDUVlEVlFRR0V3SllXREVTTUJBRwpBMVVFQ0F3SlUzUmhkR1ZPWVcxbE1SRXdEd1lEVlFRSERBaERhWFI1VG1GdFpURVVNQklHQTFVRUNnd0xRMjl0CmNHRnVlVTVoYldVeEd6QVpCZ05WQkFzTUVrTnZiWEJoYm5sVFpXTjBhVzl1VG1GdFpURWRNQnNHQTFVRUF3d1UKUTI5dGJXOXVUbUZ0WlU5eVNHOXpkRzVoYldVd2dnSWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUNEd0F3Z2dJSwpBb0lDQVFDMjRCUThEOW1FVE5JUkJLemhzRG9Rc1cwV0NPdzRIL2xsQjVHNWtINjhuRWJhOTVNcVlOeXlpWFBUCk5BKy9qRGxFMHdlTy93bTAralhFVDBaMGFUVWUzSythL1Q5RklJZCs5cDZSRG9EY0V3dXU1TnA5WkU5RjNSQjQKSDVYZTR4ZDBGMEtPZDZ3V21GdDI0bENOM1hndmhwWmJSVVZwNHp1NENCYVRGRGxaQVI2YlA4MXI2ZE9kNTMvQQpJcWZLM2Qxa1JMUEF2Ty8wWjhuTDdNREpTRjNjQ3M4bHcxazhTYi9mVDdmaUFlWWNVQnVkakpJaHdCYUw3WDhlCnkxdVVNOVZzUEJ5ZWU0Vk9ZeUo4YzM1enVGZHFNTDBsS3N0ZFlkSUw4NkFGaGtEV3E5QWppT3N5d3R0dTU4OFYKZ1o5WVZ5Zk9pajVuVWtRY0lZcEQrd1pROXFmcnM2ME1FYkVyRG1VRkc0aUlCb2RGMnlhUFFCSjgxaVhnU2JFcwpza3FydTY1TnJSS3VCenUrT1NsT01ZbE0xUGQ4cU9LTTFSak9HS0xua0Ywc0c5SjJGbC9GVHJYK2JnYjRXa2lICjFWZVJSemp3V3AzNElzYTltc3JGVUE4dmcrb1lhMFdZVzFkQ3d3N2FPWVFveWZFdEYrdlFFVm5Mamp5YWpKOUwKVlV3NjI2d2l5S0ptaHFhcVVlUXpHdDZyQWlncnFYZkhnTFhybEM2QmlnYktqSFk5THpnV3VuNnJsbFBLL0RRRQpxZHc2R2xxTEt6TXRXT1pKU1p2WUJVUmVTVkRpYTRQd1hpaUR3T3JjS1dXTjlMQktOamtUWXV5Mm1xRGNKbHlLCnBTOU5lOXJNY0xtakJ0WW01UERBcVhlYXBSV0NZV1V1aXNFcUxEVTJ0cTNiYzA5cThRSURBUUFCbzFNd1VUQWQKQmdOVkhRNEVGZ1FVdlR0RmFvZER0UG9mNUMxdWRWNGx5OUp0N2lVd0h3WURWUjBqQkJnd0ZvQVV2VHRGYW9kRAp0UG9mNUMxdWRWNGx5OUp0N2lVd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBTkJna3Foa2lHOXcwQkFRc0ZBQU9DCkFnRUFqeTZlSmh5UmxESU82RityVEZmV0EreDdnN01EMkxadzNQZXkvUVJUbHJLZWdPSlVvTUVwTEpOdUVSNUcKRlJoMWdONmdwajZXa2g4WEg1ZFV1WmJmdWRUa0dYdXRza0ZBdXhZMklBNjZKYzNQcXhNYlgwL2h3YUkrWnVXMwpSTGxVazlta1VHZ0FOdTF5NHdXYWIwRjg4V2NtdW8zcjNqTXIzZHdTc1FVWGFUV2JNVlV2dXdCR3hJdGo2NTFhClRYc2E5UXZRL08xUys3cWx5czV4TWdMMTBNeHNBQlZMWEl4MitrY0t4TTJxdE5Hb1B4OHE0ZVJJN3BrNWMwVUEKTXhCZGJDUm11TWpYUStldFc5VFJPblhzRmc2NDdtNVpCRHRzZHVtYkQ4aG5hZTdDd2UyRkdaZk5xT3RMTVgxUAo4VlQ0b0Y4ZlBwMXM3Q05wampKYmJYbUo4L2lhcStVaHBVd3dKSXcwUzg3cFFjU2hSYk1DZzZlQWFKRXh2T3ZoCkcxUFhyam50RUtGcTdqQ1RyRElhdlJpWEgzV3I5bEhsUjJjb2ZaS0pYZG1UbU1CbWl1WWhvdDNEVGNXZXpibWEKd0Zkb3JDV1RSY3p3OURWalRESnp3NTJVNnVoRVlUYVh6aWFnaXZ6SFozVzVKMkIvRkNINnBvT1RBQ1UxUjhneQprV1BNNGtSRUJvNWpOQk9KWitqVUs3MjdnVFpUR25uRk1pcXdqS05sQ0wvbHJaeUljOWlnVE0rK1h1WXNuNXVQCnVJV2k5L3J6cWhaQnFUcTNucnJ0YzliVjloNEw1cDZZVkJRREtjNVd3dWVOVTZPaDdJRlI1ZlpXSFVLWERRdVMKWmI1NTkwRWZKeGlRemNTbUh4eXNrWDNYaVJDcUVOczVYdVJxamE5VGU3UE1DWlE9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K'
AS SELECT
  "id",
  "user_id",
  "total",
  "ts"::TIMESTAMPTZ
FROM orders;
```

### Down down BigQuery to show everything continues (and catches up)

**NOT WORKING YET** Try creating a volume.

``` sh
docker stop bigquery

docker start bigquery

bq query \
  --api http://localhost:9050 \
  --project_id local \
  "SELECT count(*) count FROM example.orders WHERE id IS NOT NULL"
```

### Teardown

``` sh
make teardown
```
# Before

### Infra

``` sh
(
  cd 001_fragile_data_integrations/edge_computing/before && \
  docker compose up --build --force-recreate -d
)
```

### Run

Connect to the primary node

``` sh
psql postgres://user:password@localhost:5432/postgres 
```

Create table and insert data

``` sql
CREATE TABLE i18n (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "word" VARCHAR(255) NOT NULL,
  "language" VARCHAR(255) NOT NULL,
  "translation" VARCHAR(255) NOT NULL
);

INSERT INTO i18n ("word", "language", "translation") VALUES
  ('Madagascar Hissing Cockroach', 'en', 'Madagascar Hissing Cockroach'),
  ('Giant Burrowing Cockroach', 'en', 'Giant Burrowing Cockroach'),
  ('Death''s Head Cockroach', 'en', 'Death''s Head Cockroach'),

  ('Madagascar Hissing Cockroach', 'de', 'Zischende Kakerlake aus Madagaskar'),
  ('Giant Burrowing Cockroach', 'de', 'Riesige grabende Kakerlake'),
  ('Death''s Head Cockroach', 'de', 'Totenkopfschabe'),

  ('Madagascar Hissing Cockroach', 'es', 'Cucaracha Silbadora de Madagascar'),
  ('Giant Burrowing Cockroach', 'es', 'Cucaracha excavadora gigante'),
  ('Death''s Head Cockroach', 'es', 'Cucaracha cabeza de muerte'),

  ('Madagascar Hissing Cockroach', 'ja', 'マダガスカルのゴキブリ'),
  ('Giant Burrowing Cockroach', 'ja', '巨大な穴を掘るゴキブリ'),
  ('Death''s Head Cockroach', 'ja', '死の頭のゴキブリ');
```

Check replication

``` sh
# US data
psql "postgres://user:password@localhost:5433/postgres" \
  -c "SELECT * FROM i18n"

# JP data
psql "postgres://user:password@localhost:5434/postgres" \
  -c "SELECT * FROM i18n"
```

### Summary

* Eventually consistent for US and JP users
* US and JP users have write data to the EU and wait for it to be asynchronously replicated back to their local regions
  * Not only is this bad for user experience
  * It's bad for regulatory compliance. Data might not be allowed to leave the US or JP
* No control over what gets replicated and what doesn't (all-or-nothing)
  * Which is also bad for regulator compliance
* Will have to partition tables to achieve data residency
  * This breaks down, as writes have to go through EU anyway

# After

### Infra

``` sh
(
  cd 001_fragile_data_integrations/edge_computing/after && \
  docker compose up --build --force-recreate -d
)
```

### Run

Initialise the cluster

``` sh
docker exec -it crdb_eu cockroach init --insecure
docker exec -it crdb_eu cockroach sql --insecure 
```

Convert to enterprise

``` sh
enterprise --url "postgres://root@localhost:26001/?sslmode=disable"
```

Create table and insert data

``` sql
CREATE DATABASE store
  PRIMARY REGION "eu-central-1"
  REGIONS "us-east-1", "ap-northeast-1";

CREATE TABLE store.i18n (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "word" STRING NOT NULL,
  "language" STRING NOT NULL,
  "translation" STRING NOT NULL
) LOCALITY GLOBAL;

INSERT INTO store.i18n ("word", "language", "translation") VALUES
  ('Madagascar Hissing Cockroach', 'en', 'Madagascar Hissing Cockroach'),
  ('Giant Burrowing Cockroach', 'en', 'Giant Burrowing Cockroach'),
  ('Death''s Head Cockroach', 'en', 'Death''s Head Cockroach'),

  ('Madagascar Hissing Cockroach', 'de', 'Zischende Kakerlake aus Madagaskar'),
  ('Giant Burrowing Cockroach', 'de', 'Riesige grabende Kakerlake'),
  ('Death''s Head Cockroach', 'de', 'Totenkopfschabe'),

  ('Madagascar Hissing Cockroach', 'es', 'Cucaracha Silbadora de Madagascar'),
  ('Giant Burrowing Cockroach', 'es', 'Cucaracha excavadora gigante'),
  ('Death''s Head Cockroach', 'es', 'Cucaracha cabeza de muerte'),

  ('Madagascar Hissing Cockroach', 'ja', 'マダガスカルのゴキブリ'),
  ('Giant Burrowing Cockroach', 'ja', '巨大な穴を掘るゴキブリ'),
  ('Death''s Head Cockroach', 'ja', '死の頭のゴキブリ');
```

Check replication

``` sh
# US data
cockroach sql --url "postgres://root@localhost:26002/store?sslmode=disable" \
  -e "SELECT * FROM i18n"

# JP data
cockroach sql --url "postgres://root@localhost:26003/store?sslmode=disable" \
  -e "SELECT * FROM i18n"
```

### Summary

* Globally consistent reads
* US and JP users can read and write to their loca regions, meaning:
  * Low read latencies
  * Low write latencies for local data
  * So great user experience
  * ...and compliant with data privacy regulations

* Ability to partition data in other tables with other topology patterns with no change to the cluster.
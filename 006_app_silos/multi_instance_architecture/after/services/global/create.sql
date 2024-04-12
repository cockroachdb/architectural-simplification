CREATE DATABASE store
  PRIMARY REGION "eu-central-1"
  REGIONS "us-east-1", "ap-northeast-1";

USE store;


SET enable_super_regions = 'on';
ALTER DATABASE store ADD SUPER REGION "eu" VALUES "eu-central-1";
ALTER DATABASE store ADD SUPER REGION "us" VALUES "us-east-1";
ALTER DATABASE store ADD SUPER REGION "jp" VALUES "ap-northeast-1";


CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name STRING NOT NULL,

  INDEX (name)
) LOCALITY GLOBAL;

INSERT INTO products (id, name) VALUES
  ('a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'Americano'),
  ('b01a8686-db0a-4a59-bc90-2c568b8af3f5', 'Cappuccino'),
  ('c5164aae-0a2e-4ce4-8b04-14255ffce085', 'Latte');


CREATE TABLE product_markets (
  product_id UUID NOT NULL REFERENCES products(id),
  market STRING NOT NULL,
  "crdb_region" CRDB_INTERNAL_REGION AS (
    CASE
      WHEN "market" IN ('de', 'es', 'uk') THEN 'eu-central-1'
      WHEN "market" IN ('mx', 'us') THEN 'us-east-1'
      WHEN "market" IN ('jp') THEN 'ap-northeast-1'
      ELSE 'eu-central-1'
    END
  ) STORED,
  sku STRING NOT NULL,
  price DECIMAL NOT NULL,
  
  PRIMARY KEY (product_id, market)
) LOCALITY REGIONAL BY ROW;

INSERT INTO product_markets (product_id, market, sku, price) VALUES
  ('a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'de', '860U', 2.90),
  ('b01a8686-db0a-4a59-bc90-2c568b8af3f5', 'de', '891A', 3.60),
  ('c5164aae-0a2e-4ce4-8b04-14255ffce085', 'de', '874P', 3.80),
  ('a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'es', '860U', 2.90),
  ('b01a8686-db0a-4a59-bc90-2c568b8af3f5', 'es', '891A', 3.60),
  ('c5164aae-0a2e-4ce4-8b04-14255ffce085', 'es', '874P', 3.80),
  ('a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'uk', '860U', 2.50),
  ('b01a8686-db0a-4a59-bc90-2c568b8af3f5', 'uk', '891A', 3.10),
  ('c5164aae-0a2e-4ce4-8b04-14255ffce085', 'uk', '874P', 3.30),
  ('a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'mx', 'c1', 53.60),
  ('b01a8686-db0a-4a59-bc90-2c568b8af3f5', 'mx', 'c2', 66.50),
  ('c5164aae-0a2e-4ce4-8b04-14255ffce085', 'mx', 'c3', 70.70),
  ('a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'us', 'c1', 3.70),
  ('b01a8686-db0a-4a59-bc90-2c568b8af3f5', 'us', 'c2', 3.95),
  ('c5164aae-0a2e-4ce4-8b04-14255ffce085', 'us', 'c3', 5.30),
  ('a50b1ae0-455d-4308-8d2f-ae17eeafd4b1', 'jp', 'C-001', 431),
  ('b01a8686-db0a-4a59-bc90-2c568b8af3f5', 'jp', 'C-002', 568),
  ('c5164aae-0a2e-4ce4-8b04-14255ffce085', 'jp', 'C-003', 605);


CREATE TABLE i18n(
  word STRING NOT NULL,
  lang STRING NOT NULL,
  translation STRING NOT NULL,
  
  PRIMARY KEY (word, lang),
  INDEX (lang, word) storing (translation)
) LOCALITY GLOBAL;

INSERT INTO i18n (word, lang, translation) VALUES
  ('Americano', 'de', 'Americano'),
  ('Cappuccino', 'de', 'Cappuccino'),
  ('Latte', 'de', 'Latté'),
  ('Americano', 'en', 'Americano'),
  ('Cappuccino', 'en', 'Cappuccino'),
  ('Latte', 'en', 'Latte'),
  ('Americano', 'es', 'Americano'),
  ('Cappuccino', 'es', 'Capuchino'),
  ('Latte', 'es', 'Latté'),
  ('Americano', 'ja', 'アメリカーノ'),
  ('Cappuccino', 'ja', 'カプチーノ'),
  ('Latte', 'ja', 'ラテ');
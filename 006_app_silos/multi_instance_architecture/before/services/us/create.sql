CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  sku VARCHAR(255) NOT NULL,
  price DECIMAL NOT NULL
);

INSERT INTO products (name, sku, price) VALUES
  ('Americano', 'c1', 5.30),
  ('Cappuccino', 'c2', 5.30),
  ('Latte', 'c3', 5.30);


CREATE TABLE i18n(
  word VARCHAR(255) NOT NULL,
  lang VARCHAR(255) NOT NULL,
  translation VARCHAR(255) NOT NULL,
  
  PRIMARY KEY (word, lang)
);
CREATE INDEX ON i18n(lang) INCLUDE (translation);

INSERT INTO i18n (word, lang, translation) VALUES
  ('Americano', 'de', 'Americano'),
  ('Cappuccino', 'de', 'Cappuccino'),
  ('Latte', 'de', 'Latté'),
  ('Americano', 'en', 'Americano'),
  ('Cappuccino', 'en', 'Cappuccino'),
  ('Latte', 'en', 'Latte'),
  ('Americano', 'es', 'Americano'),
  ('Cappuccino', 'es', 'Capuchino'),
  ('Latte', 'es', 'Latté');
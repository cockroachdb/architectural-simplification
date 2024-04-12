CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name VARCHAR(255) NOT NULL,
  sku VARCHAR(255) NOT NULL,
  price DECIMAL NOT NULL
);

INSERT INTO products (name, sku, price) VALUES
  ('Americano', 'C-001', 490),
  ('Cappuccino', 'C-002', 550),
  ('Latte', 'C-003', 580);


CREATE TABLE i18n(
  word VARCHAR(255) NOT NULL,
  lang VARCHAR(255) NOT NULL,
  translation VARCHAR(255) NOT NULL,
  
  PRIMARY KEY (word, lang)
);
CREATE INDEX ON i18n(lang) INCLUDE (translation);

INSERT INTO i18n (word, lang, translation) VALUES
  ('Americano', 'ja', 'アメリカーノ'),
  ('Cappuccino', 'ja', 'カプチーノ'),
  ('Latte', 'ja', 'ラテ');
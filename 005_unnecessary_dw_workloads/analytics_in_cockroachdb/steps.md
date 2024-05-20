### Create

**2 terminal windows**

Infra

``` sh
(cd 005_unnecessary_dw_workloads/analytics_in_cockroachdb && docker compose up -d)
docker exec -it node1 cockroach init --insecure
docker exec -it node1 cockroach sql --insecure
```

Enable enterprise

``` sh
enterprise -url "postgres://root@localhost:26257?sslmode=disable"
```

Create table and populate

``` sql
CREATE TABLE customers (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  email STRING UNIQUE NOT NULL
);

CREATE TABLE products (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name STRING NOT NULL,
  price DECIMAL NOT NULL
);

CREATE TABLE orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  customer_id UUID NOT NULL REFERENCES customers(id),
  ts TIMESTAMPTZ NOT NULL DEFAULT now(),
  total DECIMAL NOT NULL
);

CREATE TABLE order_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID NOT NULL REFERENCES orders(id),
  product_id UUID NOT NULL REFERENCES products(id),
  quantity INTEGER NOT NULL
);

CREATE TABLE payments (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  order_id UUID REFERENCES orders(id),
  ts TIMESTAMPTZ DEFAULT now(),
  amount DECIMAL NOT NULL
);
```

Generate data

``` sh
dg \
  -c 005_unnecessary_dw_workloads/analytics_in_cockroachdb/dg.yaml \
  -o 005_unnecessary_dw_workloads/analytics_in_cockroachdb/csvs \
  -i imports.sql

python3 \
  -m http.server 9090 \
  -d 005_unnecessary_dw_workloads/analytics_in_cockroachdb/csvs
```

Import data

``` sql
IMPORT INTO customers (
	id, email
)
CSV DATA (
    'http://host.docker.internal:9090/customers.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO products (
	id, name, price
)
CSV DATA (
    'http://host.docker.internal:9090/products.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO orders (
	id, customer_id, ts, total
)
CSV DATA (
    'http://host.docker.internal:9090/orders.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO order_items (
	id, order_id, product_id, quantity
)
CSV DATA (
    'http://host.docker.internal:9090/order_items.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;

IMPORT INTO payments (
	order_id, id, ts, amount
)
CSV DATA (
    'http://host.docker.internal:9090/payments.csv'
)
WITH skip='1', nullif = '', allow_quoted_null;
```

Simulate transactional workload

``` sh
go run 005_unnecessary_dw_workloads/analytics_in_cockroachdb/main.go

k6 run 005_unnecessary_dw_workloads/analytics_in_cockroachdb/load.js

```

**DEBUG** Test transactional workload

``` sh
# Insert customer
curl "http://localhost:3000/customers" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "68b790f4-9527-4a51-b0fd-b530613f34a9",
    "email": "abc@gmail.com"
  }'

# Get products
curl "http://localhost:3000/products" | jq

# Insert order
curl "http://localhost:3000/orders" \
  -H 'Content-Type: application/json' \
  -d '{
    "id": "318c7a41-aacb-4166-9179-706d4e60de83",
    "customer_id": "68b790f4-9527-4a51-b0fd-b530613f34a9",
    "items": [
      {
        "id": "c4c12e8f-6dc3-48d3-86ea-93cb01ee63c0",
        "quantity": 48
      },
      {
        "id": "9dcfbcd2-1848-4b50-b3f5-73b09d70d5be",
        "quantity": 1
      }
    ],
    "total": 100
  }'
```

### Analytics

Setup

``` sql
CREATE ROLE analytics WITH login;
GRANT SELECT ON * TO analytics;

CREATE USER analytics_user;
GRANT analytics TO analytics_user;

ALTER ROLE analytics SET default_transaction_use_follower_reads = 'on';
ALTER ROLE analytics SET default_transaction_priority = 'low';
ALTER ROLE analytics SET default_transaction_read_only = 'on';
ALTER ROLE analytics SET statement_timeout = '10m';

-- Remove some payments for the analytics queries.
DELETE FROM payments p
WHERE true
ORDER BY random()
LIMIT 5;
```

``` sh
cockroach sql --url "postgres://analytics@localhost:26257/defaultdb?sslmode=disable" --insecure
```

Queries

``` sql
-- Fetch a customer and their orders.
SELECT
  c.email,
  o.id,
  o.total,
  oi.quantity,
  p.price
FROM customers c
LEFT JOIN orders o ON c.id = o.customer_id
LEFT JOIN order_items oi ON o.id = oi.order_id
LEFT JOIN products p ON oi.product_id = p.id
WHERE c.id = '0a3546b5-6ad3-49b2-b960-dc6958faca30'
ORDER BY c.id, o.id, oi.id;

-- Show user-specific variables.
SHOW TRANSACTION PRIORITY;

-- Busiest months in history.
SELECT
  date_trunc('month', ts)::DATE mth,
  COUNT(*)
FROM orders
AS OF SYSTEM TIME follower_read_timestamp()
GROUP BY date_trunc('month', ts) 
ORDER BY count DESC
LIMIT 10;

-- Most profitable months in history.
SELECT
  date_trunc('month', o.ts) AS month,
  SUM(o.total) AS monthly_revenue
FROM orders o
AS OF SYSTEM TIME follower_read_timestamp()
GROUP BY month
ORDER BY monthly_revenue DESC
LIMIT 10;

-- Biggest spenders.
SELECT
  c.email,
  SUM(o.total) AS total_spend,
  COUNT(o.id) AS order_count,
  ROUND(SUM(o.total) / COUNT(o.id)) AS order_average
FROM customers c
JOIN orders o ON c.id = o.customer_id
AS OF SYSTEM TIME follower_read_timestamp()
GROUP BY c.email
ORDER BY total_spend DESC
LIMIT 10;

-- Biggest average spenders.
SELECT
  c.email,
  ROUND(AVG(o.total)) AS average_spend
FROM customers c
JOIN orders o ON c.id = o.customer_id
AS OF SYSTEM TIME follower_read_timestamp()
GROUP BY c.email
ORDER BY average_spend DESC
LIMIT 10;

-- Most popular products.
SELECT
  p.name AS product,
  SUM(oi.quantity) AS total_quantity_sold
FROM products p
JOIN order_items oi ON p.id = oi.product_id
AS OF SYSTEM TIME follower_read_timestamp()
GROUP BY p.name
ORDER BY total_quantity_sold DESC
LIMIT 10;

-- Least popular products.
SELECT
  p.name AS product,
  SUM(oi.quantity) AS total_quantity_sold
FROM products p
JOIN order_items oi ON p.id = oi.product_id
AS OF SYSTEM TIME follower_read_timestamp()
GROUP BY p.name
ORDER BY total_quantity_sold
LIMIT 10;

-- Idle customers.
SELECT
  c.email,
  MAX(o.ts) AS latest_order_date
FROM customers c
JOIN orders o ON c.id = o.customer_id
AS OF SYSTEM TIME follower_read_timestamp()
GROUP BY c.email
ORDER BY latest_order_date
LIMIT 10;

-- Orders pending payment.
SELECT
  o.id AS order_id,
  o.customer_id,
  o.total
FROM orders o
LEFT JOIN payments p ON o.id = p.order_id
AS OF SYSTEM TIME follower_read_timestamp()
WHERE p.id IS NULL
ORDER BY o.total DESC;

-- Product affinity.
WITH product_combinations AS (
  SELECT
    oi1.product_id AS product1,
    oi2.product_id AS product2,
    COUNT(DISTINCT oi1.order_id) AS order_count
  FROM order_items oi1
  JOIN order_items oi2
    ON oi1.order_id = oi2.order_id
    AND oi1.product_id != oi2.product_id
  GROUP BY oi1.product_id, oi2.product_id
)
SELECT
  product1,
  product2,
  order_count
FROM product_combinations
ORDER BY order_count DESC
LIMIT 10;

-- RFM (Recency, Frequency, Monetary) analysis.
-- To: Identify high-worth customers who've not purchased in a while.
WITH customer_rfm AS (
  SELECT
    customer_id,
    now()::DATE - MAX(ts)::DATE AS recency,
    COUNT(DISTINCT o.id) AS frequency,
    SUM(o.total) AS monetary
  FROM orders o
  WHERE o.ts <= now()
  GROUP BY customer_id
)
SELECT
  customer_id,
  recency,
  frequency,
  monetary
FROM customer_rfm
WHERE frequency >= 100
AND monetary >= 10000
ORDER BY recency DESC, frequency DESC, monetary DESC;

-- Product sales monthly moving average.
WITH product_sales AS (
  SELECT
    p.id AS product_id,
    DATE_TRUNC('year', o.ts) AS year,
    SUM(oi.quantity) AS sold
  FROM products p
  LEFT JOIN order_items oi ON p.id = oi.product_id
  LEFT JOIN orders o ON oi.order_id = o.id
  GROUP BY p.id, year
)
SELECT
  p.name,
  EXTRACT('year', ps.year),
  ps.sold,
  TRUNC(AVG(ps.sold) OVER (PARTITION BY ps.product_id ORDER BY ps.year ROWS BETWEEN 1 PRECEDING AND CURRENT ROW)) AS moving_average,
  COALESCE(ps.sold - LAG(ps.sold) OVER (PARTITION BY ps.product_id), 0) AS year_diff
FROM product_sales ps
JOIN products p ON ps.product_id = p.id
WHERE year >= '2020-01-01'
ORDER BY p.name, ps.year;

-- Customer churn prediction.
WITH
  customer_purchases AS (
    SELECT
      c.id AS customer_id,
      c.email,
      MAX(o.ts)::DATE AS last_purchase_date,
      COUNT(DISTINCT o.id) AS total_order_count,
      COUNT(DISTINCT CASE WHEN o.ts >= CURRENT_DATE - INTERVAL '30 days' THEN o.id END) AS orders_last_month,
      COUNT(DISTINCT CASE WHEN o.ts >= CURRENT_DATE - INTERVAL '1 year' THEN o.id END) AS orders_last_year
    FROM
      customers c
    LEFT JOIN
      orders o ON c.id = o.customer_id
    GROUP BY
      c.id, c.email
  ),
  customer_churn_risk AS (
    SELECT
      email,
      last_purchase_date,
      total_order_count,
      orders_last_year,
      orders_last_month,
      CASE
        WHEN total_order_count > 100
          AND NOW()::DATE - MAX(last_purchase_date)::DATE > 90
          AND orders_last_year < 30
          THEN 'high risk'
        WHEN NOW()::DATE - MAX(last_purchase_date)::DATE > 30
          AND orders_last_month < 10
          THEN 'medium risk'
        ELSE 'low risk'
      END AS churn_status
    FROM customer_purchases
    GROUP BY email, last_purchase_date, total_order_count, orders_last_year, orders_last_month
  )
SELECT
  email,
  total_order_count,
  orders_last_year,
  orders_last_month,
  churn_status
FROM customer_churn_risk
WHERE orders_last_year > 0
AND churn_status != 'low risk'
ORDER BY churn_status DESC, total_order_count DESC, orders_last_year DESC, orders_last_month DESC;
```

### Scratchpad

``` sql
-- ef8b898b-070b-4710-b57c-caaddcd09d3a + 6e01fd2d-e761-4414-87a7-35d0c6e63ff7 = 305
SELECT
  oi.order_id,
  COUNT(*)
FROM order_items oi
WHERE oi.product_id IN ('ef8b898b-070b-4710-b57c-caaddcd09d3a', '6e01fd2d-e761-4414-87a7-35d0c6e63ff7')
GROUP BY oi.order_id;
```

### Summary

* Follower reads won't interfere with other transactions or cause retries.
* Follower read transaction will always run without interruption, as they won't get pushed because of writes occurring mid query.

### Teardown

``` sh
make teardown
```
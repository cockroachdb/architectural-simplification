-- Orders
CREATE TYPE order_status AS ENUM ('in_progress', 'cancelled');

CREATE TABLE "orders" (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "status" order_status NOT NULL DEFAULT 'in_progress',
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Payments
CREATE TYPE payment_status AS ENUM ('preauth', 'auth', 'cancelled');

CREATE TABLE payments (
  "id" UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  "order_id" UUID NOT NULL,
  "amount" DECIMAL NOT NULL,
  "status" payment_status NOT NULL DEFAULT 'preauth',
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Inventory
CREATE TABLE stock (
  "product_id" UUID PRIMARY KEY NOT NULL,
  "quantity" INT NOT NULL
);

CREATE TABLE reservations (
  "order_id" UUID NOT NULL REFERENCES "orders"("id"),
  "product_id" UUID NOT NULL,
  "quantity" INT NOT NULL,
  "ts" TIMESTAMPTZ NOT NULL DEFAULT now(),

  PRIMARY KEY ("order_id", "product_id")
);

-- Fulfillment
CREATE TYPE shipment_status AS ENUM ('in_progress', 'cancelled', 'dispatched');

CREATE TABLE shipments (
  "order_id" UUID PRIMARY KEY NOT NULL,
  "status" shipment_status NOT NULL DEFAULT 'in_progress'
);

-- Testing
CREATE OR REPLACE FUNCTION check_order(o_id IN UUID) RETURNS STRING LANGUAGE SQL AS $$
  SELECT
    jsonb_pretty(
      json_build_object(
        'order_status', o.status,
        'payment_amount', p.amount,
        'payment_status', p.status,
        'product', array_agg(r.product_id),
        'quantity', array_agg(r.quantity),
        'shipment_status', sh.status
      )
    )
  FROM orders o
  LEFT JOIN payments p ON o.id = p.order_id
  LEFT JOIN reservations r ON o.id = r.order_id
  LEFT JOIN shipments sh ON o.id = sh.order_id
  WHERE o.id = o_id
  GROUP BY o.status, p.amount, p.status, sh.status;
$$;
# Before

### Create

Infrastructure

```sh
(cd 001_fragile_data_integrations/business_transactions/before && docker compose up -d)
```

Database

```sh
cockroach sql --insecure < 001_fragile_data_integrations/business_transactions/before/create.sql
```

### Run

Start app

```sh
go run 001_fragile_data_integrations/business_transactions/before/main.go
```

### Testing

Successful order

```sh
curl 'localhost:3000/orders' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": false, "reservations": false, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa')"
```

Error during order creation

```sh
curl 'localhost:3000/orders' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": true, "payments": false, "reservations": false, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb')"
```

Error during payment

```sh
curl 'localhost:3000/orders' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "cccccccc-cccc-cccc-cccc-cccccccccccc",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": true, "reservations": false, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('cccccccc-cccc-cccc-cccc-cccccccccccc')"
```

Error during reservation

```sh
curl 'localhost:3000/orders' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "dddddddd-dddd-dddd-dddd-dddddddddddd",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": false, "reservations": true, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('dddddddd-dddd-dddd-dddd-dddddddddddd')"
```

Error during shipment

```sh
curl 'localhost:3000/orders' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": false, "reservations": false, "shipments": true }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee')"
```

# After

### Create

Infrastructure

```sh
(cd 001_fragile_data_integrations/business_transactions/after && docker compose up -d)
```

Database

```sh
enterprise --url "postgres://root@localhost:26257/?sslmode=disable"
cockroach sql --insecure < 001_fragile_data_integrations/business_transactions/after/create.sql
```

### Run

Create topic

```sh
kafkactl create topic sagas
```

Start app

```sh
go run 001_fragile_data_integrations/business_transactions/after/main.go
```

### Testing

Successful saga

```sh
curl 'localhost:3000/sagas' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": false, "reservations": false, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa')"
```

Error during order creation

```sh
curl 'localhost:3000/sagas' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": true, "payments": false, "reservations": false, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb')"
```

Error during payment

```sh
curl 'localhost:3000/sagas' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "cccccccc-cccc-cccc-cccc-cccccccccccc",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": true, "reservations": false, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('cccccccc-cccc-cccc-cccc-cccccccccccc')"
```

Error during reservation

```sh
curl 'localhost:3000/sagas' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "dddddddd-dddd-dddd-dddd-dddddddddddd",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": false, "reservations": true, "shipments": false }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('dddddddd-dddd-dddd-dddd-dddddddddddd')"
```

Error during shipment

```sh
curl 'localhost:3000/sagas' \
  -H 'Content-Type:application/json' \
  -d '{
        "order_id": "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
        "payment": 26.97,
        "products": [
          { "id": "acd43cb9-2e14-4036-9e9d-d3ff9e89a9b7", "quantity": 1 },
          { "id": "b4fc9665-2ac5-4580-a618-ddb8b0440ebe", "quantity": 2 }
        ],
        "failures": { "orders": false, "payments": false, "reservations": false, "shipments": true }
      }'
```

Check database

```sh
cockroach sql --insecure -e "SELECT check_order('eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee')"
```

# Debugging

Clear down all tables

```sh
cockroach sql --insecure -e "DELETE FROM shipments WHERE true; DELETE FROM reservations WHERE true; DELETE FROM payments WHERE true; DELETE FROM orders WHERE true;" 
```
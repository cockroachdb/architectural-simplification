# Before

**2 terminal windows**

### Infra

Start Postgres (no replication, because we don't know we need it yet)

``` sh
(
  docker volume create --name=pg_primary && \
  cd 003_failover_region/database_migration/before && \
  mkdir -p primary/pg_archive && \
  chmod -R 777 primary/pg_archive && \
  docker run \
    -d \
    --name postgres_primary \
    -v pg_primary:/var/lib/postgresql/data \
    -v ${PWD}/primary/pg_archive:/mnt/server/archive \
    -p 5432:5432 \
    -e POSTGRES_USER=user \
    -e POSTGRES_DB=postgres \
    -e POSTGRES_PASSWORD=password \
      postgres:15.2-alpine \
)
```

Create table

``` sh
psql postgres://user:password@localhost:5432/postgres \
  -c 'CREATE TABLE purchase (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        basket_id UUID NOT NULL,
        member_id UUID NOT NULL,
        amount DECIMAL NOT NULL,
        timestamp TIMESTAMP NOT NULL DEFAULT now()
      );'
```

Run application

``` sh
CONNECTION_STRING="postgres://user:password@localhost:5432/postgres?sslmode=disable" \
  go run 003_failover_region/database_migration/main.go
```

### Migration

**NOTE**: At this point, we've realised we need to migration, so need to update Postgres to enable replication.

Commands

``` sh
psql postgres://user:password@localhost:5432/postgres \
  -c "CREATE ROLE replica_user WITH REPLICATION LOGIN PASSWORD 'password';"

psql postgres://user:password@localhost:5432/postgres \
  -c "GRANT ALL PRIVILEGES ON DATABASE postgres TO replica_user;"
```

Grant replica access

``` sh
docker exec -it postgres_primary bash

echo "host replication replica_user 0.0.0.0/0 md5" >> /var/lib/postgresql/data/pg_hba.conf
echo "wal_level = replica" >> /var/lib/postgresql/data/postgresql.conf
echo "max_wal_senders = 3" >> /var/lib/postgresql/data/postgresql.conf
echo "wal_log_hints = on" >> /var/lib/postgresql/data/postgresql.conf
echo "archive_mode = on" >> /var/lib/postgresql/data/postgresql.conf
echo "archive_command = 'test ! -f /mnt/server/archive/%f && cp %p /mnt/server/archive/%f'" >> /var/lib/postgresql/data/postgresql.conf

# Might need to restart twice to force archiving to start.
docker restart postgres_primary
docker restart postgres_primary
```

Bring up replica

``` sh
(
  docker volume create --name=pg_replica && \
  docker run \
    -d \
    --name postgres_replica \
    -v pg_replica:/var/lib/postgresql/data \
    -p 5431:5432 \
    -e POSTGRES_USER=user \
    -e POSTGRES_DB=postgres \
    -e POSTGRES_PASSWORD=password \
      postgres:15.2 \
)
```

Connect to the container

``` sh
docker exec -it postgres_replica bash

pg_basebackup -h host.docker.internal -p 5432 -U replica_user -D /data/ -Fp -Xs -R

chown postgres -R /data

echo "data_directory = '/data'" >> /var/lib/postgresql/data/postgresql.conf

docker restart postgres_replica
```

Stop the primary

``` sh
docker stop postgres_primary
```

Promote the secondary

``` sh
docker exec -it postgres_replica bash

runuser -u postgres -- pg_ctl -D /data promote
```

Restart application against replica

``` sh
CONNECTION_STRING="postgres://user:password@localhost:5431/postgres?sslmode=disable" \
  go run 003_failover_region/database_migration/main.go
```

# After

**3 terminal windows**

Create a cluster

``` sh
cd 003_failover_region/database_migration/after

cockroach start \
  --insecure \
  --store=path=node1 \
  --locality=region=us-east-1 \
  --listen-addr=localhost:26257 \
  --http-addr=localhost:8080 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach start \
  --insecure \
  --store=path=node2 \
  --locality=region=us-east-1  \
  --listen-addr=localhost:26258 \
  --http-addr=localhost:8081 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach start \
  --insecure \
  --store=path=node3 \
  --locality=region=us-east-1  \
  --listen-addr=localhost:26259 \
  --http-addr=localhost:8082 \
  --join='localhost:26257,localhost:26258,localhost:26259' \
  --background

cockroach init --host localhost:26257 --insecure
```

Start load balancer

``` sh
go run lb.go \
  -server localhost:26257 \
  -server localhost:26258 \
  -server localhost:26259 \
  -d
```

Create table

``` sh
cockroach sql --host localhost:26000 --insecure \
  -e 'CREATE TABLE purchase (
        id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
        basket_id UUID NOT NULL,
        member_id UUID NOT NULL,
        amount DECIMAL NOT NULL,
        timestamp TIMESTAMP NOT NULL DEFAULT now()
      );'
```

Run application

``` sh
CONNECTION_STRING="postgres://root@localhost:26000/?sslmode=disable" \
  go run 003_failover_region/database_migration/main.go
```

Add new nodes to the cluster

``` sh
cockroach start \
  --insecure \
  --store=path=node4 \
  --locality=region=us-east-2 \
  --listen-addr=localhost:26260 \
  --http-addr=localhost:8083 \
  --join='localhost:26257,localhost:26258,localhost:26259,localhost:26260,localhost:26261,localhost:26262' \
  --background

cockroach start \
  --insecure \
  --store=path=node5 \
  --locality=region=us-east-2  \
  --listen-addr=localhost:26261 \
  --http-addr=localhost:8084 \
  --join='localhost:26257,localhost:26258,localhost:26259,localhost:26260,localhost:26261,localhost:26262' \
  --background

cockroach start \
  --insecure \
  --store=path=node6 \
  --locality=region=us-east-2  \
  --listen-addr=localhost:26262 \
  --http-addr=localhost:8085 \
  --join='localhost:26257,localhost:26258,localhost:26259,localhost:26260,localhost:26261,localhost:26262' \
  --background
```

Watch console (http://localhost:8083)

**Wait for replication**

Add new nodes to load balancer

``` sh
curl -s -X POST http://localhost:8000/servers \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26260"}'

curl -s -X POST http://localhost:8000/servers \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26261"}'

curl -s -X POST http://localhost:8000/servers \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26262"}'
```

Remove original nodes from load balancer

``` sh
curl -s -X DELETE http://localhost:8000/servers \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26257"}'

curl -s -X DELETE http://localhost:8000/servers \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26258"}'

curl -s -X DELETE http://localhost:8000/servers \
  -H 'Content-Type:application/json' \
  -d '{"server": "localhost:26259"}'
```

Decommission original nodes

``` sh
cockroach node decommission 1 --host localhost:26260 --insecure
cockroach node decommission 2 --host localhost:26260 --insecure
cockroach node decommission 3 --host localhost:26260 --insecure
```

Watch console (http://localhost:8083)

### Resources

* https://stackoverflow.com/questions/32353055/how-to-start-a-stopped-docker-container-with-a-different-command

To access the /var/lib/docker stuff, run the following:

``` sh
docker run -it --privileged --pid=host debian nsenter -t 1 -m -u -n -i sh
ls /var/lib/docker/volumes/pgdata/_data
```

### Teardown

``` sh
make teardown
rm -rf **/node*
```
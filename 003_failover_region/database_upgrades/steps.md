# Before

**3 terminal windows**

### Introduction

* In order to do this without downtime, you'd have to run some kind of blue/green, failover setup.
* I cover that in another demo, so will just perform the

### Infra

Kubernetes cluster

``` sh
k3d registry create local-registry --port 9090

k3d cluster create local \
  --registry-use k3d-local-registry:9090 \
  --registry-config 003_failover_region/database_upgrades/registries.yaml \
  --k3s-arg "--disable=traefik,metrics-server@server:*;agents:*" \
  --k3s-arg "--disable=servicelb@server:*" \
  --wait
```

Deploy MySQL

``` sh
kubectl apply -f 003_failover_region/database_upgrades/before/manifests/mysql/pv.yaml
kubectl apply -f 003_failover_region/database_upgrades/before/manifests/mysql/v8.1.0.yaml
```

Wait for MySQL

``` sh
see kubectl get pods -A
```

Connect to MySQL

``` sh
kubectl run --rm -it mysqlshell --image=mysql:8.1.0 -- mysqlsh root:password@mysql --sql
```

Create tables

``` sql
CREATE DATABASE defaultdb;
USE defaultdb;

CREATE TABLE purchase (
  id VARCHAR(36) DEFAULT (uuid()) PRIMARY KEY,
  basket_id VARCHAR(36) NOT NULL,
  member_id VARCHAR(36) NOT NULL,
  amount DECIMAL NOT NULL,
  timestamp TIMESTAMP NOT NULL DEFAULT now()
);
```

Deploy application

``` sh
cp go.* 003_failover_region/database_upgrades/before
(cd 003_failover_region/database_upgrades/before && docker build -t app .)
docker tag app:latest localhost:9090/app:latest
docker push localhost:9090/app:latest
kubectl apply -f 003_failover_region/database_upgrades/before/manifests/app/deployment.yaml
```

Monitor application

``` sh
kubetail app
```

Update MySQL

``` sh
kubectl apply -f 003_failover_region/database_upgrades/before/manifests/mysql/v8.2.0.yaml
```

# After

### Infra

Deploy CockroachDB

``` sh
kubectl apply -f 003_failover_region/database_upgrades/after/manifests/cockroachdb/v23.1.11.yaml
```

Wait for CockroachDB

``` sh
see kubectl get pods -A
```

Initialise and connect

``` sh
kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach init --insecure
kubectl exec -it -n crdb cockroachdb-0 -- /cockroach/cockroach sql --insecure
```

Create table

``` sql
CREATE TABLE purchase (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  basket_id UUID NOT NULL,
  member_id UUID NOT NULL,
  amount DECIMAL NOT NULL,
  timestamp TIMESTAMP NOT NULL DEFAULT now()
);
```

Deploy application

``` sh
cp go.* 003_failover_region/database_upgrades/after
(cd 003_failover_region/database_upgrades/after && docker build -t app .)
docker tag app:latest localhost:9090/app:latest
docker push localhost:9090/app:latest

kubectl rollout restart deployment app
```

Restart kubetail

``` sh
kubetail app
```

Update CockroachDB

``` sh
kubectl apply -f 003_failover_region/database_upgrades/after/manifests/cockroachdb/v23.1.12.yaml
```
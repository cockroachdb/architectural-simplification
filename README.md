# architecture-simplification
Code samples to accompany the Architecture Simplification project

### Resources

* Accompanying YouTube video series: https://www.youtube.com/playlist?list=PL_QaflmEF2e9Dgiw8lW-Z8jq7TNDNy_3V
* Customer webinar: https://www.cockroachlabs.com/webinars/how-to-simplify-your-architecture
* Architectural Simplification landing page: https://www.cockroachlabs.com/architectural-simplification

### Table of Contents

##### 001_fragile_data_integrations
* [business_transactions](001_fragile_data_integrations/business_transactions/steps.md)
* [cdc](001_fragile_data_integrations/cdc/steps.md)
* [edge_computing](001_fragile_data_integrations/edge_computing/steps.md)
* [etl](001_fragile_data_integrations/etl/steps.md)
* [polling_clients](001_fragile_data_integrations/polling_clients/steps.md)
* [purging_data](001_fragile_data_integrations/purging_data/steps.md)
* [queue_coherence](001_fragile_data_integrations/queue_coherence/steps.md)

##### 002_hyper_specialized_dbs
* [data_fragmentation](002_hyper_specialized_dbs/data_fragmentation/steps.md)
* [dual_write](002_hyper_specialized_dbs/dual_write/steps.md)

##### 003_failover_region
* [database_migration](003_failover_region/database_migration/steps.md)
* [database_upgrades](003_failover_region/database_upgrades/steps.md)
* [horizontal_scaling](003_failover_region/horizontal_scaling/steps.md)
* [predictable_failover_latency](003_failover_region/predictable_failover_latency/steps.md)

##### 004_unecessary_caching_tier
* [cache_coherence](004_unecessary_caching_tier/cache_coherence/steps.md)
* [read_performance](004_unecessary_caching_tier/read_performance/steps.md)

##### 005_unnecessary_dw_workloads
* [analytics_in_cockroachdb](005_unnecessary_dw_workloads/analytics_in_cockroachdb/steps.md)
* [triplicating_data](005_unnecessary_dw_workloads/triplicating_data/steps.md)

##### 006_app_silos
* [multi_instance_architecture](006_app_silos/multi_instance_architecture/steps.md)

### Dependencies

The following dependencies will allow you to run all of the examples

##### Major dependencies

* [CockroachDB](https://www.cockroachlabs.com/docs/stable/install-cockroachdb.html) - the runnable binary to launch different kinds of clusters
* [Postgres](https://www.postgresql.org/download) - For its `psql` CLI
* [cqlsh](https://docs.datastax.com/en/dse/6.8/docs/installing/cqlsh.html) - The Cassandra CQL shell
* [gcloud](https://cloud.google.com/sdk/gcloud) - The Google Cloud CLI
* [bq](https://cloud.google.com/bigquery/docs/bq-command-line-tool) - The BigQuery CLI tool
* [Go](https://go.dev/dl) - The Go programming lanaguage
* [Python](https://www.python.org/downloads) - The Python programming language
* [k6](https://k6.io) - To run load tests
* [Docker](https://rancherdesktop.io) - The Docker runtime (I'm using Rancher Desktop)
* [Kubernetes](https://k3d.io/v5.6.0) - Local Kubernetes clusters (I'm using k3d)
* [Terraform](https://developer.hashicorp.com/terraform/install) - The IaC tool for creating cloud infrastructure
* [kubectl](https://kubernetes.io/docs/tasks/tools) - A CLI for interating with Kubernetes
* [kafkactl](https://github.com/deviceinsight/kafkactl) - A CLI for interacting with Kafka


##### Minor dependenties

* [make](https://www.gnu.org/software/make) - the GNU Make tool
* [dg](https://github.com/codingconcepts/dg) - a simple data generator
* [dp](https://github.com/codingconcepts/dp) - a simple dynamic proxy
* [dw](https://github.com/codingconcepts/dw) - a simple app that waits for a database to become available
* [see](https://github.com/codingconcepts/see) - a simple cross-platform version of the `watch` command
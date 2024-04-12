# architecture-simplification
Code samples to accompany the [Architecture Simplification video series](https://youtube.com/playlist?list=PL_QaflmEF2e9Dgiw8lW-Z8jq7TNDNy_3V&si=fny85nLpGpl9czUF)

## Table of Contents

### Fragile Data Integrations
Before and after examples pending

### Hyper Specialized DBs
Before and after examples pending

### Failover Regions
Before and after examples pending

### Unnecessary Caching Tier
Before and after examples pending


### Unnecessary Data Warehouse Workloads
Before and after examples pending


### Application Silos
* [multi_instance_architecture](006_app_silos/multi_instance_architecture/steps.md)

## Dependencies

The following dependencies will allow you to run all of the examples

### Major dependencies

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


### Minor dependenties

* [make](https://www.gnu.org/software/make) - the GNU Make tool
* [dg](https://github.com/codingconcepts/dg) - a simple data generator
* [dp](https://github.com/codingconcepts/dp) - a simple dynamic proxy
* [dw](https://github.com/codingconcepts/dw) - a simple app that waits for a database to become available
* [see](https://github.com/codingconcepts/see) - a simple cross-platform version of the `watch` command
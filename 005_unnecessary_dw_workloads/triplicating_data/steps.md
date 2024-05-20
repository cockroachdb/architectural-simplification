# Before

**3 terminal windows**

### Resources

* https://levelup.gitconnected.com/aws-run-an-s3-triggered-lambda-locally-using-localstack-ac05f03dc896

### Infra

Compose

``` sh
(
  cd 005_unnecessary_dw_workloads/triplicating_data/before && \
  docker compose up --build --force-recreate -d
)
```

### BigQuery

``` sh
bq mk \
  --api http://localhost:9050 \
  --project_id local \
  example

bq mk \
  --api http://localhost:9050 \
  --project_id local \
  --table example.orders id:STRING,user_id:STRING,total:FLOAT,ts:TIMESTAMP
```

### Localstack

Terraform

``` sh
(cd 005_unnecessary_dw_workloads/triplicating_data/before && terraform init)

cp go.* 005_unnecessary_dw_workloads/triplicating_data/before/s3-to-bigquery
(cd 005_unnecessary_dw_workloads/triplicating_data/before && terraform apply --auto-approve)
rm 005_unnecessary_dw_workloads/triplicating_data/before/s3-to-bigquery/go.*
rm 005_unnecessary_dw_workloads/triplicating_data/before/s3-to-bigquery/app
```

### CockroachDB

Convert to enterprise

``` sh
enterprise --url "postgres://root@localhost:26257/?sslmode=disable"
```

Connect

``` sh
cockroach sql --insecure
```

Create table and changefeed

``` sql
CREATE TABLE orders (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL,
  total DECIMAL NOT NULL,
  ts TIMESTAMP NOT NULL DEFAULT now()
);

SET CLUSTER SETTING kv.rangefeed.enabled = true;

CREATE CHANGEFEED FOR TABLE orders
  INTO 's3://s3-to-bigquery?AWS_ENDPOINT=http%3A%2F%2Fhost.docker.internal%3A4566&AWS_ACCESS_KEY_ID=fake&AWS_SECRET_ACCESS_KEY=fake&AWS_REGION=us-east-1';
```

### Test

Monitor

``` sh
see -n 1 bq query \
  --api http://localhost:9050 \
  --project_id local \
  "SELECT * FROM example.orders WHERE id IS NOT NULL"
```

Add orders

``` sql
INSERT INTO orders (user_id, total) VALUES
  ('aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa', ROUND(random() * 100, 2));

INSERT INTO orders (user_id, total) VALUES
  ('bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb', ROUND(random() * 100, 2));
```

# After

**3 terminal windows**

### Remove unecessary infrastructure

Localstack

``` sh
awslocal s3 rm s3://s3-to-bigquery --recursive
(cd 005_unnecessary_dw_workloads/triplicating_data/before && terraform destroy --auto-approve)
```

Connect

``` sh
cockroach sql --insecure
```

Delete S3 changefeed

``` sql
SELECT
  job_id
FROM [SHOW CHANGEFEED JOBS] WHERE
status = 'running'
AND description LIKE '%AWS%';

CANCEL JOB 925652922578042881;
```

### Infra

Run BigQuery webhook worker

``` sh
(
  cd 005_unnecessary_dw_workloads/triplicating_data/after && \
  openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -sha256 -days 3650 -nodes -subj "/C=XX/ST=StateName/L=CityName/O=CompanyName/OU=CompanySectionName/CN=CommonNameOrHostname" \
)

(cd 005_unnecessary_dw_workloads/triplicating_data/after && go run main.go)
```

Get cert.pem base64

``` sh
base64 -i 005_unnecessary_dw_workloads/triplicating_data/after/cert.pem | pbcopy
```

Create webhook changefeed

``` sql
SET CLUSTER SETTING changefeed.new_webhook_sink_enabled = true;

CREATE CHANGEFEED INTO 'webhook-https://host.docker.internal:3000/bigquery?insecure_tls_skip_verify=true&ca_cert=LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUY3ekNDQTllZ0F3SUJBZ0lVVnpKamxKMWpKblkrKzFzamJSODhNTW9RdDVnd0RRWUpLb1pJaHZjTkFRRUwKQlFBd2dZWXhDekFKQmdOVkJBWVRBbGhZTVJJd0VBWURWUVFJREFsVGRHRjBaVTVoYldVeEVUQVBCZ05WQkFjTQpDRU5wZEhsT1lXMWxNUlF3RWdZRFZRUUtEQXREYjIxd1lXNTVUbUZ0WlRFYk1Ca0dBMVVFQ3d3U1EyOXRjR0Z1CmVWTmxZM1JwYjI1T1lXMWxNUjB3R3dZRFZRUUREQlJEYjIxdGIyNU9ZVzFsVDNKSWIzTjBibUZ0WlRBZUZ3MHkKTkRBeU1qRXlNRE13TkRoYUZ3MHpOREF5TVRneU1ETXdORGhhTUlHR01Rc3dDUVlEVlFRR0V3SllXREVTTUJBRwpBMVVFQ0F3SlUzUmhkR1ZPWVcxbE1SRXdEd1lEVlFRSERBaERhWFI1VG1GdFpURVVNQklHQTFVRUNnd0xRMjl0CmNHRnVlVTVoYldVeEd6QVpCZ05WQkFzTUVrTnZiWEJoYm5sVFpXTjBhVzl1VG1GdFpURWRNQnNHQTFVRUF3d1UKUTI5dGJXOXVUbUZ0WlU5eVNHOXpkRzVoYldVd2dnSWlNQTBHQ1NxR1NJYjNEUUVCQVFVQUE0SUNEd0F3Z2dJSwpBb0lDQVFETTdqWjV4Uzhnb0JBbnBGZ2gyS3VoZmE5R3k0MjRqT0huK2d6NFNXSERJdDR4WWkvaExtNTcwWHhzClVrRW04dEZBeUgycDZEWUY3eXcveDJmalRGQld0N2daYi9tZFBaR01YeGlMbnJ5SWFDUDV4SjMxT1NNZVY1c0YKVHZ5YWc3VGREeHI2bHBqQ1R5Q2N0d3lJUjRWMGR2Mm9pR0ZRbDZpNXZUR21oS3BPVmpiei9FQUhCM09hYjhaYQptSzBJblNtTDBTb2E3Zy9vQVB1STFnVzFYeGxWVXdBaFI2NzJQWkYyZjZrS3NjWE4yejVHaXFIb3dTOGdVdDZPCnJuYTJ2c1hCdm93TG5QdkMzSFUrV0RZTWxVLzk5L3NNZFhnS2NKL0lhc0hkSTJvTWQyYWFEaEx4YWh6cW1SNW8Kc010ZHd0RUsrZzJ2TlM5Umo2NUljTGlLeDBqMFBqTkNWTmk4VURVUGpTYW82L2VMbitSRzkzTysvbHV4b1dJcQppeEdtSGY1dlJZTVBNRFBBcFVzUEt6cElzUCtiUzg4MVlHbUIwS3ovSUdRa2lYb3F5NzlFTCs0bDMxV1hrSFJqCmJKdTE3c21Oa3hxRi9ScVVMZXp6OS9HMGFWRHVKaVF0SFdOam16UWdwMjJNTUZhSHd2UlJjZTNtRSt5VWlzS00KN00wVmcvTEdhVUNiczdkNURvVFE2RGsvNU05TGJOOXBhSTZUcFRpYmQ3VXRCcnovWFFEdGo4bUMraFZHSkh5bApQV3NrckdaWjJ6R2VyQk9Db25NVzFiWmkxMkJJU2hUNE1aSUJXUHJBczJWZUV1N0tRWTNTWDVyd1NiUHZpZzlPCnloSGpQYzZMMkRESmhHT2Ntb1pFNXUrOU5IMHBGaFFNOTFrdnVjb01lakdsT3hFTXlRSURBUUFCbzFNd1VUQWQKQmdOVkhRNEVGZ1FVMUZ1TU5ydjNuakVYL2hvQ2lRZkY0UjIwLytrd0h3WURWUjBqQkJnd0ZvQVUxRnVNTnJ2MwpuakVYL2hvQ2lRZkY0UjIwLytrd0R3WURWUjBUQVFIL0JBVXdBd0VCL3pBTkJna3Foa2lHOXcwQkFRc0ZBQU9DCkFnRUFYMUtSdG84cHhkeWgweGRQRzJ0bkpydVpPaXNTVjBXdzJPdGk0V05qdTd6NUZVc0lTVyttUkNPM2F2SS8KY1hBbWRVSlRXeTRYYUtRZGZpOFRXUmRFT1E4K2w2S0I3VjVadmVRUS9oUDFZNUFFelV5SUxtNVBwMVRBTWJDMwpxRjd5ZXRuZjAweVdXWVU1eEpuamxvWkhqdjVsRXc4YUczZHhNQ2NFY2JhSW1RWit5MEIxQXdTVm1HbG5YTkFrClExUU5ibFcySjROaXovY2VUNHpJWnNKK0RXdGsxOFRRQkw4VWd5M0VybEFLRVFqRzJDL2crZVB2RkRQa1IyWUMKZHRPS1lBQ25mTk8wWDB0UFVJQitwZjA4YTJ5UkRtYUdBTm9OQWFlRXhCU0hhaHBDTnYyOHUwaGNWNmZoV3N0SwpxeHk2dGdpbXE2Sk1pcWJucHMrTnR1SmdyMGhMVExvQ3dYKzlrNkEvMGNvRFFVVmtnZE80YSsvNVRSaXdNRXFxCnZpUmdmR0lWMlo4cDN6b2w4Qll1akxWT24yZTNzSVhmWHhKWHhxdjRnVHhDTU1hL1hHQkhFRm9SbXRsdkQ4cFUKUEJvenBXNlErWUFyMjNnQ3M2c1VzMXl3ZkRrbXhqeEp2cGlmZkVmYVNGUnRoZEJXV0xGYmszNHFIcloyQ1FWMAoxcXdzQllIQ3JkaVY2R0FQZjRlQlB4ZkMrOTBZNHRTNkNFSVRBRGp5a0l2aXBYbEU2a0dwN3hVeEl2bDNHSjZNCmxXTXR5QlVibU1mWmhiampKZUNybmd2QU41WnVyT2F0YVovb2hSbDQyU1FLc0Zzd3dhSTFpdm8vQ3BQV3ZMK2EKcjZSOGNXdFhiTzlQRkc5OUdJejB4N0ZmOFpXOEF2Tk5wa2twekhzc2dkTk9DQ0E9Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0K'
AS SELECT
  "id",
  "user_id",
  "total",
  "ts"::TIMESTAMPTZ 
FROM orders;
```

Monitor

``` sh
bq query \
  --api http://localhost:9050 \
  --project_id local \
  "DELETE FROM example.orders WHERE id IS NOT NULL"

see -n 1 bq query \
  --api http://localhost:9050 \
  --project_id local \
  "SELECT * FROM example.orders WHERE id IS NOT NULL"
```

Add orders

``` sql
INSERT INTO orders (user_id, total) VALUES
  ('cccccccc-cccc-cccc-cccc-cccccccccccc', ROUND(random() * 100, 2));

INSERT INTO orders (user_id, total) VALUES
  ('dddddddd-dddd-dddd-dddd-dddddddddddd', ROUND(random() * 100, 2));
```

### Summary

* Biggest win is a vastly simpler architecture with few moving parts
  * In before scenario:
    * Any of the components could have failed for any reason
    * More network hops means more changes for things to go wrong
    * All of our components would have needed maintenance
    * We're at the behest of the SLAs of each and every component
    * If one component doesn't offer at least once or exactly once delivery semantics, all bets are off
* In the after scenario:
  * Less data duplication, which, for a large dataset, means:
    * Less network transfer
    * Less storage
    * Less cost
  * ...and as we saw, data also arrived into the DW with lower latency

### Teardown

``` sh
make teardown
```

### Debugging

``` sh
awslocal s3api list-buckets
awslocal s3api get-bucket-location --bucket s3-to-bigquery
awslocal s3api put-object --bucket s3-to-bigquery --key README.md --body README.md
awslocal s3api list-objects --bucket s3-to-bigquery
awslocal s3api get-bucket-notification-configuration --bucket s3-to-bigquery

awslocal sqs list-queues
awslocal sqs receive-message --queue-url http://sqs.us-east-1.localhost.localstack.cloud:4566/000000000000/s3-event-notification-queue

awslocal lambda get-function --function-name s3-to-bigquery

awslocal --endpoint-url=http://localhost:4566 logs tail /aws/lambda/s3-to-bigquery --follow
```
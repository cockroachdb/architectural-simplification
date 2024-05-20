provider "aws" {
  region     = "us-east-1"
  access_key = "fake"
  secret_key = "fake"

  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true
  s3_use_path_style           = true

  endpoints {
    s3     = "http://localhost:4566"
    sqs    = "http://localhost:4566"
    iam    = "http://localhost:4566"
    lambda = "http://localhost:4566"
  }

  default_tags {
    tags = {
      Environment = "Local"
      Service     = "LocalStack"
    }
  }
}

resource "aws_s3_bucket" "dw" {
  bucket = "s3-to-bigquery"
}

resource "aws_sqs_queue" "queue" {
  name = "s3-event-notification-queue"

  policy = <<POLICY
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": "*",
      "Action": "sqs:SendMessage",
      "Resource": "arn:aws:sqs:*:*:s3-event-notification-queue",
      "Condition": {
        "ArnEquals": { "aws:SourceArn": "${aws_s3_bucket.dw.arn}" }
      }
    }
  ]
}
POLICY
}

resource "aws_s3_bucket_notification" "bucket_notification" {
  bucket = aws_s3_bucket.dw.id

  queue {
    queue_arn = aws_sqs_queue.queue.arn
    events = [
      "s3:ObjectCreated:*"
    ]
  }
}

data "aws_iam_policy_document" "assume_role" {
  statement {
    effect = "Allow"

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }

    actions = ["sts:AssumeRole"]
  }
}

resource "aws_iam_role" "iam_for_lambda" {
  name               = "iam_for_lambda"
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
}

locals {
  src_path    = "s3-to-bigquery/main.go"
  binary_path = "s3-to-bigquery/app"
  zip_path    = "s3-to-bigquery/function.zip"
}

resource "null_resource" "function_binary" {
  triggers = {
    always_run = "${timestamp()}"
  }

  provisioner "local-exec" {
    command = "GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOFLAGS=-trimpath go build -mod=readonly -ldflags='-s -w' -o ${local.binary_path} ${local.src_path}"
  }
}

data "archive_file" "lambda" {
  type        = "zip"
  source_file = local.binary_path
  output_path = local.zip_path

  depends_on = [null_resource.function_binary]
}

resource "aws_lambda_function" "s3_to_bigquery" {
  filename         = local.zip_path
  function_name    = "s3-to-bigquery"
  role             = aws_iam_role.iam_for_lambda.arn
  handler          = "app"
  source_code_hash = data.archive_file.lambda.output_base64sha256
  runtime          = "go1.x"
  timeout          = 10
}

resource "aws_lambda_event_source_mapping" "s3_to_bigquery" {
  event_source_arn = aws_sqs_queue.queue.arn
  function_name    = aws_lambda_function.s3_to_bigquery.arn
}

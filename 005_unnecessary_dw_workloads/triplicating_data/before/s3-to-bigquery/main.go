package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"google.golang.org/api/option"
)

func main() {
	lambda.Start(handle)
}

func handle(ctx context.Context, e event) error {
	bigquery, err := connectBigQuery("http://host.docker.internal:9050")
	if err != nil {
		return fmt.Errorf("error connecting to bigquery: %w", err)
	}
	defer bigquery.Close()

	sess, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String(os.Getenv("AWS_ENDPOINT_URL")),
		Region:           aws.String("us-east-1"),
		Credentials:      credentials.NewStaticCredentials("fake", "fake", ""),
		S3ForcePathStyle: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("error creating session: %w", err)
	}

	runner := &runner{
		bq:         bigquery,
		downloader: s3manager.NewDownloader(sess),
	}

	if err = runner.sendObjectsToBigQuery(e); err != nil {
		return fmt.Errorf("error sending objects to bigquery: %w", err)
	}

	return nil
}

type runner struct {
	bq         *bigquery.Client
	downloader *s3manager.Downloader
}

func (run *runner) sendObjectsToBigQuery(e event) error {
	for _, record := range e.Records {
		var b body
		if err := json.Unmarshal([]byte(record.Body), &b); err != nil {
			return fmt.Errorf("unmarshalling body: %w", err)
		}

		for _, r := range b.Records {
			if err := run.sendObjectToBigQuery(r.S3.Bucket.Name, r.S3.Object.Key); err != nil {
				return fmt.Errorf("sending object to bigquery: %w", err)
			}
		}
	}

	return nil
}

type cdcMessage struct {
	After struct {
		ID     string  `json:"id"`
		Total  float64 `json:"total"`
		TS     string  `json:"ts"`
		UserID string  `json:"user_id"`
	} `json:"after"`
	Key []string `json:"key"`
}

func (m cdcMessage) Save() (map[string]bigquery.Value, string, error) {
	v := map[string]bigquery.Value{
		"id":      m.After.ID,
		"user_id": m.After.UserID,
		"total":   m.After.Total,
		"ts":      m.After.TS,
	}

	return v, m.After.ID, nil
}

func (run *runner) sendObjectToBigQuery(bucket, key string) error {
	msgs, err := getCDCMessage(run.downloader, bucket, key)
	if err != nil {
		return fmt.Errorf("getting object from s3: %w", err)
	}

	for _, msg := range msgs {
		inserter := run.bq.Dataset("example").Table("orders").Inserter()

		if err := inserter.Put(context.Background(), msg); err != nil {
			return fmt.Errorf("inserting row into bigquery: %w", err)
		}
	}

	return nil
}

func getCDCMessage(downloader *s3manager.Downloader, bucket, key string) ([]cdcMessage, error) {
	buf := aws.NewWriteAtBuffer([]byte{})

	_, err := downloader.Download(
		buf,
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		})

	if err != nil {
		return nil, fmt.Errorf("downloading object: %w", err)
	}

	lines := bytes.Split(buf.Bytes(), []byte("\n"))

	var msgs []cdcMessage
	for _, line := range lines {
		if bytes.Equal(line, []byte("")) {
			continue
		}

		var msg cdcMessage
		if err = json.Unmarshal(line, &msg); err != nil {
			return nil, fmt.Errorf("parsing cdc message: %w", err)
		}
		msgs = append(msgs, msg)
	}

	return msgs, nil
}

func connectBigQuery(url string) (*bigquery.Client, error) {
	client, err := bigquery.NewClient(
		context.Background(),
		"local",
		option.WithEndpoint(url),
		option.WithoutAuthentication(),
	)
	if err != nil {
		return nil, fmt.Errorf("creating bigquery client: %w", err)
	}

	return client, nil
}

type event struct {
	Records []struct {
		Attributes struct {
			ApproximateFirstReceiveTimestamp string `json:"ApproximateFirstReceiveTimestamp"`
			ApproximateReceiveCount          string `json:"ApproximateReceiveCount"`
			SenderID                         string `json:"SenderId"`
			SentTimestamp                    string `json:"SentTimestamp"`
		} `json:"attributes"`
		AwsRegion         string `json:"awsRegion"`
		Body              string `json:"body"`
		EventSource       string `json:"eventSource"`
		EventSourceARN    string `json:"eventSourceARN"`
		Md5OfBody         string `json:"md5OfBody"`
		MessageAttributes struct {
		} `json:"messageAttributes"`
		MessageID     string `json:"messageId"`
		ReceiptHandle string `json:"receiptHandle"`
	} `json:"Records"`
}

type body struct {
	Records []struct {
		EventVersion string    `json:"eventVersion"`
		EventSource  string    `json:"eventSource"`
		AwsRegion    string    `json:"awsRegion"`
		EventTime    time.Time `json:"eventTime"`
		EventName    string    `json:"eventName"`
		UserIdentity struct {
			PrincipalID string `json:"principalId"`
		} `json:"userIdentity"`
		RequestParameters struct {
			SourceIPAddress string `json:"sourceIPAddress"`
		} `json:"requestParameters"`
		ResponseElements struct {
			XAmzRequestID string `json:"x-amz-request-id"`
			XAmzID2       string `json:"x-amz-id-2"`
		} `json:"responseElements"`
		S3 struct {
			S3SchemaVersion string `json:"s3SchemaVersion"`
			ConfigurationID string `json:"configurationId"`
			Bucket          struct {
				Name          string `json:"name"`
				OwnerIdentity struct {
					PrincipalID string `json:"principalId"`
				} `json:"ownerIdentity"`
				Arn string `json:"arn"`
			} `json:"bucket"`
			Object struct {
				Key       string `json:"key"`
				Sequencer string `json:"sequencer"`
				Size      int    `json:"size"`
				ETag      string `json:"eTag"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}

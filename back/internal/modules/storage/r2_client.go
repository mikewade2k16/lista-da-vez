package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type ObjectClient interface {
	HeadBucket(ctx context.Context) error
	BucketIsEmpty(ctx context.Context) (bool, error)
	PutObject(ctx context.Context, key, contentType string, content []byte) (string, error)
	GetObject(ctx context.Context, key string) (io.ReadCloser, error)
}

// RangeObjectClient e opcional para manter seek/streaming nativo de videos sem
// materializar o arquivo inteiro na memoria da API.
type RangeObjectClient interface {
	GetObjectRange(ctx context.Context, key, byteRange string) (ObjectContent, error)
}

type ObjectProbeClient interface {
	HeadObject(ctx context.Context, key string) (etag string, exists bool, err error)
}

type MultipartObjectClient interface {
	CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error)
	UploadPart(ctx context.Context, key, uploadID string, partNumber int, content []byte) (string, error)
	CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []MultipartPart) (string, error)
	AbortMultipartUpload(ctx context.Context, key, uploadID string) error
}

type R2Client struct {
	bucket string
	client *s3.Client
}

func NewR2Client(cfg Config) (*R2Client, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", strings.TrimSpace(cfg.AccountID))
	awsConfig := aws.Config{
		Region:       "auto",
		BaseEndpoint: aws.String(endpoint),
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.SecretAccessKey),
			"",
		)),
		Retryer: func() aws.Retryer { return aws.NopRetryer{} },
	}
	return &R2Client{
		bucket: strings.TrimSpace(cfg.Bucket),
		client: s3.NewFromConfig(awsConfig),
	}, nil
}

func (client *R2Client) HeadBucket(ctx context.Context) error {
	_, err := client.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(client.bucket)})
	return err
}

func (client *R2Client) HeadObject(ctx context.Context, key string) (string, bool, error) {
	output, err := client.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(client.bucket), Key: aws.String(key),
	})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) && (apiError.ErrorCode() == "NotFound" || apiError.ErrorCode() == "NoSuchKey") {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.Trim(aws.ToString(output.ETag), "\""), true, nil
}

func (client *R2Client) BucketIsEmpty(ctx context.Context) (bool, error) {
	output, err := client.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(client.bucket),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return false, err
	}
	return len(output.Contents) == 0, nil
}

func (client *R2Client) PutObject(ctx context.Context, key, contentType string, content []byte) (string, error) {
	output, err := client.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(client.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(content),
		ContentLength: aws.Int64(int64(len(content))),
		ContentType:   aws.String(contentType),
		StorageClass:  types.StorageClassStandard,
	})
	if err != nil {
		return "", err
	}
	return strings.Trim(aws.ToString(output.ETag), "\""), nil
}

func (client *R2Client) CreateMultipartUpload(ctx context.Context, key, contentType string) (string, error) {
	output, err := client.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: aws.String(client.bucket), Key: aws.String(key), ContentType: aws.String(contentType),
		StorageClass: types.StorageClassStandard,
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(output.UploadId), nil
}

func (client *R2Client) UploadPart(ctx context.Context, key, uploadID string, partNumber int, content []byte) (string, error) {
	output, err := client.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(client.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)), Body: bytes.NewReader(content), ContentLength: aws.Int64(int64(len(content))),
	})
	if err != nil {
		return "", err
	}
	return strings.Trim(aws.ToString(output.ETag), "\""), nil
}

func (client *R2Client) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []MultipartPart) (string, error) {
	completed := make([]types.CompletedPart, 0, len(parts))
	for _, part := range parts {
		completed = append(completed, types.CompletedPart{ETag: aws.String(part.ETag), PartNumber: aws.Int32(int32(part.PartNumber))})
	}
	output, err := client.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(client.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	if err != nil {
		return "", err
	}
	return strings.Trim(aws.ToString(output.ETag), "\""), nil
}

func (client *R2Client) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	_, err := client.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket: aws.String(client.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
	})
	return err
}

func (client *R2Client) GetObject(ctx context.Context, key string) (io.ReadCloser, error) {
	output, err := client.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(client.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (client *R2Client) GetObjectRange(ctx context.Context, key, byteRange string) (ObjectContent, error) {
	output, err := client.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(client.bucket),
		Key:    aws.String(key),
		Range:  aws.String(byteRange),
	})
	if err != nil {
		return ObjectContent{}, err
	}
	return ObjectContent{
		Body:          output.Body,
		ContentLength: aws.ToInt64(output.ContentLength),
		ContentRange:  aws.ToString(output.ContentRange),
		ETag:          strings.Trim(aws.ToString(output.ETag), "\""),
	}, nil
}

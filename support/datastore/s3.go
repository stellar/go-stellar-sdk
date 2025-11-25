package datastore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/stellar/go-stellar-sdk/support/log"
	"github.com/stellar/go-stellar-sdk/support/url"
)

// S3DataStore implements DataStore for AWS S3 and S3-compatible services.
type S3DataStore struct {
	client   *s3.Client
	uploader *manager.Uploader
	bucket   string
	prefix   string
}

func NewS3DataStore(ctx context.Context, datastoreConfig DataStoreConfig) (DataStore, error) {
	destinationBucketPath, ok := datastoreConfig.Params["destination_bucket_path"]
	if !ok {
		return nil, errors.New("invalid S3 config, no destination_bucket_path")
	}
	region, ok := datastoreConfig.Params["region"]
	if !ok {
		return nil, errors.New("invalid S3 config, no region")
	}
	// endpoint_url is optional, if not provided it will use the default AWS S3 endpoint.
	endpointUrl := datastoreConfig.Params["endpoint_url"]

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpointUrl != "" {
			o.BaseEndpoint = aws.String(endpointUrl)
		}

		// Check if default credentials were successfully retrieved from the chain.
		// If not, fall back to anonymous credentials for public S3 bucket access.
		_, err := cfg.Credentials.Retrieve(ctx)
		if err != nil {
			log.Debugf("No default AWS credentials found, configuring S3 client for anonymous access")
			o.Credentials = aws.AnonymousCredentials{}
		}

		o.Region = region
		o.UsePathStyle = true
	})

	return FromS3Client(ctx, client, destinationBucketPath)
}

func FromS3Client(ctx context.Context, client *s3.Client, bucketPath string) (DataStore, error) {
	s3BucketURL := fmt.Sprintf("s3://%s", bucketPath)
	parsed, err := url.Parse(s3BucketURL)
	if err != nil {
		return nil, err
	}

	prefix := strings.TrimPrefix(parsed.Path, "/")
	bucketName := parsed.Host
	uploader := manager.NewUploader(client)

	log.Debugf("Creating S3 client for bucket: %s, prefix: %s", bucketName, prefix)

	listInput := &s3.ListObjectsV2Input{
		Bucket:  aws.String(bucketName),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	}

	_, err = client.ListObjectsV2(ctx, listInput)

	if err != nil {
		// check for http redirect specifically as it implies a region issue.
		var responseError *awshttp.ResponseError
		if errors.As(err, &responseError) {
			if responseError.HTTPStatusCode() == http.StatusMovedPermanently {
				return nil, fmt.Errorf("bucket '%s' requires a different endpoint (301 PermanentRedirect)."+
					" Please ensure the correct region is configured: %w", bucketName, err)
			}
		}

		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.ErrorCode() {
			case "NoSuchBucket":
				return nil, fmt.Errorf("bucket '%s' does not exist (NoSuchBucket): %w", bucketName, err)
			case "AccessDenied":
				return nil, fmt.Errorf("access denied to bucket '%s' (AccessDenied): %w", bucketName, err)
			default:
			}
		}
		return nil, fmt.Errorf("failed to list objects in bucket '%s': %w", bucketName, err)
	}

	return S3DataStore{client: client, uploader: uploader, bucket: bucketName, prefix: prefix}, nil
}

func (b S3DataStore) HeadObject(ctx context.Context, filePath string) (*s3.HeadObjectOutput, error) {
	filePath = path.Join(b.prefix, filePath)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(filePath),
	}

	output, err := b.client.HeadObject(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			return nil, os.ErrNotExist
		}
		return nil, err
	}

	return output, nil
}

// GetFileMetadata retrieves the metadata for the specified file in the S3-compatible bucket.
func (b S3DataStore) GetFileMetadata(ctx context.Context, filePath string) (map[string]string, error) {
	attrs, err := b.HeadObject(ctx, filePath)
	if err != nil {
		return nil, err
	}
	return attrs.Metadata, nil
}

// GetFileLastModified retrieves the last modified time of a file in the S3-compatible bucket.
func (b S3DataStore) GetFileLastModified(ctx context.Context, filePath string) (time.Time, error) {
	attrs, err := b.HeadObject(ctx, filePath)
	if err != nil {
		return time.Time{}, err
	}
	return *attrs.LastModified, nil
}

// GetFile retrieves a file from the S3-compatible bucket.
func (b S3DataStore) GetFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	filePath = path.Join(b.prefix, filePath)
	input := &s3.GetObjectInput{
		Bucket:       aws.String(b.bucket),
		Key:          aws.String(filePath),
		ChecksumMode: types.ChecksumModeEnabled, // Enable checksum validation
	}

	output, err := b.client.GetObject(ctx, input)
	if err != nil {
		log.Debugf("Error retrieving file '%s': %v", filePath, err)
		if isNotFoundError(err) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("error retrieving file %s: %w", filePath, err)
	}

	log.Debugf("File retrieved successfully: %s", filePath)
	return output.Body, nil
}

// PutFile uploads a file to S3-compatible bucket
func (b S3DataStore) PutFile(ctx context.Context, filePath string, in io.WriterTo, metaData map[string]string) error {
	err := b.putFile(ctx, filePath, in, false, metaData)

	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			log.Debugf("S3 error: %s %s %s", apiErr.ErrorCode(), apiErr.ErrorMessage(), apiErr.Error())
		}
		return fmt.Errorf("error uploading file %s: %w", filePath, err)
	}

	log.Debugf("File uploaded successfully: %s", filePath)
	return nil
}

// PutFileIfNotExists uploads a file to S3-compatible bucket only if it doesn't already exist.
func (b S3DataStore) PutFileIfNotExists(ctx context.Context, filePath string, in io.WriterTo, metaData map[string]string) (bool, error) {
	err := b.putFile(ctx, filePath, in, true, metaData)
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) {
			if apiErr.ErrorCode() == "PreconditionFailed" {
				log.Debugf("Precondition failed: %s already exists in the bucket", filePath)
				return false, nil // Treat as success
			} else {
				log.Debugf("S3 error: %s %s %s", apiErr.ErrorCode(), apiErr.ErrorMessage(), apiErr.Error())
			}
		}
		return false, fmt.Errorf("error uploading file %s: %w", filePath, err)
	}

	log.Debugf("File uploaded successfully: %s", filePath)
	return true, nil
}

// Exists checks if a file exists in the S3-compatible bucket.
func (b S3DataStore) Exists(ctx context.Context, filePath string) (bool, error) {
	filePath = path.Join(b.prefix, filePath)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(filePath),
	}

	_, err := b.client.HeadObject(ctx, input)
	if err == nil {
		return true, nil
	}

	if isNotFoundError(err) {
		return false, nil
	}

	return false, err
}

// Size retrieves the size of a file in the S3-compatible bucket.
func (b S3DataStore) Size(ctx context.Context, filePath string) (int64, error) {
	filePath = path.Join(b.prefix, filePath)
	input := &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(filePath),
	}

	output, err := b.client.HeadObject(ctx, input)
	if err != nil {
		if isNotFoundError(err) {
			return 0, os.ErrNotExist
		}
		return 0, err
	}

	return *output.ContentLength, nil
}

// Close does nothing for S3ObjectStore as it does not maintain a persistent connection.
func (b S3DataStore) Close() error {
	return nil
}

func (b S3DataStore) putFile(ctx context.Context, filePath string, in io.WriterTo, onlyIfFileDoesNotExist bool, metaData map[string]string) error {
	filePath = path.Join(b.prefix, filePath)

	buf := &bytes.Buffer{}
	// The files here are usually quite small, so there is no problem at the moment, but it would be best to optimize it in the future.
	if _, err := in.WriteTo(buf); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	// According to the document, the SDK will automatically add a checksum.
	// https://docs.aws.amazon.com/AmazonS3/latest/userguide/checking-object-integrity.html
	input := &s3.PutObjectInput{
		Bucket:            aws.String(b.bucket),
		Key:               aws.String(filePath),
		Body:              buf,
		Metadata:          metaData,
		ChecksumAlgorithm: types.ChecksumAlgorithmCrc32c,
	}

	if onlyIfFileDoesNotExist {
		input.IfNoneMatch = aws.String("*")
	}

	_, err := b.uploader.Upload(ctx, input)
	return err
}

// ListFilePaths lists up to 'limit' file paths under the provided prefix.
// Returned paths are relative to the bucket prefix.
// and ordered lexicographically ascending as provided by the backend.
// If limit <= 0, implementations default to a cap of 1,000; values > 1,000 are capped to 1,000.
func (b S3DataStore) ListFilePaths(ctx context.Context, options ListFileOptions) ([]string, error) {
	var fullPrefix string

	// When 'prefix' is empty, ensure the base prefix ends with a slash (e.g., "a/b/")
	// so the query returns only objects within that directory, not similarly named paths like "a/b-1".
	if options.Prefix == "" {
		fullPrefix = b.prefix
		if !strings.HasSuffix(fullPrefix, "/") {
			fullPrefix += "/"
		}
	} else {
		// Join the caller-provided prefix with the datastore prefix
		fullPrefix = path.Join(b.prefix, options.Prefix)
	}

	var StartAfter string
	if options.StartAfter != "" {
		StartAfter = path.Join(b.prefix, options.StartAfter)
	}
	// S3 returns lexicographically ordered keys by default
	// We page through until we collect 'limit' or exhaust results
	var keys []string
	var continuationToken *string
	remaining := options.Limit
	if remaining <= 0 || remaining > listFilePathsMaxLimit {
		remaining = listFilePathsMaxLimit
	}
	for remaining > 0 {
		maxKeys := int32(remaining)
		out, err := b.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(b.bucket),
			Prefix:            aws.String(fullPrefix),
			ContinuationToken: continuationToken,
			MaxKeys:           aws.Int32(maxKeys),
			FetchOwner:        aws.Bool(false),
			StartAfter:        aws.String(StartAfter),
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range out.Contents {
			name := aws.ToString(obj.Key)

			// Trim the configured prefix and any leading slash before appending
			relative := strings.TrimPrefix(name, b.prefix)
			relative = strings.TrimLeft(relative, "/")
			keys = append(keys, relative)

			remaining--
			if remaining == 0 {
				break
			}
		}
		if out.IsTruncated != nil && *out.IsTruncated {
			continuationToken = out.NextContinuationToken
		} else {
			break
		}
	}
	return keys, nil
}

func isNotFoundError(err error) bool {
	var noSuchKeyErr *types.NoSuchKey // for getObject
	var notFoundErr *types.NotFound   // for headObject
	if errors.As(err, &noSuchKeyErr) || errors.As(err, &notFoundErr) {
		return true
	}
	return false
}

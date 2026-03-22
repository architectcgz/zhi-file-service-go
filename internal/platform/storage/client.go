package storage

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	cfg config.StorageConfig
	raw *s3.Client
}

func Open(cfg config.StorageConfig) (*Client, error) {
	if _, err := url.ParseRequestURI(cfg.Endpoint); err != nil {
		return nil, fmt.Errorf("invalid storage endpoint: %w", err)
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}

	awsConfig, err := awscfg.LoadDefaultConfig(context.Background(),
		awscfg.WithRegion(region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKey,
			cfg.SecretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsConfig, func(options *s3.Options) {
		options.UsePathStyle = cfg.ForcePathStyle
		options.BaseEndpoint = aws.String(cfg.Endpoint)
	})

	return &Client{cfg: cfg, raw: client}, nil
}

func (c *Client) Validate(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("storage client is nil")
	}
	if strings.TrimSpace(c.cfg.PublicBucket) == "" || strings.TrimSpace(c.cfg.PrivateBucket) == "" {
		return fmt.Errorf("storage bucket config is invalid")
	}

	buckets := []string{c.cfg.PublicBucket}
	if !slices.Contains(buckets, c.cfg.PrivateBucket) {
		buckets = append(buckets, c.cfg.PrivateBucket)
	}

	for _, bucket := range buckets {
		if _, err := c.raw.HeadBucket(ctx, &s3.HeadBucketInput{
			Bucket: aws.String(bucket),
		}); err != nil {
			return fmt.Errorf("head bucket %s: %w", bucket, err)
		}
	}

	return nil
}

func (c *Client) Endpoint() string {
	if c == nil {
		return ""
	}
	return c.cfg.Endpoint
}

func (c *Client) Raw() *s3.Client {
	if c == nil {
		return nil
	}
	return c.raw
}

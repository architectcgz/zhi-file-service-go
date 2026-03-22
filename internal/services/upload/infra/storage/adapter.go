package storageinfra

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	platformstorage "github.com/architectcgz/zhi-file-service-go/internal/platform/storage"
	"github.com/architectcgz/zhi-file-service-go/internal/services/upload/ports"
	pkgstorage "github.com/architectcgz/zhi-file-service-go/pkg/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

const defaultPresignTTL = 15 * time.Minute

type Adapter struct {
	cfg      config.StorageConfig
	client   *s3.Client
	presign  *s3.PresignClient
	provider pkgstorage.Provider
}

func NewAdapter(client *platformstorage.Client, cfg config.StorageConfig) (*Adapter, error) {
	if client == nil || client.Raw() == nil {
		return nil, fmt.Errorf("%w: storage client is nil", pkgstorage.ErrInvalidBucketConfig)
	}

	return &Adapter{
		cfg:      cfg,
		client:   client.Raw(),
		presign:  s3.NewPresignClient(client.Raw()),
		provider: detectProvider(cfg),
	}, nil
}

func (a *Adapter) Resolve(accessLevel pkgstorage.AccessLevel) (pkgstorage.BucketRef, error) {
	accessLevel = normalizeAccessLevel(accessLevel)

	ref := pkgstorage.BucketRef{
		Provider: a.provider,
	}
	switch accessLevel {
	case pkgstorage.AccessLevelPublic:
		ref.BucketName = strings.TrimSpace(a.cfg.PublicBucket)
		ref.PublicBase = strings.TrimSpace(a.cfg.PublicBaseURL)
	case pkgstorage.AccessLevelPrivate:
		ref.BucketName = strings.TrimSpace(a.cfg.PrivateBucket)
	default:
		return pkgstorage.BucketRef{}, fmt.Errorf("%w: unsupported access level %q", pkgstorage.ErrInvalidBucketConfig, accessLevel)
	}

	if err := ref.Validate(); err != nil {
		return pkgstorage.BucketRef{}, err
	}
	return ref, nil
}

func (a *Adapter) Normalize(bucketName string) string {
	if normalized := strings.TrimSpace(bucketName); normalized != "" {
		return normalized
	}
	return strings.TrimSpace(a.cfg.PrivateBucket)
}

func (a *Adapter) HeadObject(ctx context.Context, ref pkgstorage.ObjectRef) (pkgstorage.ObjectMetadata, error) {
	if err := ref.Validate(); err != nil {
		return pkgstorage.ObjectMetadata{}, err
	}

	output, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(ref.BucketName),
		Key:    aws.String(ref.ObjectKey),
	})
	if err != nil {
		return pkgstorage.ObjectMetadata{}, mapStorageError(err)
	}

	checksum, err := decodeSHA256Checksum(output.ChecksumSHA256)
	if err != nil {
		return pkgstorage.ObjectMetadata{}, err
	}

	return pkgstorage.ObjectMetadata{
		SizeBytes:   aws.ToInt64(output.ContentLength),
		ContentType: strings.TrimSpace(aws.ToString(output.ContentType)),
		ETag:        strings.TrimSpace(aws.ToString(output.ETag)),
		Checksum:    checksum,
		VersionID:   strings.TrimSpace(aws.ToString(output.VersionId)),
	}, nil
}

func (a *Adapter) PutObject(ctx context.Context, ref pkgstorage.ObjectRef, contentType string, body io.Reader, size int64) error {
	if err := ref.Validate(); err != nil {
		return err
	}

	input := &s3.PutObjectInput{
		Bucket:      aws.String(ref.BucketName),
		Key:         aws.String(ref.ObjectKey),
		Body:        body,
		ContentType: aws.String(strings.TrimSpace(contentType)),
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}

	if _, err := a.client.PutObject(ctx, input); err != nil {
		return mapStorageError(err)
	}
	return nil
}

func (a *Adapter) CreateMultipartUpload(ctx context.Context, ref pkgstorage.ObjectRef, contentType string) (string, error) {
	if err := ref.Validate(); err != nil {
		return "", err
	}

	output, err := a.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(ref.BucketName),
		Key:         aws.String(ref.ObjectKey),
		ContentType: aws.String(strings.TrimSpace(contentType)),
	})
	if err != nil {
		return "", mapStorageError(err)
	}
	return strings.TrimSpace(aws.ToString(output.UploadId)), nil
}

func (a *Adapter) UploadPart(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string, partNumber int, body io.Reader, size int64) (string, error) {
	if err := ref.Validate(); err != nil {
		return "", err
	}

	input := &s3.UploadPartInput{
		Bucket:     aws.String(ref.BucketName),
		Key:        aws.String(ref.ObjectKey),
		UploadId:   aws.String(strings.TrimSpace(uploadID)),
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       body,
	}
	if size >= 0 {
		input.ContentLength = aws.Int64(size)
	}

	output, err := a.client.UploadPart(ctx, input)
	if err != nil {
		return "", mapStorageError(err)
	}
	return strings.TrimSpace(aws.ToString(output.ETag)), nil
}

func (a *Adapter) ListUploadedParts(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string) ([]pkgstorage.UploadedPart, error) {
	if err := ref.Validate(); err != nil {
		return nil, err
	}

	result := make([]pkgstorage.UploadedPart, 0)
	partMarker := ""
	for {
		output, err := a.client.ListParts(ctx, &s3.ListPartsInput{
			Bucket:           aws.String(ref.BucketName),
			Key:              aws.String(ref.ObjectKey),
			UploadId:         aws.String(strings.TrimSpace(uploadID)),
			PartNumberMarker: aws.String(partMarker),
		})
		if err != nil {
			return nil, mapStorageError(err)
		}

		for _, part := range output.Parts {
			checksum, err := decodeSHA256Checksum(part.ChecksumSHA256)
			if err != nil {
				return nil, err
			}
			result = append(result, pkgstorage.UploadedPart{
				PartNumber: int(aws.ToInt32(part.PartNumber)),
				ETag:       strings.TrimSpace(aws.ToString(part.ETag)),
				SizeBytes:  aws.ToInt64(part.Size),
				Checksum:   checksum,
			})
		}

		if !aws.ToBool(output.IsTruncated) {
			break
		}
		partMarker = strings.TrimSpace(aws.ToString(output.NextPartNumberMarker))
	}

	return result, nil
}

func (a *Adapter) CompleteMultipartUpload(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string, parts []pkgstorage.UploadedPart) error {
	if err := ref.Validate(); err != nil {
		return err
	}
	if len(parts) == 0 {
		return fmt.Errorf("%w: multipart upload has no parts", pkgstorage.ErrMultipartConflict)
	}

	completed := make([]s3types.CompletedPart, 0, len(parts))
	for _, part := range parts {
		completedPart := s3types.CompletedPart{
			PartNumber: aws.Int32(int32(part.PartNumber)),
		}
		if etag := strings.TrimSpace(part.ETag); etag != "" {
			completedPart.ETag = aws.String(etag)
		}
		if checksum, err := encodeSHA256Checksum(part.Checksum); err != nil {
			return err
		} else if checksum != "" {
			completedPart.ChecksumSHA256 = aws.String(checksum)
		}
		completed = append(completed, completedPart)
	}

	_, err := a.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(ref.BucketName),
		Key:      aws.String(ref.ObjectKey),
		UploadId: aws.String(strings.TrimSpace(uploadID)),
		MultipartUpload: &s3types.CompletedMultipartUpload{
			Parts: completed,
		},
	})
	if err != nil {
		return mapStorageError(err)
	}
	return nil
}

func (a *Adapter) AbortMultipartUpload(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string) error {
	if err := ref.Validate(); err != nil {
		return err
	}

	if _, err := a.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(ref.BucketName),
		Key:      aws.String(ref.ObjectKey),
		UploadId: aws.String(strings.TrimSpace(uploadID)),
	}); err != nil {
		return mapStorageError(err)
	}
	return nil
}

func (a *Adapter) PresignPutObject(ctx context.Context, ref pkgstorage.ObjectRef, contentType string, ttl time.Duration) (string, map[string]string, error) {
	if err := ref.Validate(); err != nil {
		return "", nil, err
	}

	request, err := a.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(ref.BucketName),
		Key:         aws.String(ref.ObjectKey),
		ContentType: aws.String(strings.TrimSpace(contentType)),
	}, func(options *s3.PresignOptions) {
		options.Expires = normalizePresignTTL(ttl)
	})
	if err != nil {
		return "", nil, mapStorageError(err)
	}

	return request.URL, cloneHeader(request.SignedHeader), nil
}

func (a *Adapter) PresignUploadPart(ctx context.Context, ref pkgstorage.ObjectRef, uploadID string, partNumber int, ttl time.Duration) (string, map[string]string, error) {
	if err := ref.Validate(); err != nil {
		return "", nil, err
	}

	request, err := a.presign.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(ref.BucketName),
		Key:        aws.String(ref.ObjectKey),
		UploadId:   aws.String(strings.TrimSpace(uploadID)),
		PartNumber: aws.Int32(int32(partNumber)),
	}, func(options *s3.PresignOptions) {
		options.Expires = normalizePresignTTL(ttl)
	})
	if err != nil {
		return "", nil, mapStorageError(err)
	}

	return request.URL, cloneHeader(request.SignedHeader), nil
}

// ComputeSHA256 用于在 provider 没返回对象校验值时，回退到流式计算对象哈希。
func (a *Adapter) ComputeSHA256(ctx context.Context, ref pkgstorage.ObjectRef) (string, error) {
	if err := ref.Validate(); err != nil {
		return "", err
	}

	output, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(ref.BucketName),
		Key:    aws.String(ref.ObjectKey),
	})
	if err != nil {
		return "", mapStorageError(err)
	}
	defer output.Body.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, output.Body); err != nil {
		return "", fmt.Errorf("%w: stream object for hashing: %v", pkgstorage.ErrProviderUnavailable, err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func normalizeAccessLevel(accessLevel pkgstorage.AccessLevel) pkgstorage.AccessLevel {
	if strings.TrimSpace(string(accessLevel)) == "" {
		return pkgstorage.AccessLevelPrivate
	}
	return pkgstorage.AccessLevel(strings.ToUpper(strings.TrimSpace(string(accessLevel))))
}

func normalizePresignTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultPresignTTL
	}
	return ttl
}

func detectProvider(cfg config.StorageConfig) pkgstorage.Provider {
	endpoint := strings.ToLower(strings.TrimSpace(cfg.Endpoint))
	if strings.Contains(endpoint, "minio") {
		return pkgstorage.ProviderMinIO
	}
	return pkgstorage.ProviderS3
}

func decodeSHA256Checksum(encoded *string) (string, error) {
	value := strings.TrimSpace(aws.ToString(encoded))
	if value == "" {
		return "", nil
	}

	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("%w: decode sha256 checksum: %v", pkgstorage.ErrProviderUnavailable, err)
	}
	return hex.EncodeToString(decoded), nil
}

func encodeSHA256Checksum(checksum string) (string, error) {
	checksum = strings.TrimSpace(checksum)
	if checksum == "" {
		return "", nil
	}

	decoded, err := hex.DecodeString(checksum)
	if err != nil {
		return "", fmt.Errorf("%w: encode sha256 checksum: %v", pkgstorage.ErrInvalidPresignRequest, err)
	}
	return base64.StdEncoding.EncodeToString(decoded), nil
}

func cloneHeader(header map[string][]string) map[string]string {
	if len(header) == 0 {
		return nil
	}

	result := make(map[string]string, len(header))
	for key, values := range header {
		if len(values) == 0 {
			continue
		}
		result[key] = values[0]
	}
	return result
}

func mapStorageError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return fmt.Errorf("%w: %v", pkgstorage.ErrObjectNotFound, err)
		case "NoSuchUpload":
			return fmt.Errorf("%w: %v", pkgstorage.ErrMultipartNotFound, err)
		case "InvalidPart", "InvalidPartOrder", "BadDigest":
			return fmt.Errorf("%w: %v", pkgstorage.ErrMultipartConflict, err)
		}
	}

	return fmt.Errorf("%w: %v", pkgstorage.ErrProviderUnavailable, err)
}

var (
	_ ports.BucketResolver     = (*Adapter)(nil)
	_ ports.ObjectReader       = (*Adapter)(nil)
	_ ports.InlineObjectWriter = (*Adapter)(nil)
	_ ports.MultipartManager   = (*Adapter)(nil)
	_ ports.PresignManager     = (*Adapter)(nil)
)

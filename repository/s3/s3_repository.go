package s3

import (
	"context"
	"github.com/minio/minio-go/v7"
	"mime/multipart"
	"strings"
	"time"
)

type S3Repository interface {
	// UploadFile menerima *multipart.FileHeader
	// return: URL file (public langsung / private presigned)
	UploadFile(ctx context.Context, fh *multipart.FileHeader, fileName string, filePath string, bucket string) (string, error)
	GetPresignedURL(ctx context.Context, bucket string, filename string, isPublic bool, expiry time.Duration) (string, error)
	GetDownloadURL(ctx context.Context, bucket, filename string, isPublic bool, expiry time.Duration) (string, error)
}

type s3Repository struct {
	client *minio.Client
	domain string // contoh: https://storeio.cloud.playcourt.id
}

// NewS3Repository will create an object that represent the S3Repository interface
func NewS3Repository(client *minio.Client, publicDomain string) S3Repository {
	return &s3Repository{
		client: client,
		domain: strings.TrimRight(publicDomain, "/"),
	}
}

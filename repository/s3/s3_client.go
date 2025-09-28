package s3

import (
	"context"
	"fmt"
	"mime"
	"mime/multipart"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type Client struct {
	Mc     *minio.Client
	Domain string // e.g. "https://storeio.cloud.playcourt.id"
}

func (r *s3Repository) UploadFile(
	ctx context.Context,
	fh *multipart.FileHeader,
	fileName string,
	filePath string,
	bucket string,
) (string, error) {

	if fh == nil {
		return "", fmt.Errorf("s3: file header is nil")
	}

	if err := r.ensureBucket(ctx, bucket); err != nil {
		return "", fmt.Errorf("s3: ensure bucket %w", err)
	}

	// bangun object key dari nama file asli
	key := buildObjectKey(fh.Filename, fileName)
	key = filePath + key

	// tentukan content-type
	ct := fh.Header.Get("Content-Type")
	if ct == "" {
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		ct = mime.TypeByExtension(ext)
		if ct == "" {
			ct = "application/octet-stream"
		}
	}

	// buka file dari header
	f, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("s3: open file %w", err)
	}
	defer f.Close()

	// upload
	_, err = r.client.PutObject(ctx, bucket, key, f, fh.Size, minio.PutObjectOptions{
		ContentType: ct,
	})
	if err != nil {
		return "", fmt.Errorf("s3: put object %w", err)
	}

	return key, nil
}

func (r *s3Repository) ensureBucket(ctx context.Context, bucket string) error {
	exists, err := r.client.BucketExists(ctx, bucket)
	if err != nil {
		return err
	}
	if !exists {
		if err := r.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func buildObjectKey(original, target string) string {
	ext := strings.ToLower(filepath.Ext(original))
	datePath := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("%s-%s-%s%s", datePath, target, uuid.NewString(), ext)
}

func (r *s3Repository) GetPresignedURL(
	ctx context.Context,
	bucket string,
	filename string, // object key lengkap
	isPublic bool,
	expiry time.Duration,
) (string, error) {

	if filename == "" {
		return "", nil
	}

	// Normalisasi key agar tidak ada leading slash
	key := strings.TrimLeft(filename, "/")

	if isPublic {
		return r.domain + "/" + bucket + "/" + key, nil
	}

	if expiry <= 0 {
		expiry = 24 * time.Hour
	}

	u, err := r.client.PresignedGetObject(ctx, bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("s3: presign get object %w", err)
	}
	return u.String(), nil
}

func (r *s3Repository) GetDownloadURL(
	ctx context.Context,
	bucket string,
	filename string, // object key lengkap, boleh mengandung path
	isPublic bool, // tidak dipakai untuk presign, tapi tetap disiapkan sebagai fallback
	expiry time.Duration,
) (string, error) {
	if filename == "" {
		return "", nil
	}
	key := strings.TrimLeft(filename, "/")
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}

	disposition := fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(key))

	// Build query override untuk memaksa download
	q := url.Values{}
	q.Set("response-content-disposition", disposition)
	q.Set("response-content-type", "application/octet-stream")

	// Selalu presign (lebih kompatibel di banyak provider)
	u, err := r.client.PresignedGetObject(ctx, bucket, key, expiry, q)
	if err == nil {
		return u.String(), nil
	}

	// Fallback terakhir: untuk objek public, kembalikan URL publik + query (bisa 502 di provider tertentu)
	if isPublic {
		pub, _ := url.Parse(r.domain + "/" + bucket + "/" + key)
		pub.RawQuery = q.Encode()
		return pub.String(), nil
	}

	return "", fmt.Errorf("s3: presign for download: %w", err)
}

package datastore

import (
	"fmt"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Options struct {
	Endpoint     string // "storeio.cloud.playcourt.id"
	AccessKey    string
	SecretKey    string
	UseSSL       bool   // true kalau https
	PublicDomain string // contoh: "https://storeio.cloud.playcourt.id" (boleh kosong)
}

func NewObjectStorageClient(opts S3Options) (*minio.Client, string, error) {
	mc, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, ""),
		Secure: opts.UseSSL,
	})
	if err != nil {
		return nil, "", err
	}

	pub := strings.TrimRight(opts.PublicDomain, "/")
	if pub == "" {
		scheme := "http"
		if opts.UseSSL {
			scheme = "https"
		}
		pub = fmt.Sprintf("%s://%s", scheme, opts.Endpoint)
	}
	return mc, pub, nil
}

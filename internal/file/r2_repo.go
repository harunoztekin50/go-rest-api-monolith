package file

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/config"
)

type r2Storage struct {
	bucketName string
	client     *s3.Client
	presigner  *s3.PresignClient
}

func NewR2Storage(ctx context.Context, cfg config.StorageConfig) (Storage, error) {
	if cfg.AccountID == "" {
		return nil, fmt.Errorf("storage account id boş olamaz")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("storage bucket boş olamaz")
	}
	if cfg.AccessKeyID == "" {
		return nil, fmt.Errorf("storage access key id boş olamaz")
	}
	if cfg.SecretAccessKey == "" {
		return nil, fmt.Errorf("storage secret access key boş olamaz")
	}

	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", cfg.AccountID)

	awsCfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				"",
			),
		),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &r2Storage{
		bucketName: cfg.Bucket,
		client:     client,
		presigner:  s3.NewPresignClient(client),
	}, nil
}

// UploadObject, nesneyi R2'ye yükler.
//
// ── Neden ContentLength zorunlu? ─────────────────────────────────────────────
// S3/R2 PutObject HTTP isteği bir Content-Length header'ı bekler.
// Bu olmadan bazı proxy'ler ve R2 gateway isteği reddedebilir.
// AWS SDK da ContentLength=0 gönderirse body boyutu ile uyuşmazlık oluşur.
//
// ── SizeBytes <= 0 durumu: fallback buffer stratejisi ────────────────────────
// Caller boyutu bilmiyorsa (streaming case) body'yi burada io.ReadAll ile
// tamamen okur, gerçek boyutu ölçer, ardından bytes.Reader ile yeniden sararız.
//
// Trade-off:
//
//	✓ Basit, güvenilir, ekstra bağımlılık yok
//	✗ Tüm dosya RAM'e alınır (10 MB limit ile bu kabul edilebilir)
//
// Alternatif (100 MB+ dosyalar için):
//
//	s3manager.Uploader kullanın — otomatik multipart upload yapar,
//	ContentLength gerektirmez, belleği chunk'lar halinde kullanır.
func (s *r2Storage) UploadObject(ctx context.Context, input UploadObjectInput) error {
	if input.ObjectKey == "" {
		return fmt.Errorf("object key boş olamaz")
	}
	if input.ContentType == "" {
		return fmt.Errorf("content type boş olamaz")
	}
	if input.Body == nil {
		return fmt.Errorf("body boş olamaz")
	}

	body := input.Body
	size := input.SizeBytes

	// Boyut bilinmiyorsa: önce oku, sonra gönder.
	if size <= 0 {
		data, err := io.ReadAll(input.Body)
		if err != nil {
			return fmt.Errorf("body okunamadı: %w", err)
		}
		size = int64(len(data))
		body = bytes.NewReader(data)
	}

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucketName),
		Key:           aws.String(input.ObjectKey),
		Body:          body,
		ContentType:   aws.String(input.ContentType),
		ContentLength: aws.Int64(size),
	})
	return err
}

func (s *r2Storage) GeneratePresignedDownloadURL(
	ctx context.Context,
	input PresignedDownloadInput,
) (string, error) {
	if input.ObjectKey == "" {
		return "", fmt.Errorf("object key boş olamaz")
	}
	if input.ExpiresInSec <= 0 {
		return "", fmt.Errorf("expiresInSec sıfırdan büyük olmalı")
	}

	req, err := s.presigner.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucketName),
			Key:    aws.String(input.ObjectKey),
		},
		func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(input.ExpiresInSec) * time.Second
		},
	)
	if err != nil {
		return "", err
	}

	return req.URL, nil
}

func (s *r2Storage) DeleteObject(ctx context.Context, objectKey string) error {
	if objectKey == "" {
		return fmt.Errorf("object key boş olamaz")
	}

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(objectKey),
	})
	return err
}

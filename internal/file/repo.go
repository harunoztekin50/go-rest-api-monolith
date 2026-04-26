package file

import (
	"context"
	"fmt"
	"time"

	dbx "github.com/go-ozzo/ozzo-dbx"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/entity"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/dbcontext"
)

type FileRepository interface {
	CreateFile(ctx context.Context, file entity.File) error
	GetFileByID(ctx context.Context, id string) (*entity.File, error)
	UpdateFileStatus(ctx context.Context, id string, status entity.FileStatus) error
	ListFilesByUserID(ctx context.Context, userID string) ([]entity.File, error) // YENİ

}

type repository struct {
	db *dbcontext.DB
}

func NewFileRepository(db *dbcontext.DB) FileRepository {
	return &repository{
		db: db,
	}
}

func (r *repository) CreateFile(ctx context.Context, file entity.File) error {
	result, err := r.db.DB().WithContext(ctx).NewQuery(`
		INSERT INTO files(
			id,
			user_id,
			bucket_name,
			object_key,
			original_file_name,
			mime_type,
			size_bytes,
			status,
			created_at,
			updated_at
		) VALUES (
			{:id},
			{:user_id},
			{:bucket_name},
			{:object_key},
			{:original_file_name},
			{:mime_type},
			{:size_bytes},
			{:status},
			{:created_at},
			{:updated_at}
		)
	`).Bind(dbx.Params{
		"id":                 file.ID,
		"user_id":            file.UserID,
		"bucket_name":        file.BucketName,
		"object_key":         file.ObjectKey,
		"original_file_name": file.OriginalFileName,
		"mime_type":          file.MimeType,
		"size_bytes":         file.SizeBytes,
		"status":             string(file.Status),
		"created_at":         file.CreatedAt,
		"updated_at":         file.UpdatedAt,
	}).Execute()
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("dosya kaydı oluşturulamadı")
	}

	return nil
}

func (r *repository) GetFileByID(ctx context.Context, id string) (*entity.File, error) {
	var file entity.File

	err := r.db.DB().WithContext(ctx).
		Select(
			"id",
			"user_id",
			"bucket_name",
			"object_key",
			"original_file_name",
			"mime_type",
			"size_bytes",
			"status",
			"created_at",
			"updated_at",
		).
		From("files").
		Where(dbx.HashExp{
			"id": id,
		}).
		One(&file)
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (r *repository) UpdateFileStatus(ctx context.Context, id string, status entity.FileStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("geçersiz file status")
	}

	_, err := r.db.DB().WithContext(ctx).
		NewQuery(`
			UPDATE files
			SET status = {:status},
				updated_at = {:updated_at}
			WHERE id = {:id}
		`).
		Bind(dbx.Params{
			"id":         id,
			"status":     string(status),
			"updated_at": time.Now(),
		}).
		Execute()

	return err
}

func (r *repository) ListFilesByUserID(ctx context.Context, userID string) ([]entity.File, error) {
	var files []entity.File

	err := r.db.DB().WithContext(ctx).
		Select(
			"id",
			"user_id",
			"bucket_name",
			"object_key",
			"original_file_name",
			"mime_type",
			"size_bytes",
			"status",
			"created_at",
			"updated_at",
		).
		From("files").
		Where(dbx.HashExp{
			"user_id": userID,
			"status":  string(entity.FileStatusUploaded), // silinmiş/pending dosyaları getirme
		}).
		OrderBy("created_at DESC").
		All(&files)

	if err != nil {
		return nil, fmt.Errorf("kullanıcı dosyaları sorgulanamadı: %w", err)
	}

	return files, nil
}

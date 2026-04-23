package entity

import (
	"errors"
	"time"
)

type FileStatus string

const (
	FileStatusPending  FileStatus = "pending"
	FileStatusUploaded FileStatus = "uploaded"
	FileStatusFailed   FileStatus = "failed"
	FileStatusDeleted  FileStatus = "deleted"
)

func (s FileStatus) IsValid() bool {
	switch s {
	case FileStatusPending,
		FileStatusUploaded,
		FileStatusFailed,
		FileStatusDeleted:
		return true
	default:
		return false
	}
}

type File struct {
	ID               string
	UserID           string
	BucketName       string
	ObjectKey        string
	OriginalFileName string
	MimeType         string
	SizeBytes        int64
	Status           FileStatus
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func NewFile(
	id string,
	userID string,
	bucketName string,
	objectKey string,
	originalFileName string,
	mimeType string,
	sizeBytes int64,
) (File, error) {
	if id == "" {
		return File{}, errors.New("id boş olamaz")
	}
	if userID == "" {
		return File{}, errors.New("user id boş olamaz")
	}
	if bucketName == "" {
		return File{}, errors.New("bucket name boş olamaz")
	}
	if objectKey == "" {
		return File{}, errors.New("object key boş olamaz")
	}
	if originalFileName == "" {
		return File{}, errors.New("original file name boş olamaz")
	}
	if mimeType == "" {
		return File{}, errors.New("mime type boş olamaz")
	}
	if sizeBytes < 0 {
		return File{}, errors.New("size bytes negatif olamaz")
	}

	simdi := time.Now()

	return File{
		ID:               id,
		UserID:           userID,
		BucketName:       bucketName,
		ObjectKey:        objectKey,
		OriginalFileName: originalFileName,
		MimeType:         mimeType,
		SizeBytes:        sizeBytes,
		Status:           FileStatusPending,
		CreatedAt:        simdi,
		UpdatedAt:        simdi,
	}, nil
}

func (f *File) MarkUploaded() {
	f.Status = FileStatusUploaded
	f.UpdatedAt = time.Now()
}

func (f *File) MarkFailed() {
	f.Status = FileStatusFailed
	f.UpdatedAt = time.Now()
}

func (f *File) MarkDeleted() {
	f.Status = FileStatusDeleted
	f.UpdatedAt = time.Now()
}

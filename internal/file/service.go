package file

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/entity"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
)

// ─────────────────────────────────────────────
// Domain Error Types
// ─────────────────────────────────────────────

type ErrUnsupportedMimeType struct{ Detected string }

func (e *ErrUnsupportedMimeType) Error() string {
	return fmt.Sprintf("unsupported file type: %s", e.Detected)
}

type ErrInvalidImageContent struct{}

func (e *ErrInvalidImageContent) Error() string { return "file content is not a valid image" }

type ErrInvalidInput struct {
	Field   string
	Message string
}

func (e *ErrInvalidInput) Error() string {
	return fmt.Sprintf("invalid input: field=%s message=%s", e.Field, e.Message)
}

// ─────────────────────────────────────────────
// Interfaces
// ─────────────────────────────────────────────

type FileValidator interface {
	Validate(header []byte, detectedMime string) error
}

type Storage interface {
	UploadObject(ctx context.Context, input UploadObjectInput) error
	DeleteObject(ctx context.Context, objectKey string) error
	GeneratePresignedDownloadURL(ctx context.Context, input PresignedDownloadInput) (string, error)
}

type UploadObjectInput struct {
	ObjectKey   string
	ContentType string
	Body        io.Reader
	// SizeBytes: dosyanın byte boyutu.
	// 0 veya negatif geçilirse storage katmanı body'yi önce okuyup boyutu ölçer.
	// Mümkünse gerçek boyutu geçmek tercih edilir — gereksiz RAM kullanımını önler.
	SizeBytes int64
}

type PresignedDownloadInput struct {
	ObjectKey    string
	ExpiresInSec int
}

// ─────────────────────────────────────────────
// Service
// ─────────────────────────────────────────────

type Service interface {
	UploadFile(ctx context.Context, input UploadFileInput) (*UploadFileResult, error)
	ListUserFiles(ctx context.Context, userID string) ([]FileListItem, error) // YENİ
}

// ─────────────────────────────────────────────
// UploadFile DTOs
// ─────────────────────────────────────────────

type UploadFileInput struct {
	UserID           string
	OriginalFileName string
	File             multipart.File
}

type UploadFileResult struct {
	FileID    string `json:"file_id"`
	ObjectKey string `json:"object_key"`
	ReadURL   string `json:"read_url,omitempty"`
}

// ─────────────────────────────────────────────
// ListUserFiles DTOs
// ─────────────────────────────────────────────

// FileListItem, kullanıcıya döndürülen dosya özeti.
//
// Neden entity.File değil?
//
//	entity iç domain modelidir; DB sütun isimleri ve iş kuralları içerir.
//	Bunu doğrudan dışarıya açarsak, entity değiştiğinde handler/client kırılır.
//	DTO bu sınırı çizer: service dışı hiçbir katman entity'nin iç yapısını bilmez.
//
// ReadURL neden omitempty?
//
//	Presign işlemi R2 SDK'sında lokal yapılır (ağ çağrısı yok), çok hızlıdır.
//	Yine de hata olasılığına karşı non-fatal tutulur: URL üretilemezse
//	alan boş gelir, liste yine de döner.
type FileListItem struct {
	FileID           string    `json:"file_id"`
	ObjectKey        string    `json:"object_key"`
	OriginalFileName string    `json:"original_file_name"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	Status           string    `json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	ReadURL          string    `json:"read_url,omitempty"`
}

// ─────────────────────────────────────────────
// Service Struct
// ─────────────────────────────────────────────

type service struct {
	repo      FileRepository
	storage   Storage
	logger    log.Logger
	validator FileValidator
	bucket    string
	keyPrefix string
}

func NewService(
	repo FileRepository,
	storage Storage,
	logger log.Logger,
	validator FileValidator,
	bucket string,
	keyPrefix string,
) Service {
	return &service{
		repo:      repo,
		storage:   storage,
		logger:    logger,
		validator: validator,
		bucket:    bucket,
		keyPrefix: cleanPrefix(keyPrefix),
	}
}

// ─────────────────────────────────────────────
// UploadFile
// ─────────────────────────────────────────────

func (s *service) UploadFile(ctx context.Context, input UploadFileInput) (*UploadFileResult, error) {
	// ── 1. Input validation ──────────────────────────────────────────────
	if err := validateUploadInput(input); err != nil {
		return nil, err
	}

	// ── 2. Magic bytes: gerçek MIME tespiti ─────────────────────────────
	// Client'ın Content-Type header'ına güvenmiyoruz.
	// http.DetectContentType dosyanın binary imzasına (magic number) bakar.
	headerBuf := make([]byte, 512)
	n, err := io.ReadFull(input.File, headerBuf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, fmt.Errorf("file read error: %w", err)
	}
	headerBuf = headerBuf[:n]

	detectedMime := http.DetectContentType(headerBuf)

	// ── 3. Validator ─────────────────────────────────────────────────────
	if err := s.validator.Validate(headerBuf, detectedMime); err != nil {
		return nil, err
	}

	// ── 4. Tüm dosyayı oku, gerçek boyutu say ───────────────────────────
	// Neden io.ReadAll burada?
	//   - S3 PutObject ContentLength zorunludur (streaming mode yok).
	//   - countingReader stream'i sayar ama storage'a SizeBytes geçmek için
	//     okuma bitmeden toplam bilinmez.
	//   - İki seçenek: (A) burada oku+say, (B) storage'da fallback io.ReadAll.
	//   - (A) seçimi: bir kez RAM'e alınır, storage'a doğru SizeBytes geçilir,
	//     storage'daki fallback çalışmaz → daha öngörülebilir ve verimli.
	//
	// Not: Dosya zaten MaxBytesReader ile 10 MB ile sınırlandırıldı (api.go).
	// Bu büyüklük RAM için kabul edilebilir.
	// 100 MB+ gereksinimi olursa s3manager.Uploader (multipart) kullanılmalı.
	fullReader := io.MultiReader(bytes.NewReader(headerBuf), input.File)
	counter := &countingReader{r: fullReader}

	data, err := io.ReadAll(counter)
	if err != nil {
		return nil, fmt.Errorf("file read error: %w", err)
	}
	actualSize := counter.total

	// ── 5. Object key ────────────────────────────────────────────────────
	fileID := entity.GenerateID()
	objectKey := s.buildObjectKey(input.UserID, fileID, input.OriginalFileName, detectedMime)

	// ── 6. Storage'a yükle ───────────────────────────────────────────────
	// bytes.NewReader ile body, SizeBytes ile gerçek boyut → storage'da fallback çalışmaz.
	if err := s.storage.UploadObject(ctx, UploadObjectInput{
		ObjectKey:   objectKey,
		ContentType: detectedMime,
		Body:        bytes.NewReader(data),
		SizeBytes:   actualSize,
	}); err != nil {
		s.logger.With(ctx).Errorf("storage upload failed: fileID=%s objectKey=%s err=%v", fileID, objectKey, err)
		return nil, fmt.Errorf("storage upload failed: %w", err)
	}

	// ── 7. Entity & DB ───────────────────────────────────────────────────
	fileEntity, err := entity.NewFile(
		fileID,
		input.UserID,
		s.bucket,
		objectKey,
		input.OriginalFileName,
		detectedMime,
		actualSize,
	)
	if err != nil {
		s.logger.With(ctx).Errorf("entity creation failed: fileID=%s err=%v", fileID, err)
		s.compensateDeleteObject(ctx, fileID, objectKey)
		return nil, err
	}

	fileEntity.MarkUploaded()

	if err := s.repo.CreateFile(ctx, fileEntity); err != nil {
		s.logger.With(ctx).Errorf("db insert failed: fileID=%s objectKey=%s err=%v", fileID, objectKey, err)
		s.compensateDeleteObject(ctx, fileID, objectKey)
		return nil, fmt.Errorf("db insert failed: %w", err)
	}

	// ── 8. Presigned URL (non-fatal) ─────────────────────────────────────
	readURL := s.tryGeneratePresignedURL(ctx, fileID, objectKey)

	return &UploadFileResult{
		FileID:    fileEntity.ID,
		ObjectKey: objectKey,
		ReadURL:   readURL,
	}, nil
}

// ─────────────────────────────────────────────
// ListUserFiles
// ─────────────────────────────────────────────

// ListUserFiles, kullanıcının R2'ye yüklediği tüm dosyaları
// metadata + presigned URL ile birlikte döndürür.
//
// Akış:
//  1. userID validate et
//  2. DB'den uploaded statüsündeki dosyaları çek
//  3. Her dosya için presigned URL üret (non-fatal)
//  4. []FileListItem dön
//
// Neden presign burada non-fatal?
//
//	Liste isteği, URL üretiminden bağımsız bir işlemdir.
//	R2 erişim sorunu olsa bile kullanıcı dosya listesini görebilmeli.
//	URL boş gelirse client "önizleme yok" gösterir, liste yine çalışır.
func (s *service) ListUserFiles(ctx context.Context, userID string) ([]FileListItem, error) {
	// ── 1. Input validation ──────────────────────────────────────────────
	if strings.TrimSpace(userID) == "" {
		return nil, &ErrInvalidInput{Field: "user_id", Message: "required"}
	}

	// ── 2. DB'den dosyaları çek ──────────────────────────────────────────
	files, err := s.repo.ListFilesByUserID(ctx, userID)
	if err != nil {
		s.logger.With(ctx).Errorf("ListFilesByUserID failed: userID=%s err=%v", userID, err)
		return nil, fmt.Errorf("dosyalar listelenemedi: %w", err)
	}

	// ── 3. Her dosya için presigned URL üret, DTO'ya dönüştür ───────────
	// make([]FileListItem, 0, len(files)):
	//   len=0 → boş başlıyor (append güvenli)
	//   cap=len(files) → kaç eleman geleceğini biliyoruz, Go slice'ı
	//   büyütmek için yeniden allocate etmez → gereksiz heap baskısı yok.
	result := make([]FileListItem, 0, len(files))
	for _, f := range files {
		result = append(result, FileListItem{
			FileID:           f.ID,
			ObjectKey:        f.ObjectKey,
			OriginalFileName: f.OriginalFileName,
			MimeType:         f.MimeType,
			SizeBytes:        f.SizeBytes,
			Status:           string(f.Status),
			CreatedAt:        f.CreatedAt,
			ReadURL:          s.tryGeneratePresignedURL(ctx, f.ID, f.ObjectKey),
		})
	}

	return result, nil
}

// ─────────────────────────────────────────────
// Private helpers
// ─────────────────────────────────────────────

func (s *service) compensateDeleteObject(ctx context.Context, fileID, objectKey string) {
	if err := s.storage.DeleteObject(ctx, objectKey); err != nil {
		s.logger.With(ctx).Errorf(
			"COMPENSATION FAILED — orphan object: fileID=%s objectKey=%s err=%v",
			fileID, objectKey, err,
		)
	}
}

func (s *service) tryGeneratePresignedURL(ctx context.Context, fileID, objectKey string) string {
	url, err := s.storage.GeneratePresignedDownloadURL(ctx, PresignedDownloadInput{
		ObjectKey:    objectKey,
		ExpiresInSec: 600,
	})
	if err != nil {
		s.logger.With(ctx).Errorf(
			"presigned URL generation failed (non-fatal): fileID=%s err=%v", fileID, err,
		)
		return ""
	}
	return url
}

func (s *service) buildObjectKey(userID, fileID, originalFileName, contentType string) string {
	ext := strings.ToLower(filepath.Ext(originalFileName))
	if ext == "" {
		ext = mimeTypeToExt(contentType)
	}
	if ext == "" {
		ext = ".bin"
	}
	if s.keyPrefix == "" {
		return fmt.Sprintf("users/%s/files/%s%s", userID, fileID, ext)
	}
	return fmt.Sprintf("%s/users/%s/files/%s%s", s.keyPrefix, userID, fileID, ext)
}

func validateUploadInput(input UploadFileInput) error {
	if input.UserID == "" {
		return &ErrInvalidInput{Field: "user_id", Message: "required"}
	}
	if input.OriginalFileName == "" {
		return &ErrInvalidInput{Field: "original_file_name", Message: "required"}
	}
	if input.File == nil {
		return &ErrInvalidInput{Field: "file", Message: "required"}
	}
	return nil
}

func cleanPrefix(prefix string) string {
	return strings.Trim(strings.TrimSpace(prefix), "/")
}

func mimeTypeToExt(contentType string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}

// countingReader, okunan gerçek byte sayısını biriktirir.
type countingReader struct {
	r     io.Reader
	total int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.total += int64(n)
	return n, err
}

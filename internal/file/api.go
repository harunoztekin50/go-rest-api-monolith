package file

import (
	"errors"
	"net/http"

	routing "github.com/go-ozzo/ozzo-routing/v2"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/auth"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
)

// ─────────────────────────────────────────────
// Sabitler
// ─────────────────────────────────────────────

const (
	// maxUploadSize: Kabul edilecek maksimum dosya boyutu.
	// http.MaxBytesReader ile body okumadan önce uygulanır.
	// Bu olmadan: 500 MB dosya gönderilirse RAM tükenir veya
	// TCP bağlantısı belirsiz şekilde kapanır → "empty reply from server".
	maxUploadSize = 10 << 20 // 10 MB

	// ramParseLimit: Multipart form parse edilirken RAM'de tutulacak max boyut.
	// Aşılan kısım geçici disk dosyasına yazılır.
	// maxUploadSize'ın yarısı makul bir değerdir.
	ramParseLimit = 5 << 20 // 5 MB
)

// ─────────────────────────────────────────────
// RegisterHandlers
//
// Route kayıt fonksiyonu. main.go veya wire/fx gibi DI
// araçlarından çağrılır. authHandler middleware olarak
// tüm route'lara uygulanır.
// ─────────────────────────────────────────────

func RegisterHandlers(
	rg *routing.RouteGroup,
	service Service,
	authHandler routing.Handler,
	logger log.Logger,
) {
	r := &resource{
		service: service,
		logger:  logger,
	}

	rg.Use(authHandler)
	rg.Post("/files/upload", r.uploadFile)
}

// ─────────────────────────────────────────────
// resource struct
//
// Handler'ların bağlı olduğu receiver.
// Service ve logger dışında bağımlılık tutmaz.
// HTTP state'i (request/response) burada değil, context'te taşınır.
// ─────────────────────────────────────────────

type resource struct {
	service Service
	logger  log.Logger
}

// ─────────────────────────────────────────────
// uploadFile Handler
//
// Sorumlulukları:
//   1. Kimlik doğrulama kontrolü
//   2. Body boyut sınırlama
//   3. Multipart form parse
//   4. Dosya alanını al
//   5. Service katmanına delege et
//   6. Hata tipine göre doğru HTTP status dön
//   7. Başarı yanıtı yaz
//
// Handler'ın YAPMADIĞI şeyler (servisin işi):
//   - MIME tespiti
//   - Dosya doğrulama
//   - Storage işlemleri
//   - DB işlemleri
// ─────────────────────────────────────────────

func (r *resource) uploadFile(c *routing.Context) error {
	// ── 1. Kimlik doğrulama ──────────────────────────────────────────────
	// auth.CurrentUser, JWT middleware'inin context'e koyduğu user'ı alır.
	// nil dönmesi: middleware atlandı veya token geçersizdi.
	currentUser := auth.CurrentUser(c.Request.Context())
	if currentUser == nil {
		return writeJSON(c, http.StatusUnauthorized, errorResponse{
			Error: "unauthorized",
		})
	}

	// ── 2. Body boyut sınırlama ──────────────────────────────────────────
	// Bu satır olmadan büyük dosyalar sunucuyu çökertebilir.
	// MaxBytesReader, okuma sırasında sınır aşılırsa *http.MaxBytesError döner.
	// Bağlantıyı temiz şekilde keser — "empty reply from server" olmaz.
	c.Request.Body = http.MaxBytesReader(c.Response, c.Request.Body, maxUploadSize)

	// ── 3. Multipart form parse ──────────────────────────────────────────
	// Go'nun net/http'si multipart body'yi otomatik parse etmez.
	// ParseMultipartForm çağrılmazsa FormFile belirsiz davranış gösterebilir.
	// ramParseLimit: bu boyuta kadar RAM'de tut, fazlasını diske yaz.
	if err := c.Request.ParseMultipartForm(ramParseLimit); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return writeJSON(c, http.StatusRequestEntityTooLarge, errorResponse{
				Error:   "file too large",
				Details: "maximum allowed size is 10MB",
			})
		}
		return writeJSON(c, http.StatusBadRequest, errorResponse{
			Error: "invalid multipart form",
		})
	}

	// ── 4. Dosyayı al ────────────────────────────────────────────────────
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return writeJSON(c, http.StatusBadRequest, errorResponse{
			Error:   "field 'file' is required",
			Details: "send file as multipart/form-data with field name 'file'",
		})
	}
	defer file.Close()

	// ── 5. Service katmanına delege et ───────────────────────────────────
	// Handler input'u toplar ve pakeder, iş mantığını ÇALIŞTIRMAZ.
	// ContentType burada YOK — servis kendisi magic bytes ile tespit eder.
	result, err := r.service.UploadFile(c.Request.Context(), UploadFileInput{
		UserID:           currentUser.GetID(),
		OriginalFileName: header.Filename,
		File:             file,
	})

	// ── 6. Hata tipine göre doğru HTTP status ────────────────────────────
	// errors.As: hata zincirinde (wrap edilmiş) doğru tipi arar.
	// Switch ile typed error → HTTP status eşleşmesi yapılır.
	//
	// Neden fmt.Errorf("...").Error() client'a gönderilmez?
	//   - İç hata mesajları DB adı, dosya yolu, stack trace içerebilir.
	//   - Bunlar güvenlik açığıdır (information disclosure).
	//   - Client sadece ne yapacağını bilmesi gereken mesajı alır.
	if err != nil {
		return r.handleServiceError(c, err)
	}

	// ── 7. Başarı yanıtı ─────────────────────────────────────────────────
	// 201 Created: Yeni kaynak oluşturulduğunda 200 değil 201 kullanılır.
	// RFC 7231: "The 201 response payload typically describes and links to
	// the resource(s) created."
	return writeJSON(c, http.StatusCreated, uploadResponse{
		FileID:    result.FileID,
		ObjectKey: result.ObjectKey,
		ReadURL:   result.ReadURL,
	})
}

// handleServiceError, service katmanından gelen typed error'ları
// doğru HTTP status koduna ve client-safe mesajlara dönüştürür.
//
// Bu fonksiyon handler'ın en kritik parçasıdır:
//   - Typed error → HTTP status eşleşmesi merkezi bir yerde.
//   - İç detaylar loglanır, client'a asla gönderilmez.
//   - Yeni hata tipi eklenirse sadece burası değişir.
func (r *resource) handleServiceError(c *routing.Context, err error) error {
	var (
		unsupportedMime *ErrUnsupportedMimeType
		invalidContent  *ErrInvalidImageContent
		invalidInput    *ErrInvalidInput
	)

	switch {
	case errors.As(err, &unsupportedMime):
		// 415: Client desteklenmeyen bir dosya türü gönderdi.
		return writeJSON(c, http.StatusUnsupportedMediaType, errorResponse{
			Error:   "unsupported file type",
			Details: "allowed types: image/jpeg, image/png, image/webp",
		})

	case errors.As(err, &invalidContent):
		// 422: MIME doğru ama dosya içeriği bozuk/geçersiz.
		return writeJSON(c, http.StatusUnprocessableEntity, errorResponse{
			Error: "invalid image content",
		})

	case errors.As(err, &invalidInput):
		// 400: Eksik veya hatalı input alanı.
		return writeJSON(c, http.StatusBadRequest, errorResponse{
			Error:   "invalid request",
			Details: err.Error(),
		})

	default:
		// 500: Beklenmeyen hata — logla ama detayı client'a verme.
		r.logger.With(c.Request.Context()).Errorf("uploadFile unexpected error: %v", err)
		return writeJSON(c, http.StatusInternalServerError, errorResponse{
			Error: "upload failed",
		})
	}
}

// ─────────────────────────────────────────────
// Response Types
//
// Neden ayrı struct? map[string]any yerine struct:
//   - Derleme zamanı tip güvenliği: alan adı yazım hatası derlenmez.
//   - Dokümantasyon: Response şeması kod içinde görünür.
//   - Swagger/OpenAPI üretimi için reflect edilebilir.
//   - Test'te karşılaştırma kolaylığı: struct == struct, map != map.
// ─────────────────────────────────────────────

type errorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

type uploadResponse struct {
	FileID    string `json:"file_id"`
	ObjectKey string `json:"object_key"`
	ReadURL   string `json:"read_url,omitempty"`
}

// writeJSON, ozzo-routing context'ine JSON yanıt yazar.
// Content-Type header'ını set eder ve status kodu uygular.
func writeJSON(c *routing.Context, status int, body any) error {
	c.Response.Header().Set("Content-Type", "application/json; charset=utf-8")
	return c.WriteWithStatus(body, status)
}

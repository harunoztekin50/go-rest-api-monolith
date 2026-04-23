package file

import (
	"bytes"
	"image"
	_ "image/jpeg" // JPEG decoder'ı register eder
	_ "image/png"  // PNG decoder'ı register eder
	"strings"
)

// ─────────────────────────────────────────────
// ImageValidator
//
// FileValidator interface'ini uygular.
// Sadece görsel dosyaları (JPEG, PNG, WebP) kabul eder.
//
// Neden ayrı bir dosya/struct?
//   - Tek Sorumluluk: Servisten bağımsız, sadece doğrulama yapar.
//   - Test edilebilir: Servis testinde mock kullanılır, bu ayrı test edilir.
//   - Genişletilebilir: PDF validator, video validator aynı interface'i uygular.
//   - allowed map inject edilebilir → runtime'da izin verilen tipler değişebilir.
// ─────────────────────────────────────────────

type ImageValidator struct {
	// allowed: izin verilen MIME type'lar. map lookup O(1).
	allowed map[string]bool
}

func NewImageValidator() *ImageValidator {
	return &ImageValidator{
		allowed: map[string]bool{
			"image/jpeg": true,
			"image/png":  true,
			"image/webp": true,
		},
	}
}

// Validate, magic bytes + MIME kontrolü yapar.
//
// İki katmanlı doğrulama:
//  1. MIME whitelist: Desteklenmeyen tipler reddedilir.
//  2. image.DecodeConfig: JPEG/PNG için gerçek decode denemesi.
//     Bu, uzantısı .png olan ama içeriği bozuk/sahte dosyaları yakalar.
//
// WebP için stdlib decode desteği yoktur (golang.org/x/image/webp gerekir).
// WebP sadece magic bytes kontrolüne dayanır — bu kabul edilebilir bir trade-off.
func (v *ImageValidator) Validate(header []byte, detectedMime string) error {
	normalized := strings.ToLower(strings.TrimSpace(detectedMime))

	if !v.allowed[normalized] {
		return &ErrUnsupportedMimeType{Detected: detectedMime}
	}

	// JPEG ve PNG için: image.DecodeConfig ile header decode'u dene.
	// Bu fonksiyon tam dosyayı decode etmez, sadece boyut/format için
	// header'a bakar. JPEG için ~20 byte, PNG için ~33 byte yeterli.
	// 512 byte ile güvenli margin bırakılmıştır.
	//
	// NOT: Bozuk bir JPEG header'ı 512 byte içinde ortaya çıkmayabilir.
	// Tam güvenlik için tam dosyayı decode etmek gerekir ama bu
	// büyük dosyalarda performans sorununa yol açar.
	// Bu bir bilinçli trade-off: %99 geçerli kullanım, %1 edge case.
	if normalized == "image/jpeg" || normalized == "image/png" {
		if _, _, err := image.DecodeConfig(bytes.NewReader(header)); err != nil {
			return &ErrInvalidImageContent{}
		}
	}

	return nil
}

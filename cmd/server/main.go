package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	dbx "github.com/go-ozzo/ozzo-dbx"
	routing "github.com/go-ozzo/ozzo-routing/v2"
	"github.com/go-ozzo/ozzo-routing/v2/content"
	"github.com/go-ozzo/ozzo-routing/v2/cors"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/album"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/auth"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/config"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/errors"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/file"
	"github.com/harunoztekin50/go-rest-api-monolith.git/internal/healthcheck"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/accesslog"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/dbcontext"
	"github.com/harunoztekin50/go-rest-api-monolith.git/pkg/log"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var Version = "1.0.0"

var flagConfig = flag.String("config", "./config/local.yml", "path to the config file")

func main() {
	flag.Parse()

	logger := log.New().With(nil, "version", Version)

	// .env yükleme hatası ölümcül değil — production'da env variable'lar
	// zaten inject edilmiş olur. Sadece uyar, çıkma.
	if err := godotenv.Load(".env"); err != nil {
		logger.Infof(".env file not found, relying on environment variables: %v", err)
	}

	cfg, err := config.Load(*flagConfig, logger)
	if err != nil {
		logger.Errorf("failed to load application configuration: %v", err)
		os.Exit(1)
	}

	db, err := dbx.MustOpen("postgres", cfg.DSN)
	if err != nil {
		logger.Errorf("failed to connect to database: %v", err)
		os.Exit(1)
	}
	db.QueryLogFunc = logDBQuery(logger)
	db.ExecLogFunc = logDBExec(logger)
	defer func() {
		if err := db.Close(); err != nil {
			logger.Errorf("failed to close database connection: %v", err)
		}
	}()

	// ── Storage başlatma main'de yapılır, buildHandler içinde değil ────────
	// Neden?
	//   - buildHandler bir constructor'dır; yan etkisi (network bağlantısı,
	//     panic) olmamalıdır.
	//   - Hata yönetimi burada açık ve kontrollüdür: panic yerine os.Exit.
	//   - Test sırasında buildHandler'a mock storage inject edilebilir.
	fileStorage, err := file.NewR2Storage(context.Background(), cfg.Storage)
	if err != nil {
		logger.Errorf("failed to initialize R2 storage: %v", err)
		os.Exit(1)
	}

	// ── WriteTimeout dosya upload için yetersiz ──────────────────────────
	// Orijinal kod: WriteTimeout: 10 * time.Second
	// 10 saniye küçük dosyalar için yeterli ama 10 MB dosya + yavaş
	// bağlantıda timeout yaşanır → "empty reply from server".
	//
	// Çözüm seçenekleri:
	//   A) WriteTimeout'u artır (basit ama tüm endpoint'leri etkiler)
	//   B) Upload endpoint'i için http.TimeoutHandler kullan (granüler)
	//   C) ReadTimeout'u artır — body okuma için kritik
	//
	// Burada upload gerçeği yansıtacak şekilde artırıyoruz.
	// Production'da reverse proxy (nginx/caddy) kendi timeout'larını uygular.
	address := fmt.Sprintf(":%v", cfg.ServerPort)
	hs := &http.Server{
		Addr:    address,
		Handler: buildHandler(logger, dbcontext.New(db), cfg, fileStorage),

		// ReadTimeout: İstemcinin tüm request body'sini göndermesi için süre.
		// 10 MB @ ~1 MB/s bağlantı = ~10 saniye. 30s güvenli margin.
		ReadTimeout: 30 * time.Second,

		// WriteTimeout: Response yazma süresi.
		// Presigned URL üretimi dahil. 30s yeterli.
		WriteTimeout: 30 * time.Second,

		// ReadHeaderTimeout: Sadece header okuma için ayrı limit.
		// Slowloris saldırısına karşı koruma.
		ReadHeaderTimeout: 5 * time.Second,

		IdleTimeout: 60 * time.Second,
	}

	go routing.GracefulShutdown(hs, 10*time.Second, logger.Infof)
	logger.Infof("server %v is running at %v", Version, address)

	if err := hs.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Errorf("server error: %v", err)
		os.Exit(1)
	}
}

// buildHandler, HTTP routing'i kurar ve handler'ı döner.
//
// Bu fonksiyon SAF olmalıdır:
//   - Yan etki (network bağlantısı, dosya okuma, panic) içermez.
//   - Tüm bağımlılıklar dışarıdan inject edilir.
//   - Test sırasında mock bağımlılıklarla çağrılabilir.
//
// fileStorage parametresi eklendi: önceki kodda buildHandler içinde
// storage başlatılıyordu ve hata olunca panic yapılıyordu. Bu yanlış.
func buildHandler(
	logger log.Logger,
	db *dbcontext.DB,
	cfg *config.Config,
	fileStorage file.Storage,
) http.Handler {
	router := routing.New()

	router.Use(
		accesslog.Handler(logger),
		errors.Handler(logger),
		content.TypeNegotiator(content.JSON),
		cors.Handler(cors.AllowAll),
	)

	healthcheck.RegisterHandlers(router, Version)

	rg := router.Group("/v1")
	authHandler := auth.Handler(cfg.JWTSigningKey)

	// ── Auth ─────────────────────────────────────────────────────────────
	authRepo := auth.NewsRepoAuth(db, logger)
	auth.RegisterHandlers(
		rg.Group(""),
		auth.NewService(cfg.JWTSigningKey, cfg.JWTExpiration, logger, authRepo),
		authHandler,
		logger,
	)

	// ── Album ────────────────────────────────────────────────────────────
	album.RegisterHandlers(
		rg.Group(""),
		album.NewService(album.NewRepository(db, logger), logger),
		authHandler,
		logger,
	)

	// ── File ─────────────────────────────────────────────────────────────
	// NewService artık FileValidator da alıyor.
	// ImageValidator: JPEG, PNG, WebP destekler.
	// Farklı validator inject ederek PDF, video desteği eklenebilir.
	fileService := file.NewService(
		file.NewFileRepository(db),
		fileStorage,
		logger,
		file.NewImageValidator(),
		cfg.Storage.Bucket,
		cfg.Storage.Prefix,
	)
	file.RegisterHandlers(rg.Group(""), fileService, authHandler, logger)

	return router
}

// logDBQuery, SQL sorgu loglaması için kullanılır.
func logDBQuery(logger log.Logger) dbx.QueryLogFunc {
	return func(ctx context.Context, t time.Duration, sql string, rows *sql.Rows, err error) {
		if err == nil {
			logger.With(ctx, "duration", t.Milliseconds(), "sql", sql).Info("DB query successful")
		} else {
			logger.With(ctx, "sql", sql).Errorf("DB query error: %v", err)
		}
	}
}

// logDBExec, SQL execution loglaması için kullanılır.
func logDBExec(logger log.Logger) dbx.ExecLogFunc {
	return func(ctx context.Context, t time.Duration, sql string, result sql.Result, err error) {
		if err == nil {
			logger.With(ctx, "duration", t.Milliseconds(), "sql", sql).Info("DB execution successful")
		} else {
			logger.With(ctx, "sql", sql).Errorf("DB execution error: %v", err)
		}
	}
}

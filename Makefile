MODULE = $(shell go list -m)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo "1.0.0")
PACKAGES := $(shell go list ./... | grep -v /vendor/)
LDFLAGS := -ldflags "-X main.Version=${VERSION}"

CONFIG_FILE ?= ./config/local.yml
APP_DSN ?= postgres://postgres:postgres@localhost:5432/go_restful?sslmode=disable

# host.docker.internal: migrate container'ından host'taki postgres'e ulaşmak için
MIGRATE := docker run --rm \
	-v $(shell pwd)/migrations:/migrations \
	migrate/migrate:v4.18.1 \
	-path=/migrations/ \
	-database "$(subst localhost,host.docker.internal,$(APP_DSN))"

.PHONY: default help
default: help

help: ## Komutlar hakkında yardım bilgisi gösterir
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# --- DATABASE İŞLEMLERİ ---

.PHONY: db-start
db-start: ## Veritabanını (Postgres) Docker üzerinde başlatır
	@docker rm -f postgres 2>/dev/null || true
	docker run -d --name postgres \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=go_restful \
		-p 5432:5432 \
		postgres
	@echo "Postgres hazır olana kadar bekleniyor..."
	@until docker exec postgres pg_isready -U postgres; do sleep 1; done
	@echo "Veritabanı hazır!"

.PHONY: db-stop
db-stop: ## Veritabanı konteynerini durdurur ve siler
	docker stop postgres && docker rm postgres

# --- MIGRATION İŞLEMLERİ ---

.PHONY: migrate
migrate: ## Tüm yeni migration'ları (up) çalıştırır
	@echo "Migration'lar uygulanıyor..."
	@$(MIGRATE) up

.PHONY: migrate-down
migrate-down: ## Son migration adımını geri alır (down 1)
	@echo "Son işlem geri alınıyor..."
	@$(MIGRATE) down 1

.PHONY: migrate-new
migrate-new: ## Yeni migration dosyası oluşturur — Kullanım: make migrate-new name=create_users_table
	@if [ -z "$(name)" ]; then \
		echo "HATA: 'name' parametresi gerekli!"; \
		echo "Kullanim: make migrate-new name=create_users_table"; \
		exit 1; \
	fi
	@$(MIGRATE) create -ext sql -dir migrations -digits 14 $(name)

.PHONY: migrate-reset
migrate-reset: ## Veritabanını tamamen sıfırlar ve migration'ları baştan çalıştırır
	@echo "Veritabanı sıfırlanıyor (drop)..."
	@$(MIGRATE) drop -f
	@echo "Migration'lar baştan yükleniyor..."
	@$(MIGRATE) up

.PHONY: testdata
testdata: migrate-reset ## Veritabanını sıfırlar ve test verilerini yükler
	@echo "Test verileri içeri aktarılıyor..."
	@docker exec -i postgres psql -U postgres -d go_restful < testdata/testdata.sql

# --- UYGULAMA VE DERLEME ---

.PHONY: run
run: ## Uygulamayı çalıştırır
	go run ${LDFLAGS} cmd/server/main.go

.PHONY: build
build: ## Uygulamayı binary olarak derler
	CGO_ENABLED=0 go build ${LDFLAGS} -a -o server $(MODULE)/cmd/server

.PHONY: fmt
fmt: ## Tüm paketleri formatlar
	@go fmt $(PACKAGES)

.PHONY: version
version: ## Uygulama versiyonunu gösterir
	@echo $(VERSION)
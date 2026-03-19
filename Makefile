MODULE = $(shell go list -m)
VERSION ?= $(shell git describe --tags --always --dirty --match=v* 2> /dev/null || echo "1.0.0")
PACKAGES := $(shell go list ./... | grep -v /vendor/)
LDFLAGS := -ldflags "-X main.Version=${VERSION}"

CONFIG_FILE ?= ./config/local.yml
# DSN'i config'den çekemezse diye güvenli bir varsayılan bırakıyoruz
APP_DSN ?= postgres://postgres:postgres@localhost:5432/go_restful?sslmode=disable

# Docker üzerindeki migrate aracını her yerde çalışacak şekilde optimize ettik
# localhost yerine host.docker.internal kullanarak konteyner-host iletişimini çözdük
MIGRATE := docker run --rm -v $(shell pwd)/migrations:/migrations migrate/migrate:v4.10.0 -path=/migrations/ -database "$(subst localhost,host.docker.internal,$(APP_DSN))"

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
	@echo "Veritabanı uyanıyor (5sn)..."
	@sleep 5

.PHONY: db-stop
db-stop: ## Veritabanı konteynerini durdurur ve siler
	docker stop postgres && docker rm postgres

# --- MIGRATION (GÖÇ) İŞLEMLERİ ---

.PHONY: migrate
migrate: ## Tüm yeni migration'ları (up) çalıştırır
	@echo "Migration'lar uygulanıyor..."
	@$(MIGRATE) up

.PHONY: migrate-down
migrate-down: ## Son migration adımını geri alır (down 1)
	@echo "Son işlem geri alınıyor..."
	@$(MIGRATE) down 1

.PHONY: migrate-new
migrate-new: ## Yeni bir migration dosyası oluşturur (Tarih formatı destekli)
	@if [ -z "$(name)" ]; then \
		read -p "Migration ismini girin (Örn: 20261903021200_user_table): " name_input; \
		name=$$name_input; \
	else \
		name=$(name); \
	fi; \
	$(MIGRATE) create -ext sql -dir migrations -digits 14 $$name
	
.PHONY: migrate-reset
migrate-reset: ## Veritabanını tamamen sıfırlar ve tüm migration'ları baştan çalıştırır
	@echo "Veritabanı sıfırlanıyor (Drop)..."
	@$(MIGRATE) drop -f
	@echo "Migration'lar baştan yükleniyor (Up)..."
	@$(MIGRATE) up

.PHONY: testdata
testdata: migrate-reset ## Veritabanını sıfırlar ve test verilerini (testdata.sql) yükler
	@echo "Test verileri içeri aktarılıyor..."
	@docker exec -i postgres psql -U postgres -d go_restful < testdata/testdata.sql

# --- UYGULAMA VE DERLEME ---

.PHONY: run
run: ## Uygulamayı (Server) çalıştırır
	go run ${LDFLAGS} cmd/server/main.go

.PHONY: build
build: ## Uygulamayı binary olarak derler
	CGO_ENABLED=0 go build ${LDFLAGS} -a -o server $(MODULE)/cmd/server

.PHONY: fmt
fmt: ## Tüm paketleri formatlar (go fmt)
	@go fmt $(PACKAGES)

.PHONY: version
version: ## Uygulama versiyonunu gösterir
	@echo $(VERSION)
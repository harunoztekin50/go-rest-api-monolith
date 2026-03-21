FROM golang:alpine AS build

RUN apk update && \
    apk add --no-cache \
        curl \
        git \
        bash \
        make \
        ca-certificates && \
    rm -rf /var/cache/apk/*

# migrate binary kurulumu (curl ile, güncel versiyon)
ARG MIGRATE_VERSION=4.18.1
RUN curl -L "https://github.com/golang-migrate/migrate/releases/download/v${MIGRATE_VERSION}/migrate.linux-amd64.tar.gz" \
    | tar -xzf - -C /usr/local/bin \
    && chmod +x /usr/local/bin/migrate

WORKDIR /app

COPY go.* ./
RUN go mod download && go mod verify

COPY . .
RUN make build


FROM alpine:latest

RUN apk --no-cache add ca-certificates bash
RUN mkdir -p /var/log/app

WORKDIR /app/

COPY --from=build /usr/local/bin/migrate /usr/local/bin/migrate
COPY --from=build /app/migrations ./migrations/
COPY --from=build /app/server .
COPY --from=build /app/cmd/server/entrypoint.sh .
# config sadece local build için — K8s'te ConfigMap/Secret kullan
COPY --from=build /app/config/*.yml ./config/

RUN chmod +x ./entrypoint.sh && ls -la

ENTRYPOINT ["./entrypoint.sh"]
FROM golang:1.26-bookworm AS builder
WORKDIR /src

RUN apt-get update \
    && apt-get install -y --no-install-recommends gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/audicatalogd ./cmd/audicatalogd \
    && CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o /out/audicatalog-worker ./cmd/audicatalog-worker

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl ffmpeg sqlite3 \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app

COPY --from=builder /out/audicatalogd /app/audicatalogd
COPY --from=builder /out/audicatalog-worker /app/audicatalog-worker

VOLUME ["/var/lib/audicatalog"]
EXPOSE 8080

CMD ["/app/audicatalogd"]

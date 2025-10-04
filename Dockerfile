FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS TARGETARCH TARGETVARIANT

RUN CGO_ENABLED=1 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w" -o fireflow ./cmd/server

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata sqlite-libs && \
    mkdir -p /app/configs

WORKDIR /app

# 设置生产环境变量
ENV ENV=production
ENV GIN_MODE=release

COPY --from=builder /app/fireflow .

EXPOSE 9686

CMD ["./fireflow"]
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

WORKDIR /app

# 安装必要的工具
RUN apk add --no-cache curl jq unzip

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 下载最新的前端 release 包
RUN echo "Downloading latest frontend release..." && \
    LATEST_RELEASE=$(curl -s https://api.github.com/repos/lm379/fireflow-frontend/releases/latest | jq -r '.tag_name') && \
    echo "Latest release: $LATEST_RELEASE" && \
    curl -L -o dist.zip "https://github.com/lm379/fireflow-frontend/releases/download/$LATEST_RELEASE/dist.zip" && \
    mkdir -p cmd/server/web && \
    unzip -o dist.zip -d cmd/server/web && \
    rm dist.zip && \
    echo "Frontend files extracted to cmd/server/web:" && \
    ls -la cmd/server/web/

ARG TARGETOS TARGETARCH TARGETVARIANT

RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -ldflags="-s -w" -o fireflow ./cmd/server

FROM alpine:latest

RUN apk add --no-cache ca-certificates tzdata && \
    mkdir -p /app/configs

WORKDIR /app

# 设置生产环境变量
ENV APP_MODE=production
ENV GIN_MODE=release

COPY --from=builder /app/fireflow .

EXPOSE 9686

CMD ["./fireflow"]
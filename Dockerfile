# Go 版本与 go.mod 中 go 指令对齐（1.25.x）；默认配置监听 8078
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o /ai-code-review ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /ai-code-review .

EXPOSE 8078
VOLUME ["/app/data"]

ENTRYPOINT ["./ai-code-review"]
CMD ["-config", "config.yaml"]

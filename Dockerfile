# LeapNode 后端 Dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# 复制 go.mod（go.sum 会自动生成）
COPY go.mod ./
RUN go mod download

# 复制源代码
COPY . .

# 编译
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o leapnode ./cmd/server

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# 复制编译好的二进制文件
COPY --from=builder /app/leapnode .
COPY --from=builder /app/config ./config
COPY --from=builder /app/migrations ./migrations

EXPOSE 8080

CMD ["./leapnode"]

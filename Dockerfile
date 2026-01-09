# 构建阶段
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
# RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN go mod tidy
RUN go build -ldflags="-s -w" -o demo-go-tiny .

# 制作镜像阶段
FROM alpine:3.19
ENV TZ=Asia/Shanghai
WORKDIR /app
RUN ln -sf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone && apk add --no-cache ca-certificates
COPY --from=builder /app/demo-go-tiny /app/demo-go-tiny
ENV PORT=8080
ENTRYPOINT ["/app/demo-go-tiny"]
CMD ["-c", "gin"]
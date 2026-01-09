# 构建阶段
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go mod tidy && go build -ldflags="-s -w" -o demo-go-tiny .

# 制作镜像阶段
FROM alpine:3.19
ENV TZ=Asia/Shanghai
RUN ln -sf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone && apk add --no-cache ca-certificates
COPY --from=builder /app/demo-go-tiny /demo-go-tiny
ENV PORT=8080
ENTRYPOINT ["/demo-go-tiny"]
CMD ["-c", "gin"]
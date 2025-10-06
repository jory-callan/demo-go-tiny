FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY main.go .
# RUN go mod init demo && go build -ldflags="-s -w" -o demo .
# 静态编译，完全无外部依赖，适合 scratch/alpine
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
      -ldflags="-s -w -extldflags=-static" -o demo .


FROM alpine:3.21
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.aliyun.com/g' /etc/apk/repositories
RUN apk add --no-cache ca-certificates tzdata \
 && ln -snf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
 && echo "Asia/Shanghai" > /etc/timezone
COPY --from=builder /app/demo /usr/local/bin/demo
ENV PORT=8080
EXPOSE 8080
ENTRYPOINT ["demo"]
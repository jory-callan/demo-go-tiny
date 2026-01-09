FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init demo && go build -ldflags="-s -w" -o demo .

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
COPY --from=builder /app/demo /demo
ENV PORT=8080
ENTRYPOINT ["/demo"]
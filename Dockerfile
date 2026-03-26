# Build Stage
FROM golang:1.22-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod ./
RUN go mod download && touch go.sum

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o opentreder ./cmd/cli

# Runtime Stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

RUN addgroup -g 1000 opentreder && \
    adduser -u 1000 -G opentreder -s /bin/sh -D opentreder

COPY --from=builder /app/opentreder .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/migrations ./migrations

RUN chown -R opentreder:opentreder /app

USER opentreder

EXPOSE 8080 8081 8082 9090

ENTRYPOINT ["./opentreder"]
CMD ["--config", "configs/config.yaml"]

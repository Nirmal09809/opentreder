# Build Stage
FROM alpine:3.19 AS builder

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 opentreder && \
    adduser -u 1000 -G opentreder -s /bin/sh -D opentreder

RUN echo "OpenTrader AI Trading Framework" > /app/README.txt

RUN chown -R opentreder:opentreder /app

USER opentreder

EXPOSE 8080 8081 8082 9090

CMD ["cat", "/app/README.txt"]

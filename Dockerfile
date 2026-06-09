FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

ARG BUILD_TARGET=app
RUN if [ "$BUILD_TARGET" = "processor" ]; then \
        go build -o /out/service ./cmd/processor; \
    else \
        go build -o /out/service .; \
    fi

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /out/service /service

EXPOSE 8080
CMD ["/service"]

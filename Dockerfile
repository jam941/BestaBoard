# syntax=docker/dockerfile:1

# ---- Build stage ----
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o vestad ./cmd/vestad

# ---- Runtime stage ----
FROM alpine:latest

# ca-certificates needed for outbound HTTPS to Vestaboard API
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app
COPY --from=builder /app/vestad .
COPY config.yaml .

EXPOSE 8080

ENTRYPOINT ["/app/vestad"]

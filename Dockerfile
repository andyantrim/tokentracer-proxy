# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o proxy .

# Run stage
FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/proxy .
COPY --from=builder /app/web ./web

EXPOSE 8080

CMD ["./proxy"]

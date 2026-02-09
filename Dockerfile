FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the server
RUN go build -o xcloud-server ./cmd/xcloud-server

FROM alpine:latest

WORKDIR /app

# Install certificates for HTTPS requests if needed
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/xcloud-server .

# Default port for Cloud Run
EXPOSE 8080

CMD ["./xcloud-server"]

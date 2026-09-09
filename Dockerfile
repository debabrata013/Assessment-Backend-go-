# Build stage
FROM golang:1.26 AS builder

WORKDIR /app

# Copy dependency files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build application
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o app .

# Runtime stage
FROM alpine:3.20

WORKDIR /app

# Install CA certificates
RUN apk --no-cache add ca-certificates

# Copy binary from builder
COPY --from=builder /app/app .

# Application port
EXPOSE 8080

# Start application
CMD ["./app"]

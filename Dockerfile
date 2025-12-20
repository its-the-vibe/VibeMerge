# Build stage
FROM golang:1.24-alpine AS builder

# Install CA certificates and git
RUN apk --no-cache add ca-certificates git

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY main.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o vibemerge .

# Runtime stage
FROM scratch

# Copy the binary from builder
COPY --from=builder /build/vibemerge /vibemerge

# Copy CA certificates for HTTPS requests to Slack API
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Run the application
ENTRYPOINT ["/vibemerge"]

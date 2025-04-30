# Stage 1: Builder Stage
FROM golang:1.24.1-alpine AS builder

# Set working directory
WORKDIR /app

# Copy Go module files and install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy all source code
COPY . .

# Build the Go binary
RUN go build -o codeexecutionengine ./cmd

# Stage 2: Final Stage (Minimal Image)
FROM alpine:latest

# Set working directory
WORKDIR /app

# Copy the built binary from the builder stage
COPY --from=builder /app/codeexecutionengine .

# Expose the necessary port
EXPOSE 50053

# Command to run the binary
CMD ["./codeexecutionengine"]

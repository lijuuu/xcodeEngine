# Stage 1: Build Stage (with Go tools)
FROM golang:1.24-alpine AS builder  

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod tidy

# Copy the rest of the application
COPY . .

# Build the Go application
RUN go build -o app .

# Stage 2: Final Stage (minimal runtime environment)
FROM alpine:latest  

WORKDIR /root/

# Copy the compiled Go binary from the builder stage
COPY --from=builder /app/app .

# Expose the port the app will run on
EXPOSE 8080

# Command to run the app
CMD ["./app"]

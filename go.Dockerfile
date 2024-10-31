FROM golang:1.22-alpine
WORKDIR /app

CMD ["sh", "-c", "echo \"$CODE\" > /tmp/main.go && go run /tmp/main.go"]

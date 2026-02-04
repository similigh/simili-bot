FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /simili ./cmd/simili/main.go

FROM alpine:latest

# Install git and ca-certificates (needed for git operations and https)
RUN apk add --no-cache ca-certificates git

COPY --from=builder /simili /bin/simili

ENTRYPOINT ["/bin/simili"]

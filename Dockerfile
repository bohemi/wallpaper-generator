# Stage 1: Build the binary
FROM golang:alpine AS builder

WORKDIR /app

# Install git
RUN apk add --no-cache git

# Copy dependency definitions
COPY go.mod go.sum ./

# Download dependencies with direct proxy fallback
ENV GOPROXY=https://proxy.golang.org,direct
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build statically compiled binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./main.go

# Stage 2: Minimal runtime image
FROM alpine:3.20

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

COPY --from=builder /app/server .

EXPOSE 8080

CMD ["./server"]
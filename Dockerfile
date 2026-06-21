# ====================================================================
# STAGE 1: Build the compiled binary using the Go SDK
# ====================================================================
FROM golang:1.25-alpine AS builder

# Set the active working directory inside the build sandbox
WORKDIR /app

# Copy dependency manifests first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your backend source code files
COPY . .

# Compile the Go application into a tight, statically-linked production binary
RUN CGO_ENABLED=0 GOOS=linux go build -o torogan-be ./cmd/server/main.go

# ====================================================================
# STAGE 2: Ship the server binary inside a tiny runtime image
# ====================================================================
FROM alpine:3.19

WORKDIR /root/

# Copy only the compiled binary file from the builder stage
COPY --from=builder /app/torogan-be .

# Expose the API port your server listens on (e.g., 8080 or ConnectRPC port)
EXPOSE 8080

# The execution target when the container starts up
CMD ["./torogan-be"]

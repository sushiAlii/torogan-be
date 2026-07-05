# ====================================================================
# Development Environment with Go SDK and Air Hot-Reload
# ====================================================================
FROM golang:1.26-alpine

# Set the active working directory inside the container
WORKDIR /app

# Install Air for hot-reloading
RUN go install github.com/air-verse/air@latest

# Copy dependency manifests first to leverage Docker layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of your backend source code files
COPY . .

# Expose the API port your server listens on
EXPOSE 8080

# Run Air to monitor changes and auto-compile
CMD ["air"]
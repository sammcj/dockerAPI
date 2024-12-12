# Start from the official Golang image
FROM golang:1.23-alpine AS builder

# Install git and Docker client
RUN apk add --no-cache git docker-cli

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dockerapi .

# Start a new stage from scratch
FROM alpine:latest

RUN apk --no-cache add ca-certificates docker-cli docker-compose

WORKDIR /root/

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/dockerapi .

# If using Docker via a socket, you can uncomment the following:
# VOLUME /var/run/docker.sock

# Add a volume for optional compose files, e.g:
VOLUME /mnt/docker-compose-files

# Expose the port the app runs on
EXPOSE 8080

# Command to run the executable
ENTRYPOINT ["./dockerapi"]

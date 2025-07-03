# Use official Golang image as the base
FROM golang:1.20 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go mod and sum files
COPY go.mod go.sum ./

# Download the dependencies
RUN go mod tidy

# Copy the rest of the application code
COPY . .

# Build the Go app
RUN go build -o main .

# Start a new stage from scratch with a smaller base image (Alpine)
FROM alpine:latest  

# Set the working directory in the new container
WORKDIR /root/

# Copy the pre-built binary from the builder stage
COPY --from=builder /app/main .

# Expose port 8080 to be accessible outside the container
EXPOSE 8080

# Command to run the executable
CMD ["./main"]

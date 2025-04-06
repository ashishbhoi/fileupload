# Use the official Golang image as the base image
FROM golang:1.24-alpine as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the source code into the container
COPY . .

# Build the Go application
RUN go build -o main .

# Use a minimal base image for the final container
FROM scratch

# Copy the compiled binary from the builder stage
COPY --from=builder /app/main /

# Copy the templates directory to the root of the container
COPY ./templates/ /templates/

# Set the working directory
WORKDIR /

# Expose the application port
EXPOSE 8080

# Command to run the application
CMD ["./main"]
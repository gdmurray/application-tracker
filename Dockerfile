# Use the official Golang image to create a build artifact.
FROM golang:1.22 as builder

# Copy local code to the container image.
WORKDIR /app
COPY . .

# Build the binary.
RUN CGO_ENABLED=0 GOOS=linux go build -v -o server

# Use a Docker multi-stage build to create a lean production image.
FROM gcr.io/distroless/base-debian10

WORKDIR /app
COPY --from=builder /app/server /app

# Run the web service on container startup.
CMD ["/app/server"]

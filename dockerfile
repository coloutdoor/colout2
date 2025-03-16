# Stage 1: Build the Go binary
FROM golang:1.24 AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o colout2

# Stage 2: Create a lean runtime image
FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/colout2 .
COPY static/ ./static/
COPY templates/ ./templates/
COPY images/ ./images/
COPY costs.yaml ./costs.yaml
RUN echo ":8080" > .env
EXPOSE 8080
CMD ["./colout2"]
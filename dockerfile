# Stage 1: Build the Go binary
# FROM golang:1.24 AS builder
FROM ubuntu:22.04
WORKDIR /app
RUN apt-get update && apt-get install -y gcc wget

## Get the latest Go binary 1.24.0
RUN wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
ENV PATH="/usr/local/go/bin:${PATH}"

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o colout2

# Stage 2: Create a lean runtime image
COPY static/ ./static/
COPY templates/ ./templates/
COPY images/ ./images/
# COPY costs.yaml ./costs.yaml
RUN echo ":8080" > .env
ENV DB_DIR=/db
RUN mkdir -p /db
EXPOSE 8080
CMD ["./colout2"]

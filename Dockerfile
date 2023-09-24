# syntax=docker/dockerfile:1
FROM golang:1.21 AS build-stage

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

#copy the file to build
COPY main.go ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /vault-plugin-sidecar

FROM build-stage AS run-test-stage
RUN go test -v ./...

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian12 AS build-release-stage
LABEL org.opencontainers.image.source=https://github.com/DarkMukke/vault-plugin-sidecar

WORKDIR /

COPY --from=build-stage /vault-plugin-sidecar /vault-plugin-sidecar

USER nonroot:nonroot

# Run
CMD ["/vault-plugin-sidecar"]
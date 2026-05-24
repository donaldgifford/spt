# Build stage
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

# Cache dependencies.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build.
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
  go build -trimpath -ldflags="-s -w" -o /spt ./cmd/spt

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /spt /spt

EXPOSE 8080 9090

USER nonroot:nonroot

ENTRYPOINT ["/spt"]

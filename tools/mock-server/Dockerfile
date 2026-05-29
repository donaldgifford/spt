# syntax=docker/dockerfile:1
#
# spt-mock-server: in-memory eBay-shaped HTTP mock for tests and local dev.
# See DESIGN-0006 "mock-server" and IMPL-0002 Phase 1.
#
# Multi-stage build lifted from prior art (donaldgifford/server-price-tracker)
# with version bumps to match the main spt binary.

FROM golang:1.26-alpine AS builder

WORKDIR /src

# Copy module files first so layer cache reuses module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source — embed.FS for fixtures means we need the
# tools/mock-server/fixtures tree alongside the .go files.
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH:-amd64} \
    go build -trimpath -ldflags="-s -w" \
    -o /spt-mock-server ./tools/mock-server

FROM alpine:3.21

RUN apk add --no-cache ca-certificates && \
    addgroup -S app && adduser -S app -G app

USER app
COPY --from=builder /spt-mock-server /usr/local/bin/spt-mock-server

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/spt-mock-server"]
CMD ["serve", "--port", "8080", "--scenario", "default"]

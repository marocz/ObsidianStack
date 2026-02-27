FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
      -ldflags="-s -w" \
      -o /obsidianstack-server \
      ./server/cmd/server

FROM gcr.io/distroless/static:nonroot
USER nonroot:nonroot
COPY --from=builder /obsidianstack-server /obsidianstack-server
ENTRYPOINT ["/obsidianstack-server"]

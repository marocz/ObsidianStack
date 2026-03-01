# Use the build host's native platform for the builder so Go cross-compiles
# to the target instead of running under QEMU (avoids runtime crashes on ARM hosts).
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder
ARG TARGETOS TARGETARCH
ENV GOTOOLCHAIN=local
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build \
      -ldflags="-s -w" \
      -o /obsidianstack-server \
      ./server/cmd/server

FROM gcr.io/distroless/static:nonroot
USER nonroot:nonroot
COPY --from=builder /obsidianstack-server /obsidianstack-server
ENTRYPOINT ["/obsidianstack-server"]

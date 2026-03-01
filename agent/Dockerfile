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
      -o /obsidianstack-agent \
      ./agent/cmd/agent

FROM gcr.io/distroless/static:nonroot
USER nonroot:nonroot
COPY --from=builder /obsidianstack-agent /obsidianstack-agent
ENTRYPOINT ["/obsidianstack-agent"]

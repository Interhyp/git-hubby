# Build the manager binary
<<<<<<< HEAD
FROM golang:1.26 AS builder
=======
FROM --platform=$BUILDPLATFORM golang:1.26 AS builder
>>>>>>> tmp-original-30-06-26-04-09
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder --chmod=0755 /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]


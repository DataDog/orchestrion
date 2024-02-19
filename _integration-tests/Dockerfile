FROM golang:alpine AS build

# Install a compiler toolchain (some tests use sqlite3 which CGO), and other
# useful utilities (needed by start.sh)
RUN apk add bash clang curl jq
ENV CGO_ENABLED=1 \
  CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

# Install delve (useful for debugging)
RUN CGO_ENABLED=0 go install -ldflags "-s -w -extldflags '-static'" github.com/go-delve/delve/cmd/dlv@latest

# Copy into the container instead of mounting, so that file changes are local.
COPY ./ /src/
WORKDIR /src

# Instrument the test applications with Orchestrion in `-w` mode
RUN go run . -w ./_integration-tests/tests/*
WORKDIR /src/_integration-tests
RUN go mod tidy

# Build all the test binaries at once into .bin
RUN mkdir .bin && go build -gcflags "all=-N -l" -o .bin ./tests/...

ENTRYPOINT ["/bin/bash", "/src/_integration-tests/start.sh"]

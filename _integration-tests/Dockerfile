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

# Build Orchestrion
RUN go build -o bin/ .
WORKDIR /src/_integration-tests

# Build all the test binaries at once into .bin
RUN mkdir .bin && GOFLAGS='-gcflags "all=-N -l"' ../bin/orchestrion go build -o .bin ./tests/...

ENTRYPOINT ["/bin/bash", "/src/_integration-tests/start.sh"]

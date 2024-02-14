FROM golang:alpine AS build

# Install a compiler toolchain (some tests use sqlite3 which CGO)
RUN apk add bash clang curl
ENV CGO_ENABLED=1 \
  CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

# Copy into the container instead of mounting, so that file changes are local.
COPY ./ /src/
WORKDIR /src

# Instrument the test applications with Orchestrion in `-w` mode
RUN go run . -next -w ./_integration-tests/tests/*
WORKDIR /src/_integration-tests
RUN go mod tidy

# Build all the test binaries at once into .bin
RUN mkdir .bin && go build -o .bin ./tests/...

ENTRYPOINT ["/bin/bash", "/src/_integration-tests/start.sh"]

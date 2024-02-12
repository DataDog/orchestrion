FROM golang:alpine AS build

# Install a compiler toolchain (some tests use sqlite3 which CGO)
RUN apk add clang
ENV CGO_ENABLED=1 \
  CGO_CFLAGS="-D_LARGEFILE64_SOURCE"

# Copy into the container instead of mounting, so that file changes are local.
COPY ./ /src/
WORKDIR /src

# Instrument the test applications with Orchestrion in `-w` mode
RUN go run . -w /src/_integration-tests/tests
WORKDIR /src/_integration-tests
RUN go mod tidy

# Build all the test binaries at once into .bin
RUN mkdir .bin && go build -o .bin ./tests/...

# Now actually run the test...
FROM golang:alpine
RUN apk add bash curl
COPY ./_integration-tests/start.sh /start.sh
ENTRYPOINT ["/bin/bash", "/start.sh"]
COPY --from=build /src/_integration-tests/.bin/ /.bin/

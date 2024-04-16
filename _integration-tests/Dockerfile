FROM golang:alpine AS build
RUN apk add clang git
COPY ./ /src/
WORKDIR /src
RUN git config --global --add safe.directory /src
RUN go build -v -o /usr/local/bin/orchestrion .

FROM golang:alpine
# Install a compiler toolchain (some tests use sqlite3 which CGO), and other
# useful utilities (needed by start.sh)
RUN apk add bash clang curl git jq
RUN git config --global --add safe.directory /src
ENV CGO_ENABLED=1 \
  CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
COPY --from=build /usr/local/bin/orchestrion /usr/local/bin/orchestrion

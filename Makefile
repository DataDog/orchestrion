.DEFAULT_GOAL := build

.PHONY: build generate tidy vet test clean fmt licenses verify-licenses verify-dd-headers integration-tests

build: generate test build-only

build-only:
	go build -o ./bin/orchestrion ./

build-linux-x64: generate test
	GOOS=linux GOARCH=amd64 go build -o ./bin/orchestrion ./

test: tidy fmt vet verify-licenses verify-dd-headers
	go test ./... -cover

generate:
	go generate ./...

tidy:
	go mod tidy

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -fv orchestrion
	go clean

licenses:
	tools/make-licenses.sh

verify-licenses:
	tools/verify-licenses.sh

verify-dd-headers:
	go run tools/headercheck/header_check.go

integration-tests:
	./integration-tests.sh

toolexec-tests:
	./internal/toolexec/tests/tests.sh

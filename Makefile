.DEFAULT_GOAL := build

.PHONY: build generate vet test clean fmt licenses verify-licenses

build: generate test build-only

build-only:
	go build -o ./bin/orchestrion ./

build-linux-x64: generate test
	GOOS=linux GOARCH=amd64 go build -o ./bin/orchestrion ./

test: generate 
	go test ./... -cover

lint: fmt vet verify-licenses verify-dd-headers

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -fv orchestrion
	go clean

licenses: bin/go-licenses
	tools/make-licenses.sh

verify-licenses: bin/go-licenses
	tools/verify-licenses.sh

bin/go-licenses:
	mkdir -p $(PWD)/bin
	GOBIN=$(PWD)/bin go install github.com/google/go-licenses@v1.6.0

verify-dd-headers:
	go run tools/headercheck/header_check.go

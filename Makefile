.DEFAULT_GOAL := build

.PHONY: build generate vet test clean fmt licenses verify-licenses

build: generate test build-only

build-only:
	go build -o ./bin/orchestrion ./

build-linux-x64: generate test
	GOOS=linux GOARCH=amd64 go build -o ./bin/orchestrion ./

test: generate fmt vet verify-licenses verify-dd-headers
	go test ./... -cover

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -fv orchestrion
	go clean

licenses: bin/go-licenses
	./bin/go-licenses report . --template ./tools/licenses.tpl > LICENSE-3rdparty.csv 2> errors

verify-licenses: bin/go-licenses
	tools/verify-licenses.sh

bin/go-licenses:
	mkdir -p $(PWD)/bin
	GOBIN=$(PWD)/bin go install github.com/google/go-licenses@v1.5.0

verify-dd-headers:
	go run tools/header_check.go

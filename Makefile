.DEFAULT_GOAL := build

.PHONY: build generate vet test clean fmt

build: generate test
	go build ./cmd/orchestrion

build-linux-x64: generate test
	GOOS=linux GOARCH=amd64 go build ./cmd/orchestrion

test: generate fmt vet
	go test ./... -cover

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	rm -fv orchestrion
	go clean

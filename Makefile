.PHONY: build test

build:
	go build -o alertherald ./cmd/alertherald

test:
	go test -v ./internal/...
.PHONY: build run test vet tidy install clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  = -s -w -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/drip ./cmd/drip

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/drip

install:
	go install -ldflags "$(LDFLAGS)" ./cmd/drip

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin

.PHONY: build run test vet tidy clean

build:
	go build -o bin/drip ./cmd/drip

run:
	go run ./cmd/drip

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin

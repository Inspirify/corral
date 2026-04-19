.PHONY: build test install clean

build:
	go build -o bin/corral ./cmd/corral

test:
	go test ./...

install:
	go install ./cmd/corral

clean:
	rm -rf bin/

lint:
	go vet ./...

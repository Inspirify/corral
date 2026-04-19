.PHONY: build test install clean lint fmt vulncheck check hooks

build:
	go build -o bin/corral ./cmd/corral

test:
	go test ./...

install:
	go install ./cmd/corral

clean:
	rm -rf bin/

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -d . | tee /dev/stderr | (! read)

vulncheck:
	govulncheck ./...

check: fmt lint vulncheck test
	@echo "All checks passed."

hooks:
	pre-commit install --install-hooks

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build clean test vet

build:
	go build -ldflags "-X main.version=$(VERSION)" -o xless .

clean:
	rm -f xless
	rm -rf .build/

test:
	go test ./...

vet:
	go vet ./...

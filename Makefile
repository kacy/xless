VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

.PHONY: build clean test vet smoke package-release

build:
	go build -ldflags "-X main.version=$(VERSION)" -o xless .

clean:
	rm -f xless
	rm -rf .build/

test:
	go test ./...

vet:
	go vet ./...

smoke:
	./scripts/smoke.sh

package-release:
	VERSION=$(VERSION) ./scripts/package-release.sh

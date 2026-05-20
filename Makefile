VERSION ?= $(shell ./version.sh)

.PHONY: build clean test check cache

build:
	go build -ldflags "-X main.version=$(VERSION)" -o revmap .
	go build -ldflags "-X main.version=$(VERSION)" -o cache-build ./cmd/cache-build

clean:
	rm -f revmap cache-build
	rm -rf cache/

test:
	go test -race ./...

check:
	./checks.sh

cache:
	go run ./cmd/cache-build

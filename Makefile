VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build clean test check cache

build:
	go build -ldflags "-X main.version=$(VERSION)" -o revmap .

clean:
	rm -f revmap
	rm -rf cache/

test:
	go test -race ./...

check:
	./checks.sh

cache: build
	./revmap cache-build

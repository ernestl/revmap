VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build clean test check

build:
	go build -ldflags "-X main.version=$(VERSION)" -o revmap .

clean:
	rm -f revmap

test:
	go test -race ./...

check:
	./checks.sh

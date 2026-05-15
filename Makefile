VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: build clean test check

build:
	go build -ldflags "-X main.version=$(VERSION)" -o snaprev .

clean:
	rm -f snaprev

test:
	go test -race ./...

check:
	./checks.sh

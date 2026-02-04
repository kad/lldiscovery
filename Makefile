.PHONY: build clean install test run fmt vet

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BINARY = lldiscovery
BUILD_FLAGS = -ldflags "-X main.version=$(VERSION)"

build:
	go build $(BUILD_FLAGS) -o $(BINARY) ./cmd/lldiscovery

clean:
	rm -f $(BINARY)
	rm -f *.dot *.png *.svg

install: build
	sudo install -m 755 $(BINARY) /usr/local/bin/$(BINARY)

test:
	go test -v ./...

run: build
	./$(BINARY) -log-level debug

fmt:
	go fmt ./...

vet:
	go vet ./...

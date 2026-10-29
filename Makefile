PREFIX ?= $(HOME)/.local
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT) -X main.BuildDate=$(BUILD_DATE)"

.PHONY: all build install test clean

all: build

build:
	go build $(LDFLAGS) -o bin/trinity ./cmd/trinity
	go build $(LDFLAGS) -o bin/dev-tui ./cmd/dev-tui

install: build
	mkdir -p $(PREFIX)/bin
	install -m 755 bin/trinity $(PREFIX)/bin/trinity
	install -m 755 bin/dev-tui $(PREFIX)/bin/dev-tui
	@echo "trinity and dev-tui installed to $(PREFIX)/bin"

test:
	go test -v ./...

clean:
	rm -rf bin/

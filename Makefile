PREFIX ?= $(HOME)/.local

.PHONY: all build install test clean

all: build

build:
	go build -o bin/dev-tui ./cmd/dev-tui

install: build
	mkdir -p $(PREFIX)/bin
	cp bin/dev-tui $(PREFIX)/bin/dev-tui
	@echo "dev-tui installed to $(PREFIX)/bin/dev-tui"

test:
	go test -v ./...

clean:
	rm -rf bin/

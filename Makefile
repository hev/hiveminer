.PHONY: build install clean help

BINARY := hiveminer

build:
	go build -o $(BINARY) ./cmd/hiveminer

install:
	go install ./cmd/hiveminer

clean:
	rm -f $(BINARY)

help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  install    - Install to GOPATH/bin"
	@echo "  clean      - Remove build artifacts"

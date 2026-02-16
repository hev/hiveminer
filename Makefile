.PHONY: build install clean help

BINARY := threadminer

build:
	go build -o $(BINARY) ./cmd/threadminer

install:
	go install ./cmd/threadminer

clean:
	rm -f $(BINARY)

help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  install    - Install to GOPATH/bin"
	@echo "  clean      - Remove build artifacts"

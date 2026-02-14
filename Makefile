.PHONY: build install clean help viewer

BINARY := threadminer

build:
	go build -o $(BINARY) ./cmd/threadminer

install:
	go install ./cmd/threadminer

clean:
	rm -f $(BINARY)

viewer:
	cd viewer && npm install && npm run build

viewer-dev:
	cd viewer && npm run dev

help:
	@echo "Available targets:"
	@echo "  build      - Build the binary"
	@echo "  install    - Install to GOPATH/bin"
	@echo "  clean      - Remove build artifacts"
	@echo "  viewer     - Build the Vue viewer"
	@echo "  viewer-dev - Start viewer in dev mode"

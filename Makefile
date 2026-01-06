VERSION ?= 1.0.0
BINARY_NAME = slack-cli
DIST_DIR = dist

.PHONY: all build clean release test

all: build

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v ./...

clean:
	rm -rf $(BINARY_NAME) $(DIST_DIR)

# Build for all platforms
release: clean
	mkdir -p $(DIST_DIR)

	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build -o $(DIST_DIR)/$(BINARY_NAME) .
	tar -czvf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_arm64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)
	rm $(DIST_DIR)/$(BINARY_NAME)

	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME) .
	tar -czvf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_darwin_amd64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)
	rm $(DIST_DIR)/$(BINARY_NAME)

	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build -o $(DIST_DIR)/$(BINARY_NAME) .
	tar -czvf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_arm64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)
	rm $(DIST_DIR)/$(BINARY_NAME)

	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build -o $(DIST_DIR)/$(BINARY_NAME) .
	tar -czvf $(DIST_DIR)/$(BINARY_NAME)_$(VERSION)_linux_amd64.tar.gz -C $(DIST_DIR) $(BINARY_NAME)
	rm $(DIST_DIR)/$(BINARY_NAME)

	@echo "Release archives created in $(DIST_DIR)/"
	@ls -la $(DIST_DIR)/

# Generate SHA256 checksums for Homebrew formula
checksums:
	@echo "SHA256 checksums for Homebrew formula:"
	@for f in $(DIST_DIR)/*.tar.gz; do \
		echo "$$(shasum -a 256 $$f | cut -d' ' -f1)  $$(basename $$f)"; \
	done

install: build
	install -m 755 $(BINARY_NAME) /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

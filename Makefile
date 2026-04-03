.PHONY: build test test-short test-cover lint clean run-server run-client devices install vet fmt check docker-build docker-run docker-push build-arm build-arm64

BINARY_NAME=EchoWarp
ifeq ($(OS),Windows_NT)
  BINARY_EXT=.exe
endif
BINARY=$(BINARY_NAME)$(BINARY_EXT)
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || grep 'Version =' internal/version/version.go | sed 's/.*"\(.*\)"/\1/')
LDFLAGS=-ldflags "-s -w -X github.com/lHumaNl/echowarp/internal/version.Version=$(VERSION)"
BUILD_TAGS=-tags nolibopusfile

# Static opus linkage: create temp dir with only .a file so linker can't find .dylib/.so
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
  OPUS_STATIC_DIR := /tmp/opus_static_build
  OPUS_PREFIX := $(shell brew --prefix opus 2>/dev/null)
  OPUS_A := $(OPUS_PREFIX)/lib/libopus.a
  CGO_EXTRA_LDFLAGS := -L$(OPUS_STATIC_DIR)
  CGO_EXTRA_CFLAGS := -I$(OPUS_PREFIX)/include
else ifeq ($(UNAME_S),Linux)
  OPUS_STATIC_DIR := /tmp/opus_static_build
  OPUS_A := $(shell pkg-config --variable=libdir opus 2>/dev/null)/libopus.a
  CGO_EXTRA_LDFLAGS := -L$(OPUS_STATIC_DIR)
else ifneq (,$(findstring MINGW,$(UNAME_S))$(findstring MSYS,$(UNAME_S)))
  OPUS_STATIC_DIR := $(CURDIR)/.opus_static
  OPUS_A := $(shell pkg-config --variable=libdir opus)/libopus.a
  CGO_EXTRA_LDFLAGS := -L$(OPUS_STATIC_DIR) -static
  CGO_EXTRA_CFLAGS := $(shell pkg-config --cflags opus)
endif

define check_opus
	@if ! pkg-config --exists opus 2>/dev/null; then \
		echo ""; \
		echo "ERROR: libopus development package not found."; \
		echo ""; \
		echo "  macOS:   brew install opus pkg-config"; \
		echo "  Ubuntu:  sudo apt-get install libopus-dev pkg-config"; \
		echo "  Fedora:  sudo dnf install opus-devel pkgconfig"; \
		echo ""; \
		echo "libopus is required to compile EchoWarp from source."; \
		echo "Pre-built binaries from GitHub Releases do NOT require opus."; \
		echo ""; \
		exit 1; \
	fi
endef

define setup_static_opus
	@mkdir -p $(OPUS_STATIC_DIR) && cp $(OPUS_A) $(OPUS_STATIC_DIR)/ 2>/dev/null || true
endef

SYSO_FILE := cmd/echowarp/rsrc_windows_amd64.syso
define embed_windows_icon
	@if [ ! -f $(SYSO_FILE) ]; then \
		echo "Embedding Windows icon..."; \
		go install github.com/akavel/rsrc@latest && \
		"$$(go env GOPATH)/bin/rsrc" -ico build/icons/icon.ico -o $(SYSO_FILE); \
	fi
endef

build:
	$(call check_opus)
ifeq ($(OS),Windows_NT)
	$(call embed_windows_icon)
endif
ifdef OPUS_STATIC_DIR
	$(call setup_static_opus)
	CGO_LDFLAGS="$(CGO_EXTRA_LDFLAGS)" CGO_CFLAGS="$(CGO_EXTRA_CFLAGS)" go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY) ./cmd/echowarp/
else
	go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY) ./cmd/echowarp/
endif

test:
	go test $(BUILD_TAGS) -race -v ./...

test-short:
	go test $(BUILD_TAGS) -race -v -short ./...

test-cover:
	go test $(BUILD_TAGS) -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run --build-tags nolibopusfile ./...

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe coverage.out coverage.html $(SYSO_FILE)
	rm -rf /tmp/opus_static_build .opus_static

run-server:
	go run $(BUILD_TAGS) ./cmd/echowarp/ server $(ARGS)

run-client:
	go run $(BUILD_TAGS) ./cmd/echowarp/ client $(ARGS)

devices:
	go run $(BUILD_TAGS) ./cmd/echowarp/ devices

install:
ifdef OPUS_STATIC_DIR
	$(call setup_static_opus)
	CGO_LDFLAGS="$(CGO_EXTRA_LDFLAGS)" CGO_CFLAGS="$(CGO_EXTRA_CFLAGS)" go install $(BUILD_TAGS) $(LDFLAGS) ./cmd/echowarp/
else
	go install $(BUILD_TAGS) $(LDFLAGS) ./cmd/echowarp/
endif

vet:
	go vet $(BUILD_TAGS) ./...

fmt:
	gofmt -s -w .

check: fmt vet lint test-short

docker-build:
	docker build -t echowarp:latest .

docker-run:
	docker run -it --rm -p 4415:4415 -p 8080:8080 echowarp:latest

docker-push:
	docker tag echowarp:latest $(REGISTRY)/echowarp:latest
	docker push $(REGISTRY)/echowarp:latest

build-arm:
ifdef OPUS_STATIC_DIR
	$(call setup_static_opus)
	CGO_LDFLAGS="$(CGO_EXTRA_LDFLAGS)" GOOS=linux GOARCH=arm GOARM=6 go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY_NAME)-linux-armv6 ./cmd/echowarp/
	CGO_LDFLAGS="$(CGO_EXTRA_LDFLAGS)" GOOS=linux GOARCH=arm GOARM=7 go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY_NAME)-linux-armv7 ./cmd/echowarp/
else
	GOOS=linux GOARCH=arm GOARM=6 go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY_NAME)-linux-armv6 ./cmd/echowarp/
	GOOS=linux GOARCH=arm GOARM=7 go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY_NAME)-linux-armv7 ./cmd/echowarp/
endif

build-arm64:
ifdef OPUS_STATIC_DIR
	$(call setup_static_opus)
	CGO_LDFLAGS="$(CGO_EXTRA_LDFLAGS)" GOOS=linux GOARCH=arm64 go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/echowarp/
else
	GOOS=linux GOARCH=arm64 go build $(BUILD_TAGS) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 ./cmd/echowarp/
endif

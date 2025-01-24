# Default values for environment variables
# linux, windows, darwin
PLATFORM ?=
# amd64, arm64, arm, mips, mips64
ARCH ?=
# Docker image to use for building (docker.elastic.co/beats-dev/golang-crossbuild:1.22.4-mips-debian11)
IMAGE =

BIN_NAME = lantern-headless-$(ARCH)-$(PLATFORM)

REVISION_DATE := $(shell git log -1 --pretty=format:%ad --date=format:%Y%m%d.%H%M%S)
BUILD_DATE := $(shell date -u +%Y%m%d.%H%M%S)
UID := $(shell id -u)
GID := $(shell id -g)
LATEST_TAG = $(shell git describe --tags $(shell git rev-list --tags --max-count=1))
VERSION ?= $(subst v,,$(LATEST_TAG))

LDFLAGS := -s -w -X github.com/getlantern/lantern-headless-client/shared.RevisionDate=$(REVISION_DATE) -X github.com/getlantern/lantern-headless-client/shared.BuildDate=$(BUILD_DATE) -X github.com/getlantern/lantern-headless-client/shared.ApplicationVersion=$(VERSION)

# Check if required environment variables are set
check-env:
	@mkdir -p build
	@if [ -z "$(PLATFORM)" ]; then \
		echo "Error: PLATFORM is not set"; \
		exit 1; \
	fi
	@if [ -z "$(IMAGE)" ]; then \
		echo "Error: IMAGE is not set"; \
		exit 1; \
	fi
	@if [ -z "$(ARCH)" ]; then \
		echo "Error: ARCH is not set"; \
		exit 1; \
	fi


# Actually Build the application (called inside the build container)
build-internal: export GOPRIVATE = github.com/getlantern
build-internal: export CGO_ENABLED=1
build-internal: check-env
	@apt update
	@apt install git-lfs
	@git lfs install
	go build -ldflags="$(LDFLAGS)" -buildvcs=false -o ./build/$(BIN_NAME) .
	chown $(TARGET_UID):$(TARGET_GID) ./build/$(BIN_NAME)

clean:
	@echo "Cleaning up"
	rm -f build/*

# Build the application using build container
build: check-env
	@echo "Building for $(PLATFORM)/$(ARCH) version $(VERSION) using $(IMAGE)"
	docker run -v .:/src -w /src -e TARGET_UID=$(UID) -e TARGET_GID=$(GID) \
        $(IMAGE) \
        --build-cmd "make VERSION=$(VERSION) PLATFORM=$(PLATFORM) ARCH=$(ARCH) IMAGE=$(IMAGE) build-internal" \
        -p $(PLATFORM)/$(ARCH)


# Show help
help:
	@echo "Available targets:"
	@echo "  build    - Build the application (requires PLATFORM, ARCH, IMAGE and VERSION)"
	@echo "  clean    - Remove build/ contents"
	@echo "  help     - Show this help message"
	@echo ""
	@echo "Required environment variables:"
	@echo "  PLATFORM - Target platform"
	@echo "  ARCH     - Target architecture"
	@echo "  VERSION  - Build version"

.PHONY: build clean help check-env package

# Default target
.DEFAULT_GOAL := help

# Default values for environment variables
# linux, windows, darwin
PLATFORM ?=
# amd64, arm64, arm, mips, mips64
ARCH ?=
VERSION ?=
# Docker image to use for building (docker.elastic.co/beats-dev/golang-crossbuild:1.22.4-mips-debian11)
IMAGE =

BIN_NAME = "lantern-headless-$(ARCH)-$(PLATFORM)"

REVISION_DATE := $(shell git log -1 --pretty=format:%ad --date=format:%Y%m%d.%H%M%S)
BUILD_DATE := $(shell date -u +%Y%m%d.%H%M%S)
UID := $(shell id -u)
GID := $(shell id -g)

LDFLAGS := -s -w -X github.com/getlantern/lantern-headless-client/main.RevisionDate=$(REVISION_DATE) -X github.com/getlantern/lantern-headless-client/main.BuildDate=$(BUILD_DATE) -X github.com/getlantern/lantern-headless-client/main.CompileTimePackageVersion=$(VERSION)

# Ensure we have nfpm installed. If not install using go install
nfpm:
	@command -v nfpm >/dev/null 2>&1 || { \
		echo "nfpm is not installed. Installing..."; \
		go install github.com/goreleaser/nfpm/v2/cmd/nfpm@latest; \
	}

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
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION is not set"; \
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
	docker run -it --rm -v .:/src -w /src -e TARGET_UID=$(UID) -e TARGET_GID=$(GID) \
        $(IMAGE) \
        --build-cmd "make VERSION=$(VERSION) PLATFORM=$(PLATFORM) ARCH=$(ARCH) IMAGE=$(IMAGE) build-internal" \
        -p $(PLATFORM)/$(ARCH)


# Show help
help:
	@echo "Available targets:"
	@echo "  package  - Build packages for all supported platforms"
	@echo "  build    - Build the application (requires PLATFORM, ARCH, IMAGE and VERSION)"
	@echo "  clean    - Remove built docker images"
	@echo "  help     - Show this help message"
	@echo ""
	@echo "Environment variables:"
	@echo "  PLATFORM - Target platform (default: linux)"
	@echo "  ARCH     - Target architecture (default: amd64)"
	@echo "  VERSION  - Build version (default: latest)"

.PHONY: build clean help check-env package

# Default target
.DEFAULT_GOAL := help

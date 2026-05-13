BINARY := mgm
PKG    := github.com/MGM-Laboratory/mgm-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
  -X $(PKG)/internal/version.Version=$(VERSION) \
  -X $(PKG)/internal/version.Commit=$(COMMIT) \
  -X $(PKG)/internal/version.Date=$(DATE)

PLATFORMS := \
  linux/amd64 \
  linux/arm64 \
  darwin/amd64 \
  darwin/arm64 \
  windows/amd64

.PHONY: all build install test tidy fmt vet clean dist snapshot release

all: build

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/mgm

install: build
	install -m 0755 bin/$(BINARY) $(DESTDIR)/usr/local/bin/$(BINARY)

test:
	go test ./...

tidy:
	go mod tidy

fmt:
	gofmt -s -w .

vet:
	go vet ./...

clean:
	rm -rf bin dist

# Cross-compile every platform into ./dist/. No goreleaser required.
dist: clean
	@mkdir -p dist
	@for plat in $(PLATFORMS); do \
	  os=$${plat%/*}; arch=$${plat#*/}; \
	  ext=""; if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
	  out="dist/$(BINARY)-$$os-$$arch$$ext"; \
	  echo "==> $$out"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	    go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" ./cmd/mgm || exit 1; \
	done
	@cd dist && sha256sum $(BINARY)-* > checksums.txt 2>/dev/null || shasum -a 256 $(BINARY)-* > checksums.txt
	@ls -1 dist

snapshot:
	goreleaser release --snapshot --clean

release:
	goreleaser release --clean

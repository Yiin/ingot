# VERSION is embedded into release builds via -ldflags -X below.
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS_RELEASE := -s -w -X github.com/Yiin/ingot/internal/buildinfo.version=$(VERSION)

.PHONY: build run test test-integration vet fmt lint check release clean screenshot

# No -trimpath here, unlike release: every distinct combination of build
# flags forces a full cold rebuild of gotk4's ~160k lines of generated cgo
# (4-10 minutes; go does not hash C headers, golang/go#24355). Keep these
# flags identical across build/vet/test/lint so only `release` ever pays
# that cost.
build:
	go build -o bin/ingot ./cmd/ingot

run:
	go run ./cmd/ingot $(ARGS)

test:
	go test ./...

# Display- and Wayland-dependent tests are gated behind the integration
# build tag and need a real (or headless) compositor to do anything but
# skip themselves, so this runs them inside scripts/headless.sh's sway
# session rather than the bare `go test`.
test-integration:
	./scripts/headless.sh go test -tags integration ./...

# Regenerates assets/screenshot.png from the same map-and-capture
# machinery internal/layershell's screenshot test uses, so the README
# image is reproducible instead of hand-made.
screenshot:
	INGOT_SCREENSHOT_OUT=$(CURDIR)/assets/screenshot.png \
		./scripts/headless.sh go test -tags integration -count=1 -run TestScreenshot_MapsAndCapturesANonUniformImage -v ./internal/layershell/...

# Plain `go vet` fails on internal/layershell: the gotk4 idiom for getting a
# C pointer from a widget converts Native()'s uintptr to unsafe.Pointer,
# which vet reports as "possible misuse of unsafe.Pointer". This is
# mirrored in .golangci.yml (govet.disable: [unsafeptr]).
vet:
	go vet -unsafeptr=false ./...

fmt:
	gofmt -l .

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "lint: golangci-lint not installed, skipping"; \
	fi

check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:" && gofmt -l . && exit 1)
	$(MAKE) vet
	$(MAKE) build
	$(MAKE) test
	$(MAKE) lint

# -trimpath and stripped/version-stamped ldflags are release-only: they
# change the build flags from the dev loop above, which is exactly what
# forces the expensive cold cgo rebuild. No separate strip step needed —
# -s -w already strips symbols and DWARF.
release:
	go build -trimpath -ldflags "$(LDFLAGS_RELEASE)" -o dist/ingot ./cmd/ingot

clean:
	rm -rf bin dist

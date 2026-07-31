# VERSION is embedded into release builds via -ldflags -X below.
VERSION ?= $(shell git describe --tags --always --dirty)
LDFLAGS_RELEASE := -s -w -X github.com/Yiin/ingot/internal/buildinfo.version=$(VERSION)

.PHONY: build run test test-integration vet fmt lint check release clean screenshot bench

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

# BenchmarkSearch_3000Notes asserts its own 2ms budget via b.Fatalf, so
# just running it is the enforcement — no separate threshold-parsing
# step needed.
bench:
	go test -run=^$$ -bench=BenchmarkSearch_3000Notes ./internal/store/fsstore/...

# Display- and Wayland-dependent tests are gated behind the integration
# build tag and need a real (or headless) compositor to do anything but
# skip themselves, so this runs them inside scripts/headless.sh's sway
# session rather than the bare `go test`.
#
# -p 1 is load-bearing, not a speed knob. Every package here shares the
# ONE sway session headless.sh starts, and several open real toplevels.
# Run in parallel, those windows steal keyboard focus from each other,
# so internal/e2e's wtype keystrokes land in another package's window
# and its note never reaches disk. Measured: internal/e2e passed 3/3 in
# isolation and failed in the same full suite run without -p 1; the
# whole suite passes with it.
test-integration:
	./scripts/headless.sh go test -tags integration -p 1 ./...

# Regenerates assets/screenshot.png from a genuine capture of the real
# assembled panel (internal/ui/panel.Shell with fixture notes, mapped as
# an actual layer-shell surface) rather than a hand-built mockup.
screenshot:
	INGOT_SCREENSHOT_OUT=$(CURDIR)/assets/screenshot.png \
		./scripts/headless.sh go test -tags integration -count=1 -run TestScreenshot_CapturesTheAssembledPanel -v ./internal/ui/panel/...

# Plain `go vet` fails on internal/layershell: the gotk4 idiom for getting a
# C pointer from a widget converts Native()'s uintptr to unsafe.Pointer,
# which vet reports as "possible misuse of unsafe.Pointer". This is
# mirrored in .golangci.yml (govet.disable: [unsafeptr]).
vet:
	go vet -unsafeptr=false ./...

fmt:
	gofmt -l .

# Fails rather than skips when golangci-lint is missing. Skipping made
# every local `make check` look green while CI enforced the linter, so a
# backlog of findings built up that nobody saw until the repo went
# public and the badge went red.
# LINT=skip opts out explicitly, for a machine that genuinely cannot
# install the linter. Anything else must have it.
lint:
ifeq ($(LINT),skip)
	@echo "lint: skipped by explicit LINT=skip"
else
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "lint: golangci-lint is not installed — install it (pacman -S golangci-lint) or run 'make lint LINT=skip'"; \
		exit 1; \
	}
	golangci-lint run
endif

check:
	@test -z "$$(gofmt -l .)" || (echo "gofmt found unformatted files:" && gofmt -l . && exit 1)
	$(MAKE) vet
	$(MAKE) build
	$(MAKE) test
	$(MAKE) bench
	$(MAKE) lint

# -trimpath and stripped/version-stamped ldflags are release-only: they
# change the build flags from the dev loop above, which is exactly what
# forces the expensive cold cgo rebuild. No separate strip step needed —
# -s -w already strips symbols and DWARF.
release:
	go build -trimpath -ldflags "$(LDFLAGS_RELEASE)" -o dist/ingot ./cmd/ingot

clean:
	rm -rf bin dist

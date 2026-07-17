# lazyBG desktop app (Wails v2 + Svelte).
#
# The real app (main.go) is gated behind the custom `lazybggui` build tag so
# that `go build ./...` from the repo root compiles the stub (main_stub.go)
# instead and stays green on machines without webkit2gtk. The wails CLI does
# NOT add this tag on its own, so every wails invocation must pass
# `-tags lazybggui` — these targets do it for you.
#
# The tag is deliberately NOT named `desktop`: Wails reserves that name and
# STRIPS it during TS-binding generation (see wails/v2/pkg/commands/bindings/
# bindings.go), which would leave main.go's build constraint unsatisfied and
# compile the stub mid-build, aborting `wails build`. `lazybggui` is preserved
# through every step.
#
# webkit2_41 : link against webkit2gtk-4.1. Wails defaults to the 4.0 lib,
# which on Arch is broken (undefined JxlDecoder* refs from a libjxl soname
# bump). 4.1 is the maintained package.

TAGS := lazybggui webkit2_41

.PHONY: dev build clean

dev: ## Run the app with hot reload
	wails dev -tags "$(TAGS)"

build: ## Build the production binary (build/bin/lazybg-gui)
	wails build -tags "$(TAGS)"

clean:
	rm -rf build/bin

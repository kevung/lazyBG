//go:build !lazybggui

// Stub main so `go build ./...` stays green on machines without the Wails
// system dependencies (webkit2gtk). The real app is main.go, compiled when the
// `lazybggui` build tag is passed explicitly (`wails dev -tags lazybggui` /
// `wails build -tags lazybggui` in gui/, or `make dev` / `make build`).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "lazybg-gui: this binary was built without the lazybggui tag.")
	fmt.Fprintln(os.Stderr, "Build the real app with: cd gui && wails build -tags lazybggui (or: make build)")
	os.Exit(1)
}

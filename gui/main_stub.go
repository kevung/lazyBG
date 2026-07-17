//go:build !desktop

// Stub main so `go build ./...` stays green on machines without the Wails
// system dependencies (webkit2gtk). The real app is main.go, compiled when the
// wails CLI adds the `desktop` build tag (`wails dev` / `wails build` in gui/).
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "lazybg-gui: this binary was built without the desktop tag.")
	fmt.Fprintln(os.Stderr, "Build the real app with the wails CLI: cd gui && wails build")
	os.Exit(1)
}

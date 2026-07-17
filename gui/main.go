//go:build lazybggui

// The lazyBG desktop app: a thin Wails v2 shell over internal/session
// (ADR-0002, ADR-0003). Build with `wails build -tags lazybggui` /
// `wails dev -tags lazybggui` from gui/ (or `make build` / `make dev`) — the
// `lazybggui` tag is a lazyBG-specific tag the wails CLI does NOT add on its
// own, so it must be passed explicitly. Plain `go build ./...` omits it and
// compiles the stub instead, so the repo stays green on machines without
// webkit. (The tag is deliberately NOT named `desktop`: Wails reserves that
// name and strips it during TS-binding generation, which would compile the
// stub mid-build — see gui/Makefile.)
package main

import (
	"embed"
	"log"
	"net/http"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	lazybg "lazybg"
	"lazybg/internal/engine"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := engine.Init(lazybg.DataFS); err != nil {
		log.Fatal("engine init: ", err)
	}
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "lazyBG",
		Width:  1440,
		Height: 900,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: app.mediaHandler(),
		},
		OnStartup: app.startup,
		Bind:      []interface{}{app},
	})
	if err != nil {
		log.Fatal(err)
	}
}

// mediaHandler serves the currently-open video file under /media/current so
// the webview's HTML5 <video> can play and seek it (http.ServeFile handles
// Range requests). Only paths the user explicitly picked are ever served.
func (a *App) mediaHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/media/current") {
			http.NotFound(w, r)
			return
		}
		path := a.currentVideoPath()
		if path == "" {
			http.Error(w, "no video open", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, path)
	})
}

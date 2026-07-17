//go:build desktop

// The lazyBG desktop app: a thin Wails v2 shell over internal/session
// (ADR-0002, ADR-0003). Build with `wails build` / `wails dev` from gui/ —
// the wails CLI adds the `desktop` tag; plain `go build ./...` compiles the
// stub instead, so the repo stays green on machines without webkit.
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

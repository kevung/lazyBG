package main

import (
	"embed"
	"fmt"
	"lazybg/gnubg"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
)

//go:embed build/appicon.png
var icon []byte

//go:embed all:frontend/dist
var assets embed.FS

//go:embed all:data
var gnubgData embed.FS

func main() {

	app := NewApp()
	cfg := NewConfig()

	// Initialize gnubg engine with embedded data
	if err := gnubg.Init(gnubgData); err != nil {
		fmt.Println("Error initializing gnubg:", err)
		return
	}
	defer gnubg.Destroy()

	// Load the configuration file
	config, err := cfg.LoadConfig()
	if err != nil {
		fmt.Println("Error loading configuration file:", err)
		return
	}

	// Initialize width and height from config
	initialWidth := config.WindowWidth
	initialHeight := config.WindowHeight
	fmt.Println("Initial dimensions:", initialWidth, "x", initialHeight)
	fmt.Println("Aspect ratio:", float64(initialHeight)/float64(initialWidth))

	// Create application with options
	err = wails.Run(&options.App{
		Title:  "lazyBG",
		Width:  initialWidth,
		Height: initialHeight,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 240, G: 240, B: 240, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
			cfg,
		},
		Linux: &linux.Options{
			Icon:                icon,
			WindowIsTranslucent: false,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyNever,
			ProgramName:         "lazyBG",
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: false,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

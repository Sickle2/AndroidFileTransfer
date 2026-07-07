package main

import (
	"context"
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// androidUIAssets holds the embedded android-ui static files (HTML/CSS/JS)
// served by the WiFiServer to the Android browser.
//
//go:embed android-ui
var androidUIAssets embed.FS

func main() {
	// Create the App (which internally constructs the Manager and WiFiServer).
	app := newApp()

	// Inject the android-ui embedded filesystem into the WiFiServer before
	// manager.Start() is called (startup() does the Start call).
	// fs.Sub strips the "android-ui/" prefix so "/" maps to index.html.
	uiSub, err := fs.Sub(androidUIAssets, "android-ui")
	if err != nil {
		log.Fatalf("android-ui embed sub: %v", err)
	}
	app.manager.WiFiServer().SetUIFS(uiSub)

	// Run Wails application.
	if err := wails.Run(&options.App{
		Title:  "AndroidFileTransfer",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		// OnBeforeClose is called when the user closes the window; Stop the
		// backend services gracefully.  Returning false lets the window close.
		OnBeforeClose: func(ctx context.Context) bool {
			app.manager.Stop()
			return false
		},
		Bind: []interface{}{
			app,
		},
	}); err != nil {
		log.Println("Error:", err.Error())
	}
}

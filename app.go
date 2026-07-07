package main

import (
	"context"
	"log/slog"
	"os"

	"AndroidFileTransfer/internal/connection"
	"AndroidFileTransfer/internal/model"
	"AndroidFileTransfer/internal/util"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App holds the application state and backend services exposed to the Wails frontend.
type App struct {
	ctx     context.Context
	manager *connection.Manager
}

// newApp creates a new App instance.
// WiFiServer root defaults to the user's home directory for MVP.
// A more restrictive path (e.g. ~/Downloads) can be offered via a future settings UI.
func newApp() *App {
	home, _ := os.UserHomeDir()
	wifiSrv := connection.NewWiFiServer(home)

	// NewADBManager may fail if adb is not installed; that is non-fatal — the
	// app continues in WiFi-only mode.
	adbMgr, err := connection.NewADBManager()
	if err != nil {
		slog.Warn("ADB not available, running in WiFi-only mode", "err", err)
		adbMgr = nil
	}

	mgr := connection.NewManager(wifiSrv, adbMgr)
	return &App{manager: mgr}
}

// startup is called by Wails immediately after the window is ready.
// It initialises the logger, starts backend services, and launches the progress
// fan-out goroutine so ADB transfer events are forwarded to the Wails runtime.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	util.InitLogger()

	if err := a.manager.Start(); err != nil {
		runtime.LogErrorf(ctx, "manager start failed: %v", err)
		// Non-fatal: WiFiServer failure means WiFi path is unavailable but ADB
		// may still work (or vice-versa).
	}

	// Fan ADB progress events to the Wails frontend event bus.
	ch := a.manager.SubscribeProgress()
	go func() {
		for p := range ch {
			runtime.EventsEmit(a.ctx, "transfer:progress", p)
		}
	}()
}

// shutdown is called by Wails before the application exits.
func (a *App) shutdown(ctx context.Context) {
	a.manager.Stop()
}

// --- Bound methods (callable from TypeScript via Wails bindings) ---

// ListDevices returns all currently visible devices (WiFi + ADB).
func (a *App) ListDevices() []model.Device {
	return a.manager.ListDevices()
}

// GetFileList returns directory contents for the given device and path.
func (a *App) GetFileList(deviceID, path string) ([]model.FileInfo, error) {
	return a.manager.GetFileList(deviceID, path)
}

// Download transfers a file from an ADB device to localPath on the Mac.
// A "transfer:progress" event is emitted upon completion (or failure).
func (a *App) Download(deviceID, remotePath, localPath string) error {
	err := a.manager.Download(deviceID, remotePath, localPath)
	progress := model.TransferProgress{
		DeviceID: deviceID,
		FileName: remotePath,
	}
	if err != nil {
		progress.Error = err.Error()
	} else {
		progress.BytesDone = -1 // sentinel: transfer completed
	}
	runtime.EventsEmit(a.ctx, "transfer:progress", progress)
	return err
}

// Upload transfers a file from the Mac to an ADB device at remotePath.
// A "transfer:progress" event is emitted upon completion (or failure).
func (a *App) Upload(deviceID, localPath, remotePath string) error {
	err := a.manager.Upload(deviceID, localPath, remotePath)
	progress := model.TransferProgress{
		DeviceID: deviceID,
		FileName: localPath,
	}
	if err != nil {
		progress.Error = err.Error()
	} else {
		progress.BytesDone = -1 // sentinel: transfer completed
	}
	runtime.EventsEmit(a.ctx, "transfer:progress", progress)
	return err
}

// GetWiFiAddress returns the URL the Android browser should open (e.g. http://192.168.1.x:8080).
func (a *App) GetWiFiAddress() string {
	return a.manager.WiFiAddress()
}

// GetWiFiQRCode returns a PNG data URI encoding the WiFi server URL.
func (a *App) GetWiFiQRCode() string {
	return a.manager.WiFiQRCode()
}

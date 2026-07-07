# AndroidFileTransfer

A macOS desktop app for transferring files between a Mac and an Android phone, built with [Wails](https://wails.io) (Go backend + React/TypeScript frontend).

Two transfer paths are supported, side by side:

- **WiFi**: the phone opens a URL in its browser (no app install, no cable) to browse/download/upload files against a local HTTP server run by the app.
- **ADB (USB)**: with USB debugging enabled, the phone's file system (`/sdcard`) is browsed and files are pushed/pulled directly over USB.

## System requirements

- macOS 12 (Monterey) or later, Apple Silicon or Intel
- [ADB](https://developer.android.com/tools/adb) (`android-platform-tools`) installed for the USB path — optional if you only use WiFi transfer
- An Android phone and Mac on the same local network for the WiFi path

## Installing ADB

The ADB (USB) transfer path requires the Android platform tools to be installed and available on your `PATH`. The easiest way on macOS is via Homebrew:

```bash
brew install android-platform-tools
```

Verify it's installed:

```bash
adb version
```

If `adb` isn't found, the app still runs and falls back to WiFi-only mode — ADB devices simply won't appear in the device list.

## Building and running

```bash
# install frontend deps and build
cd frontend && npm install && npm run build && cd ..

# run in dev mode (hot reload)
wails dev

# build a production app bundle
wails build -platform darwin/universal
```

The built app is placed at `build/bin/AndroidFileTransfer.app`.

## Using WiFi transfer

1. Make sure your Mac and Android phone are connected to the same WiFi network.
2. Launch the app. It starts a local HTTP server and shows a QR code alongside an address like `http://192.168.1.x:8080`.
3. On your phone, scan the QR code (or open the address manually in a mobile browser).
4. Browse files from your Mac's home directory in the phone's browser: tap to download to the phone, or use the upload control to send a file from the phone to the Mac.

No app installation is required on the Android phone — everything happens through the browser.

## Using ADB (USB) transfer

1. On your Android phone, enable Developer Options (Settings → About phone → tap "Build number" 7 times).
2. In Developer Options, enable **USB debugging**.
3. Connect the phone to the Mac with a USB cable.
4. The first time you connect, your phone will prompt **"Allow USB debugging?"** — tap Allow (and optionally "Always allow from this computer").
5. The device should appear automatically in the app's device list. Browse `/sdcard`, and pull files to the Mac or push files from the Mac to the phone.

If the device shows as "unauthorized," check the phone screen for the USB debugging permission prompt and accept it.

## Troubleshooting

- **ADB not installed**: the app shows a notice and continues in WiFi-only mode instead of crashing.
- **Device unauthorized**: accept the "Allow USB debugging" prompt on the phone.
- **WiFi ports unavailable**: the app tries a small range of ports (8080-8084); if all are in use, it reports an error rather than crashing. Free up a port or restart the app.

## Development

```bash
# run backend tests
go test ./...

# build the Go backend
go build ./...

# build the frontend
cd frontend && npm run build
```

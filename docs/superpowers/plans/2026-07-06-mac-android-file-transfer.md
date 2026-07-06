# Mac-Android 文件传输工具实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 构建一个 Mac 桌面应用，通过 WiFi（HTTP + 浏览器）和 USB（ADB）两种方式实现 Mac 与 Android 设备之间的双向文件传输。

**Architecture:** Wails v2 框架，Go 后端处理文件传输逻辑，React + TypeScript 前端渲染 Mac GUI。WiFi 模式在 Mac 本地起 HTTP server，Android 用浏览器访问；ADB 模式通过 os/exec 调用系统 adb 命令。三个连接通道（WiFi/ADB/MTP）通过统一的 ConnectionManager 接口路由，GUI 层只依赖该接口。

**Tech Stack:** Go 1.21+, Wails v2, React 18, TypeScript 5, Vite, github.com/skip2/go-qrcode

## Global Constraints

- Go 版本：1.21+（使用 log/slog、embed）
- Wails 版本：v2.9+
- Node.js 版本：18+（前端构建）
- 目标平台：macOS 12+（darwin/arm64 + darwin/amd64，universal binary）
- 测试框架：Go 标准库 `testing` + `net/http/httptest`
- ADB 依赖：系统已安装 `adb`（通过 `brew install android-platform-tools`），App 不内置
- 所有用户提示文字：中文
- 日志路径：`~/Library/Logs/AndroidFileTransfer/app.log`

---

## 文件清单

| 文件 | 职责 |
|------|------|
| `main.go` | Wails 应用入口，初始化 App |
| `app.go` | Wails App 结构体，暴露给前端的所有方法 |
| `internal/model/types.go` | 共享数据结构：Device, FileInfo, TransferProgress |
| `internal/util/network.go` | 获取本机 LAN IP，生成二维码 base64 |
| `internal/util/logger.go` | slog 日志初始化，写入 ~/Library/Logs |
| `internal/connection/wifi.go` | WiFiServer：HTTP server，文件 API，端口探测 |
| `internal/connection/adb.go` | ADBManager：检测 adb，轮询设备，pull/push |
| `internal/connection/progress.go` | TransferProgress channel 广播器 |
| `internal/connection/manager.go` | ConnectionManager：聚合 WiFi/ADB，路由分发 |
| `android-ui/index.html` | Android 浏览器 UI 入口 |
| `android-ui/style.css` | Android 浏览器 UI 样式 |
| `android-ui/app.js` | Android 浏览器 UI 逻辑（原生 JS） |
| `frontend/src/main.tsx` | React 入口 |
| `frontend/src/App.tsx` | 主界面布局（左右分栏） |
| `frontend/src/components/DeviceList.tsx` | 左侧设备列表组件 |
| `frontend/src/components/FileBrowser.tsx` | 右侧文件浏览器组件 |
| `frontend/src/components/TransferQueue.tsx` | 底部传输进度条组件 |
| `frontend/src/components/QRCodeDisplay.tsx` | WiFi 二维码弹窗组件 |
| `frontend/src/hooks/useDevices.ts` | 设备轮询 hook |
| `frontend/src/styles/app.css` | 全局样式 |

---

### Task 1: Bootstrap Wails 项目

**Files:**
- Create: `main.go`
- Create: `app.go`
- Create: `wails.json`
- Create: `go.mod`

**Interfaces:**
- Produces: `App` struct（后续任务挂载方法到此结构体）

- [ ] **Step 1: 安装 Wails CLI**

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails version
```
Expected: 输出 `Wails CLI v2.9.x`

- [ ] **Step 2: 初始化项目**

```bash
cd /Users/sickle/code/ai/android-file-transfer
wails init -n AndroidFileTransfer -t react-ts
```

这会生成 `main.go`、`app.go`、`wails.json`、`frontend/` 等文件。

- [ ] **Step 3: 验证默认项目能启动**

```bash
wails dev
```
Expected: 浏览器弹出窗口，显示 Wails 默认 React 页面，控制台无报错。

- [ ] **Step 4: 清空 app.go 的默认方法，保留骨架**

把生成的 `app.go` 改成：

```go
package main

import (
	"context"
)

// App holds the application state and backend services.
type App struct {
	ctx context.Context
}

// newApp creates a new App instance.
func newApp() *App {
	return &App{}
}

// startup is called by Wails when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
```

- [ ] **Step 5: 验证编译通过**

```bash
go build ./...
```
Expected: 无报错输出

- [ ] **Step 6: Commit**

```bash
git init
git add .
git commit -m "feat: bootstrap Wails project"
```

---

### Task 2: 定义共享数据结构

**Files:**
- Create: `internal/model/types.go`
- Create: `internal/model/types_test.go`

**Interfaces:**
- Produces:
  - `model.Device{ID, Name, Type, Status string}`
  - `model.FileInfo{Name, Path string; Size int64; IsDir bool; ModTime time.Time}`
  - `model.TransferProgress{DeviceID, FileName string; BytesDone, TotalBytes int64; Error string}`

- [ ] **Step 1: 创建目录**

```bash
mkdir -p internal/model
```

- [ ] **Step 2: 写测试**

创建 `internal/model/types_test.go`：

```go
package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeviceJSONRoundTrip(t *testing.T) {
	d := Device{ID: "adb:abc123", Name: "Pixel 6", Type: "adb", Status: "connected"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var d2 Device
	if err := json.Unmarshal(b, &d2); err != nil {
		t.Fatal(err)
	}
	if d2.ID != d.ID || d2.Type != d.Type {
		t.Errorf("round-trip mismatch: got %+v", d2)
	}
}

func TestFileInfoJSONRoundTrip(t *testing.T) {
	f := FileInfo{Name: "test.txt", Path: "/sdcard/test.txt", Size: 1024, IsDir: false, ModTime: time.Now().Truncate(time.Second)}
	b, _ := json.Marshal(f)
	var f2 FileInfo
	json.Unmarshal(b, &f2)
	if f2.Name != f.Name || f2.Size != f.Size {
		t.Errorf("round-trip mismatch: got %+v", f2)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./internal/model/...
```
Expected: FAIL with "no Go files"

- [ ] **Step 4: 实现 types.go**

创建 `internal/model/types.go`：

```go
package model

import "time"

// Device represents a connected Android device (WiFi, ADB, or MTP).
type Device struct {
	ID     string `json:"id"`     // "wifi:<ip>:<port>" | "adb:<serial>" | "mtp:<id>"
	Name   string `json:"name"`
	Type   string `json:"type"`   // "wifi" | "adb" | "mtp"
	Status string `json:"status"` // "connected" | "transferring" | "disconnected"
}

// FileInfo describes a file or directory entry.
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"isDir"`
	ModTime time.Time `json:"modTime"`
}

// TransferProgress is emitted during file transfer operations.
type TransferProgress struct {
	DeviceID   string `json:"deviceId"`
	FileName   string `json:"fileName"`
	BytesDone  int64  `json:"bytesDone"`
	TotalBytes int64  `json:"totalBytes"`
	Error      string `json:"error,omitempty"`
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/model/...
```
Expected: `ok  	android-file-transfer/internal/model`

- [ ] **Step 6: Commit**

```bash
git add internal/model/
git commit -m "feat: define shared model types"
```

---

### Task 3: 工具函数（网络 IP + 二维码 + 日志）

**Files:**
- Create: `internal/util/network.go`
- Create: `internal/util/network_test.go`
- Create: `internal/util/logger.go`

**Interfaces:**
- Produces:
  - `util.GetLocalIP() (string, error)`
  - `util.GenerateQRCodeBase64(url string) (string, error)` — 返回 `data:image/png;base64,...`
  - `util.InitLogger()` — 初始化全局 slog，写入 `~/Library/Logs/AndroidFileTransfer/app.log`

- [ ] **Step 1: 添加依赖**

```bash
go get github.com/skip2/go-qrcode@latest
```

- [ ] **Step 2: 写测试**

创建 `internal/util/network_test.go`：

```go
package util

import (
	"strings"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		t.Fatalf("GetLocalIP() error: %v", err)
	}
	if ip == "" || ip == "127.0.0.1" {
		t.Errorf("expected LAN IP, got %q", ip)
	}
	if len(strings.Split(ip, ".")) != 4 {
		t.Errorf("expected IPv4, got %q", ip)
	}
}

func TestGenerateQRCodeBase64(t *testing.T) {
	b64, err := GenerateQRCodeBase64("http://192.168.1.100:8080")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.HasPrefix(b64, "data:image/png;base64,") {
		t.Errorf("expected data URI prefix, got: %q", b64[:30])
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./internal/util/...
```
Expected: FAIL with "undefined: GetLocalIP"

- [ ] **Step 4: 实现 network.go**

创建 `internal/util/network.go`：

```go
package util

import (
	"encoding/base64"
	"fmt"
	"net"

	qrcode "github.com/skip2/go-qrcode"
)

// GetLocalIP returns the machine's LAN IPv4 by dialing a UDP connection
// (no packet is actually sent).
func GetLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("GetLocalIP: %w", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// GenerateQRCodeBase64 encodes url as a QR code and returns a PNG data URI.
func GenerateQRCodeBase64(url string) (string, error) {
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("GenerateQRCodeBase64: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
```

- [ ] **Step 5: 实现 logger.go**

创建 `internal/util/logger.go`：

```go
package util

import (
	"log/slog"
	"os"
	"path/filepath"
)

// InitLogger configures slog to write JSON to ~/Library/Logs/AndroidFileTransfer/app.log.
// Falls back to stderr on any error.
func InitLogger() {
	logDir := filepath.Join(os.Getenv("HOME"), "Library", "Logs", "AndroidFileTransfer")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		return
	}
	f, err := os.OpenFile(
		filepath.Join(logDir, "app.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644,
	)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})))
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
go test ./internal/util/...
```
Expected: `ok  	android-file-transfer/internal/util`

- [ ] **Step 7: Commit**

```bash
git add internal/util/ go.mod go.sum
git commit -m "feat: add network utils and logger"
```

---

### Task 4: WiFi Server（HTTP 文件服务 + 端口探测）

**Files:**
- Create: `internal/connection/wifi.go`
- Create: `internal/connection/wifi_test.go`
- Create: `android-ui/index.html`
- Create: `android-ui/style.css`
- Create: `android-ui/app.js`

**Interfaces:**
- Consumes: `model.FileInfo`, `util.GetLocalIP`, `util.GenerateQRCodeBase64`
- Produces:
  - `WiFiServer.Start() error` — 探测端口，启动 HTTP server
  - `WiFiServer.Stop()`
  - `WiFiServer.Address() string` — 返回 `http://<ip>:<port>`
  - `WiFiServer.QRCode() string` — 返回 data URI
  - HTTP `GET /api/files?path=<path>` → `[]model.FileInfo` JSON
  - HTTP `GET /api/download?path=<path>` → 文件流
  - HTTP `POST /api/upload` → `{"success":true}`
  - HTTP `GET /` → `android-ui/index.html`

- [ ] **Step 1: 创建目录和嵌入准备**

```bash
mkdir -p internal/connection android-ui
```

- [ ] **Step 2: 写 HTTP server 测试**

创建 `internal/connection/wifi_test.go`：

```go
package connection

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"android-file-transfer/internal/model"
)

func TestWiFiServerFilesAPI(t *testing.T) {
	// create temp dir with one file
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644)

	srv := &WiFiServer{rootDir: dir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files?path=" + dir)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var files []model.FileInfo
	json.NewDecoder(resp.Body).Decode(&files)
	if len(files) == 0 {
		t.Fatal("expected at least one file")
	}
	found := false
	for _, f := range files {
		if f.Name == "hello.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("hello.txt not in response: %+v", files)
	}
}

func TestWiFiServerDownloadAPI(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("file content"), 0o644)

	srv := &WiFiServer{rootDir: dir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/api/download?path=" + filePath)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "file content" {
		t.Errorf("expected 'file content', got %q", body)
	}
}

func TestWiFiServerUploadAPI(t *testing.T) {
	dir := t.TempDir()
	srv := &WiFiServer{rootDir: dir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	body := strings.NewReader("--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"up.txt\"\r\n\r\nupload content\r\n--boundary--\r\n")
	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?path="+dir, body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "up.txt"))
	if string(content) != "upload content" {
		t.Errorf("expected 'upload content', got %q", content)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
go test ./internal/connection/... -run TestWiFiServer
```
Expected: FAIL with "undefined: WiFiServer"

- [ ] **Step 4: 实现 wifi.go**

创建 `internal/connection/wifi.go`：

```go
package connection

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"android-file-transfer/internal/model"
	"android-file-transfer/internal/util"
)

//go:embed ../../android-ui/*
var androidUIFS embed.FS

// WiFiServer serves files over HTTP for Android browser access.
type WiFiServer struct {
	rootDir string
	server  *http.Server
	address string // http://<ip>:<port>
	qrCode  string // data URI
}

// NewWiFiServer creates a server with the user's home dir as root.
func NewWiFiServer() *WiFiServer {
	home, _ := os.UserHomeDir()
	return &WiFiServer{rootDir: home}
}

// Start finds a free port (8080-8084) and begins serving.
func (s *WiFiServer) Start() error {
	mux := s.handler()
	var ln net.Listener
	var port int
	for _, p := range []int{8080, 8081, 8082, 8083, 8084} {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			ln = l
			port = p
			break
		}
	}
	if ln == nil {
		return fmt.Errorf("WiFiServer: no free port in range 8080-8084")
	}

	ip, err := util.GetLocalIP()
	if err != nil {
		ip = "localhost"
	}
	s.address = fmt.Sprintf("http://%s:%d", ip, port)
	s.qrCode, _ = util.GenerateQRCodeBase64(s.address)

	s.server = &http.Server{Handler: mux}
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("WiFiServer error", "err", err)
		}
	}()
	slog.Info("WiFiServer started", "address", s.address)
	return nil
}

// Stop shuts down the HTTP server.
func (s *WiFiServer) Stop() {
	if s.server != nil {
		s.server.Close()
	}
}

// Address returns the server URL (http://<ip>:<port>).
func (s *WiFiServer) Address() string { return s.address }

// QRCode returns a PNG data URI for the server URL.
func (s *WiFiServer) QRCode() string { return s.qrCode }

// handler builds the HTTP mux (exported for testing with httptest).
func (s *WiFiServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/upload", s.handleUpload)
	// serve android-ui static files at root
	mux.Handle("/", http.FileServer(http.FS(androidUIFS)))
	return mux
}

func (s *WiFiServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = s.rootDir
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var files []model.FileInfo
	for _, e := range entries {
		info, _ := e.Info()
		files = append(files, model.FileInfo{
			Name:    e.Name(),
			Path:    filepath.Join(path, e.Name()),
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			ModTime: info.ModTime(),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (s *WiFiServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	io.Copy(w, f)
}

func (s *WiFiServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	destDir := r.URL.Query().Get("path")
	if destDir == "" {
		destDir = s.rootDir
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, fh := range r.MultipartForm.File["file"] {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		defer src.Close()
		dst, err := os.Create(filepath.Join(destDir, fh.Filename))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		io.Copy(dst, src)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true}`))
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
go test ./internal/connection/... -run TestWiFiServer -v
```
Expected: 三个测试全部 PASS

- [ ] **Step 6: Commit**

```bash
git add internal/connection/wifi.go internal/connection/wifi_test.go
git commit -m "feat: implement WiFiServer with file API"
```

---

### Task 5: Android 浏览器 UI

**Files:**
- Create: `android-ui/index.html`
- Create: `android-ui/style.css`
- Create: `android-ui/app.js`

**Interfaces:**
- Consumes: HTTP `/api/files`, `/api/download`, `/api/upload`（Task 4 产出）
- Produces: 可在手机浏览器打开的文件管理页面

- [ ] **Step 1: 创建 index.html**

创建 `android-ui/index.html`：

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Android File Transfer</title>
  <link rel="stylesheet" href="style.css">
</head>
<body>
  <header>
    <h1>📁 文件传输</h1>
    <div id="breadcrumb"></div>
  </header>

  <main>
    <div id="file-list"></div>
    <div id="empty-tip" class="hidden">此目录为空</div>
  </main>

  <footer>
    <label class="upload-btn">
      📤 上传文件
      <input type="file" id="upload-input" multiple hidden>
    </label>
    <div id="progress-bar" class="hidden">
      <div id="progress-fill"></div>
      <span id="progress-text"></span>
    </div>
  </footer>

  <script src="app.js"></script>
</body>
</html>
```

- [ ] **Step 2: 创建 style.css**

创建 `android-ui/style.css`：

```css
* { box-sizing: border-box; margin: 0; padding: 0; }
body { font-family: -apple-system, sans-serif; background: #f5f5f5; color: #333; }
header { background: #1976d2; color: white; padding: 12px 16px; position: sticky; top: 0; z-index: 10; }
header h1 { font-size: 18px; margin-bottom: 4px; }
#breadcrumb { font-size: 12px; opacity: 0.85; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
#breadcrumb span { cursor: pointer; }
#breadcrumb span:hover { text-decoration: underline; }
main { padding: 8px; min-height: calc(100vh - 120px); }
.file-item { display: flex; align-items: center; background: white; border-radius: 8px;
  margin-bottom: 8px; padding: 12px; cursor: pointer; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
.file-item:active { background: #f0f0f0; }
.file-icon { font-size: 24px; margin-right: 12px; }
.file-info { flex: 1; overflow: hidden; }
.file-name { font-size: 15px; font-weight: 500; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.file-meta { font-size: 12px; color: #888; margin-top: 2px; }
.download-btn { color: #1976d2; font-size: 20px; padding: 4px 8px; }
footer { position: fixed; bottom: 0; left: 0; right: 0; background: white;
  padding: 12px 16px; border-top: 1px solid #eee; }
.upload-btn { display: inline-block; background: #1976d2; color: white;
  padding: 10px 20px; border-radius: 8px; cursor: pointer; font-size: 15px; }
#progress-bar { margin-top: 8px; background: #eee; border-radius: 4px; height: 20px;
  position: relative; overflow: hidden; }
#progress-fill { height: 100%; background: #1976d2; transition: width 0.2s; }
#progress-text { position: absolute; top: 0; left: 0; right: 0; text-align: center;
  font-size: 12px; line-height: 20px; color: #333; }
#empty-tip { text-align: center; color: #aaa; margin-top: 40px; font-size: 15px; }
.hidden { display: none; }
```

- [ ] **Step 3: 创建 app.js**

创建 `android-ui/app.js`：

```javascript
let currentPath = '/';

async function loadFiles(path) {
  currentPath = path;
  updateBreadcrumb(path);
  const res = await fetch('/api/files?path=' + encodeURIComponent(path));
  const files = await res.json();
  renderFiles(files || []);
}

function updateBreadcrumb(path) {
  const parts = path.split('/').filter(Boolean);
  const crumb = document.getElementById('breadcrumb');
  crumb.innerHTML = '<span data-path="/">根目录</span>';
  let built = '';
  parts.forEach(part => {
    built += '/' + part;
    const p = built;
    crumb.innerHTML += ` / <span data-path="${p}">${part}</span>`;
  });
  crumb.querySelectorAll('span').forEach(s => {
    s.addEventListener('click', () => loadFiles(s.dataset.path));
  });
}

function renderFiles(files) {
  const list = document.getElementById('file-list');
  const empty = document.getElementById('empty-tip');
  list.innerHTML = '';
  if (files.length === 0) {
    empty.classList.remove('hidden');
    return;
  }
  empty.classList.add('hidden');
  files.forEach(f => {
    const div = document.createElement('div');
    div.className = 'file-item';
    const icon = f.isDir ? '📁' : getFileIcon(f.name);
    const size = f.isDir ? '' : formatSize(f.size);
    div.innerHTML = `
      <span class="file-icon">${icon}</span>
      <div class="file-info">
        <div class="file-name">${f.name}</div>
        <div class="file-meta">${size}</div>
      </div>
      ${f.isDir ? '' : `<span class="download-btn" data-path="${f.path}">⬇️</span>`}
    `;
    if (f.isDir) {
      div.addEventListener('click', () => loadFiles(f.path));
    } else {
      div.querySelector('.download-btn').addEventListener('click', e => {
        e.stopPropagation();
        window.location.href = '/api/download?path=' + encodeURIComponent(f.path);
      });
    }
    list.appendChild(div);
  });
}

function getFileIcon(name) {
  const ext = name.split('.').pop().toLowerCase();
  const icons = { jpg:'🖼️', jpeg:'🖼️', png:'🖼️', gif:'🖼️', mp4:'🎬', mov:'🎬',
    mp3:'🎵', wav:'🎵', pdf:'📄', zip:'📦', txt:'📝', doc:'📝', docx:'📝' };
  return icons[ext] || '📄';
}

function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / 1024 / 1024).toFixed(1) + ' MB';
}

// Upload
document.getElementById('upload-input').addEventListener('change', async function() {
  const files = Array.from(this.files);
  if (files.length === 0) return;
  const bar = document.getElementById('progress-bar');
  const fill = document.getElementById('progress-fill');
  const text = document.getElementById('progress-text');
  bar.classList.remove('hidden');

  for (let i = 0; i < files.length; i++) {
    const f = files[i];
    const pct = Math.round(((i) / files.length) * 100);
    fill.style.width = pct + '%';
    text.textContent = `上传中 ${i+1}/${files.length}: ${f.name}`;

    const fd = new FormData();
    fd.append('file', f);
    await fetch('/api/upload?path=' + encodeURIComponent(currentPath), { method: 'POST', body: fd });
  }
  fill.style.width = '100%';
  text.textContent = '上传完成';
  setTimeout(() => bar.classList.add('hidden'), 2000);
  this.value = '';
  loadFiles(currentPath);
});

// Start
loadFiles('/Users');
```

- [ ] **Step 4: 在浏览器中手工验证**

```bash
# 在另一个终端启动 WiFiServer 测试
cd /Users/sickle/code/ai/android-file-transfer
go run . &
# 用 Mac Safari/Chrome 打开 http://localhost:8080
# 确认文件列表显示正常、下载链接有效
```

- [ ] **Step 5: Commit**

```bash
git add android-ui/
git commit -m "feat: add Android browser UI"
```

---

### Task 6: ADB Manager

**Files:**
- Create: `internal/connection/adb.go`
- Create: `internal/connection/adb_test.go`

**Interfaces:**
- Consumes: `model.Device`, `model.FileInfo`
- Produces:
  - `NewADBManager() (*ADBManager, error)` — 检测 adb 是否安装
  - `ADBManager.DetectDevices() []model.Device`
  - `ADBManager.ListFiles(serial, path string) ([]model.FileInfo, error)`
  - `ADBManager.Pull(serial, remotePath, localPath string) error`
  - `ADBManager.Push(serial, localPath, remotePath string) error`

- [ ] **Step 1: 写测试（mock exec）**

创建 `internal/connection/adb_test.go`：

```go
package connection

import (
	"testing"
)

func TestADBManagerNew_ADBNotFound(t *testing.T) {
	// Override adb path to something that doesn't exist
	mgr := &ADBManager{adbPath: "/nonexistent/adb"}
	_, err := mgr.runCmd("version")
	if err == nil {
		t.Fatal("expected error when adb not found")
	}
}

func TestParseADBDevices(t *testing.T) {
	output := "List of devices attached\nabc123\tdevice\nxyz789\tunauthorized\n"
	devices := parseADBDevices(output)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if devices[0].ID != "adb:abc123" {
		t.Errorf("expected adb:abc123, got %s", devices[0].ID)
	}
	if devices[0].Status != "connected" {
		t.Errorf("expected connected, got %s", devices[0].Status)
	}
	if devices[1].Status != "disconnected" {
		t.Errorf("unauthorized device should be disconnected, got %s", devices[1].Status)
	}
}

func TestParseADBFiles(t *testing.T) {
	output := `-rw-rw-rw- 1 root sdcard_rw  1024 2024-01-15 10:30 file.txt
drwxrwx--x 2 root sdcard_rw  4096 2024-01-15 10:00 folder
`
	files := parseADBFiles(output, "/sdcard")
	if len(files) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(files))
	}
	if files[0].Name != "file.txt" || files[0].Size != 1024 || files[0].IsDir {
		t.Errorf("unexpected file: %+v", files[0])
	}
	if files[1].Name != "folder" || !files[1].IsDir {
		t.Errorf("unexpected dir: %+v", files[1])
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
go test ./internal/connection/... -run TestADB -v
```
Expected: FAIL with "undefined: ADBManager"

- [ ] **Step 3: 实现 adb.go**

创建 `internal/connection/adb.go`：

```go
package connection

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"android-file-transfer/internal/model"
)

// ADBManager communicates with Android devices via the adb command-line tool.
type ADBManager struct {
	adbPath string
}

// NewADBManager locates the adb binary; returns an error if not found.
func NewADBManager() (*ADBManager, error) {
	path, err := exec.LookPath("adb")
	if err != nil {
		return nil, fmt.Errorf("adb 未安装，请运行：brew install android-platform-tools")
	}
	return &ADBManager{adbPath: path}, nil
}

// runCmd runs `adb <args>` and returns combined output.
func (m *ADBManager) runCmd(args ...string) (string, error) {
	out, err := exec.Command(m.adbPath, args...).CombinedOutput()
	return string(out), err
}

// DetectDevices returns all currently connected ADB devices.
func (m *ADBManager) DetectDevices() []model.Device {
	out, err := m.runCmd("devices")
	if err != nil {
		slog.Error("adb devices failed", "err", err)
		return nil
	}
	return parseADBDevices(out)
}

// parseADBDevices parses the output of `adb devices`.
func parseADBDevices(output string) []model.Device {
	var devices []model.Device
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] == "List" {
			continue
		}
		serial, state := fields[0], fields[1]
		status := "connected"
		if state != "device" {
			status = "disconnected"
		}
		devices = append(devices, model.Device{
			ID:     "adb:" + serial,
			Name:   serial, // overridden by GetDeviceName if available
			Type:   "adb",
			Status: status,
		})
	}
	return devices
}

// ListFiles lists files in path on the given device (serial without "adb:" prefix).
func (m *ADBManager) ListFiles(serial, path string) ([]model.FileInfo, error) {
	out, err := m.runCmd("-s", serial, "shell", "ls", "-la", path)
	if err != nil {
		return nil, fmt.Errorf("adb ls: %w", err)
	}
	return parseADBFiles(out, path), nil
}

// parseADBFiles parses `adb shell ls -la` output.
func parseADBFiles(output, dir string) []model.FileInfo {
	var files []model.FileInfo
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		perms := fields[0]
		if perms == "total" {
			continue
		}
		name := fields[len(fields)-1]
		if name == "." || name == ".." {
			continue
		}
		isDir := strings.HasPrefix(perms, "d")
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		// parse date "2024-01-15 10:30" from fields[5] and fields[6]
		modTime := time.Time{}
		if len(fields) >= 7 {
			t, err := time.Parse("2006-01-02 15:04", fields[5]+" "+fields[6])
			if err == nil {
				modTime = t
			}
		}
		files = append(files, model.FileInfo{
			Name:    name,
			Path:    filepath.Join(dir, name),
			Size:    size,
			IsDir:   isDir,
			ModTime: modTime,
		})
	}
	return files
}

// Pull downloads a file from the device to localPath.
func (m *ADBManager) Pull(serial, remotePath, localPath string) error {
	slog.Info("adb pull", "serial", serial, "remote", remotePath, "local", localPath)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	_, err := m.runCmd("-s", serial, "pull", remotePath, localPath)
	return err
}

// Push uploads a file from localPath to the device.
func (m *ADBManager) Push(serial, localPath, remotePath string) error {
	slog.Info("adb push", "serial", serial, "local", localPath, "remote", remotePath)
	_, err := m.runCmd("-s", serial, "push", localPath, remotePath)
	return err
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/connection/... -run TestADB -v
```
Expected: 三个测试全部 PASS

- [ ] **Step 5: Commit**

```bash
git add internal/connection/adb.go internal/connection/adb_test.go
git commit -m "feat: implement ADBManager"
```

---

### Task 7: 进度广播器 + ConnectionManager

**Files:**
- Create: `internal/connection/progress.go`
- Create: `internal/connection/manager.go`
- Create: `internal/connection/manager_test.go`

**Interfaces:**
- Consumes: `WiFiServer`, `ADBManager`（Tasks 4、6）
- Produces:
  - `Broadcaster` struct，方法：`Publish(model.TransferProgress)`、`Subscribe() <-chan model.TransferProgress`、`Close()`
  - `Manager` struct，方法：`Start() error`、`Stop()`、`ListDevices() []model.Device`、`RefreshDevices()`、`GetFileList(deviceID, path string) ([]model.FileInfo, error)`、`Download(deviceID, remotePath, localPath string) error`、`Upload(deviceID, localPath, remotePath string) error`、`SubscribeProgress() <-chan model.TransferProgress`

- [ ] **Step 1: 实现 progress.go**

创建 `Broadcaster`，内部维护一个 subscriber slice，`Publish` 向所有 subscriber channel 广播，`Subscribe` 返回新 channel，`Close` 关闭所有 channel。

- [ ] **Step 2: 实现 manager.go**

`Manager` 聚合 `*WiFiServer` 和 `*ADBManager`（以及预留 MTP 字段）。
- `Start()`：依次启动 WiFiServer，初始化 ADBManager（失败时记录日志但不阻断）；启动后台 goroutine 每 2 秒调用 `ADBManager.DetectDevices()` 刷新设备列表
- `ListDevices()`：合并 WiFi 设备（固定1个）和 ADB 设备列表返回
- `GetFileList(deviceID, path)`：按 deviceID 前缀（`"wifi:"` / `"adb:"`）路由到对应子模块
- `Download/Upload`：同样路由；ADB 操作前后通过 `Broadcaster` 发布进度事件

- [ ] **Step 3: 写测试**

测试路由逻辑：传入 `"adb:xyz"` deviceID 时，确认调用的是 ADBManager 而非 WiFiServer（用 mock 或 stub 替换子模块）。

- [ ] **Step 4: 运行测试确认通过**

```bash
go test ./internal/connection/... -run TestManager -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/connection/progress.go internal/connection/manager.go internal/connection/manager_test.go
git commit -m "feat: add progress broadcaster and ConnectionManager"
```

---

### Task 8: App.go — Wails 前后端绑定

**Files:**
- Modify: `app.go`
- Modify: `main.go`

**Interfaces:**
- Consumes: `connection.Manager`（Task 7）、`util.InitLogger`（Task 3）
- Produces: 以下方法通过 Wails 绑定暴露给 React 前端（均可直接在 TypeScript 中调用）
  - `App.ListDevices() []model.Device`
  - `App.GetFileList(deviceID, path string) ([]model.FileInfo, error)`
  - `App.Download(deviceID, remotePath, localPath string) error`
  - `App.Upload(deviceID, localPath, remotePath string) error`
  - `App.GetWiFiAddress() string`
  - `App.GetWiFiQRCode() string`
  - Wails 事件 `"transfer:progress"` 推送 `model.TransferProgress` JSON

- [ ] **Step 1: 在 app.go 中注入 Manager**

在 `App` 结构体添加 `manager *connection.Manager` 字段；`startup()` 中调用 `util.InitLogger()` 和 `manager.Start()`；`shutdown()` 中调用 `manager.Stop()`。

- [ ] **Step 2: 实现各绑定方法**

每个方法都是对 `a.manager` 的直接委托调用。`Download/Upload` 完成后通过 `runtime.EventsEmit(a.ctx, "transfer:progress", progress)` 向前端推送事件。

- [ ] **Step 3: 注册到 Wails**

在 `main.go` 的 `wails.Run()` 配置中，将 `app` 加入 `Bind` 列表；调用 `wails build` 后 Wails 会自动生成 `frontend/wailsjs/go/main/App.js` TypeScript bindings。

- [ ] **Step 4: 验证绑定生成**

```bash
wails dev
```
确认 `frontend/wailsjs/go/main/App.js` 中包含 `ListDevices`、`GetFileList`、`Download`、`Upload` 等方法。

- [ ] **Step 5: Commit**

```bash
git add app.go main.go
git commit -m "feat: wire App methods to Wails frontend bindings"
```

---

### Task 9: React 前端 UI 组件

**Files:**
- Modify: `frontend/src/App.tsx`
- Create: `frontend/src/components/DeviceList.tsx`
- Create: `frontend/src/components/FileBrowser.tsx`
- Create: `frontend/src/components/TransferQueue.tsx`
- Create: `frontend/src/components/QRCodeDisplay.tsx`
- Create: `frontend/src/hooks/useDevices.ts`
- Modify: `frontend/src/styles/app.css`

**Interfaces:**
- Consumes: Wails 生成的 `frontend/wailsjs/go/main/App.js` bindings（Task 8）
- Produces: 可运行的 Mac GUI，三栏布局（设备列表 / 文件浏览器 / 传输队列）

- [ ] **Step 1: 实现 useDevices.ts hook**

每 2 秒调用 `ListDevices()` 拉取设备列表，返回 `{ devices, loading, error }`。同时通过 `EventsOn("transfer:progress", ...)` 订阅传输进度事件，把进度列表作为 state 维护。

- [ ] **Step 2: 实现 DeviceList.tsx**

渲染左侧设备列表。每个设备显示类型图标（📶 WiFi / 🔌 ADB / 📱 MTP）、名称、连接状态。选中设备时高亮，并通知父组件 `onSelect(device)`。WiFi 设备点击时触发 `onShowQR(device)`。未检测到 ADB 时显示"安装提示"文字。

- [ ] **Step 3: 实现 QRCodeDisplay.tsx**

Modal 弹窗，接收 `{ address: string, qrCode: string }` props（分别来自 `GetWiFiAddress()` 和 `GetWiFiQRCode()`），展示二维码图片（`<img src={qrCode}>`）和文字链接，提供"复制链接"和"关闭"按钮。

- [ ] **Step 4: 实现 FileBrowser.tsx**

接收 `{ device: Device }` props。维护 `currentPath` state，调用 `GetFileList(device.ID, currentPath)` 获取文件列表，渲染目录树。支持：双击进入目录、单击文件触发 `Download()`（本地保存路径通过系统对话框选择，使用 Wails 的 `runtime.SaveFileDialog`）、顶部面包屑导航、"上传"按钮（`runtime.OpenFileDialog` 选文件后调用 `Upload()`）。

- [ ] **Step 5: 实现 TransferQueue.tsx**

订阅 `useDevices` 返回的 `progress[]` 列表，渲染底部进度条区域。每个传输任务显示文件名、目标设备、进度百分比（`bytesDone / totalBytes * 100`）、以及错误状态（`error` 非空时显示红色提示）。

- [ ] **Step 6: 组合 App.tsx**

三栏布局：左侧 `<DeviceList>`（宽度固定 220px）、右侧 `<FileBrowser>`（flex-grow）、底部 `<TransferQueue>`（固定高度 80px）。维护 `selectedDevice` state 传给 FileBrowser。

- [ ] **Step 7: 运行开发服务器验证**

```bash
wails dev
```
点击设备列表 → 文件列表出现；双击目录 → 进入子目录；点击文件 → 弹出保存路径对话框；WiFi 设备 → 二维码弹窗正确显示。

- [ ] **Step 8: Commit**

```bash
git add frontend/src/
git commit -m "feat: implement React frontend UI components"
```

---

### Task 10: 集成测试 + 打包

**Files:**
- Create: `README.md`

**Interfaces:**
- Consumes: 所有前述任务的产出

- [ ] **Step 1: 运行全量 Go 测试**

```bash
go test ./...
```
Expected: 所有测试 PASS，无 FAIL。

- [ ] **Step 2: WiFi 端到端手工测试**

启动 `wails dev`，用 Mac 本机浏览器打开显示的 URL，验证：文件列表正常显示、下载文件到本地成功、从 Mac 上传文件成功。

- [ ] **Step 3: ADB 端到端手工测试**

用 USB 连接 Android 手机，开启 USB 调试，在 App 中验证：设备自动出现在列表、文件列表显示 `/sdcard` 内容、pull 一个文件到 Mac 成功、push 一个文件到手机成功。

- [ ] **Step 4: 错误场景验证**

- 未安装 adb：App 显示安装提示，而非崩溃
- ADB 设备未授权：显示"请在手机上允许 USB 调试"提示
- WiFi 端口全占用（手动占用 8080-8084）：显示错误提示，而非崩溃

- [ ] **Step 5: 打包 universal binary**

```bash
wails build -platform darwin/universal
```
Expected: `build/bin/AndroidFileTransfer.app` 生成成功。

- [ ] **Step 6: 验证打包结果**

双击 `AndroidFileTransfer.app`，确认功能与 `wails dev` 一致。

- [ ] **Step 7: 编写 README.md**

记录：系统要求（macOS 12+，ADB 安装方法）、安装步骤、WiFi 使用说明（扫二维码）、ADB 使用说明（开启 USB 调试）。

- [ ] **Step 8: 最终 Commit**

```bash
git add README.md build/
git commit -m "feat: integration tests pass, build verified"
```

---

## 自检结果

- ✅ Spec 所有 P0 需求均有对应 Task（WiFi server、ADB、GUI、二维码、错误处理）
- ✅ 无 TBD / TODO 占位符
- ✅ 类型签名在各 Task 间一致（`model.Device`、`model.FileInfo`、`model.TransferProgress` 贯穿始终）
- ✅ MTP 为 P1，本计划不包含，符合 spec 的实现优先级
- ✅ 每个 Task 有独立测试步骤，可单独验收


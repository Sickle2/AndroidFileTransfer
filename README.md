# AndroidFileTransfer

一款 macOS 桌面应用，用于在 Mac 和 Android 手机之间传输文件，基于 [Wails](https://wails.io) 构建（Go 后端 + React/TypeScript 前端）。

支持两种传输方式，可同时使用：

- **WiFi**：手机在浏览器中打开一个网址（无需安装应用、无需数据线）来浏览/下载/上传文件到应用运行的本地 HTTP 服务器。
- **ADB（USB）**：启用 USB 调试后，可浏览手机的文件系统（`/sdcard`），并直接通过 USB 推送/拉取文件。

## 系统要求

- macOS 12 (Monterey) 或更新版本，Apple Silicon 或 Intel 处理器
- 若使用 USB 传输方式，需安装 [ADB](https://developer.android.com/tools/adb)（`android-platform-tools`）— 若仅使用 WiFi 传输则可选
- 使用 WiFi 传输时，需要 Android 手机和 Mac 连接到同一个本地网络

## 安装 ADB

ADB（USB）传输方式需要在系统中安装 Android 平台工具并将其添加到 `PATH` 中。在 macOS 上最简单的方法是使用 Homebrew：

```bash
brew install android-platform-tools
```

验证是否已安装：

```bash
adb version
```

如果未找到 `adb`，应用仍能正常运行并自动降级到仅 WiFi 模式 — ADB 设备不会出现在设备列表中。

## 构建和运行

```bash
# install frontend deps and build
cd frontend && npm install && npm run build && cd ..

# run in dev mode (hot reload)
wails dev

# build a production app bundle
wails build -platform darwin/universal
```

构建完成的应用位于 `build/bin/AndroidFileTransfer.app`。

## 使用 WiFi 传输

1. 确保你的 Mac 和 Android 手机已连接到同一个 WiFi 网络。
2. 启动应用。应用将启动一个本地 HTTP 服务器，并显示一个二维码以及形如 `http://192.168.1.x:8080` 的网址。
3. 在手机上扫描二维码（或在移动浏览器中手动打开该网址）。
4. 在手机浏览器中浏览 Mac 主目录中的文件：点击可下载到手机，或使用上传控件将手机上的文件发送到 Mac。

Android 手机上无需安装任何应用 — 一切都通过浏览器完成。

## 使用 ADB（USB）传输

1. 在 Android 手机上，启用开发者选项（设置 → 关于手机 → 连续点击"构建号" 7 次）。
2. 在开发者选项中，启用 **USB 调试**。
3. 使用 USB 数据线将手机连接到 Mac。
4. 首次连接时，手机会提示 **"允许 USB 调试？"** — 点击"允许"（并可选择"始终允许来自此计算机的连接"）。
5. 设备应会自动出现在应用的设备列表中。浏览 `/sdcard`，可从 Mac 拉取文件或推送文件到手机。

如果设备显示为"未授权"，请查看手机屏幕上是否有 USB 调试权限提示，并接受它。

## 故障排除

- **ADB 未安装**：应用会显示提示并以仅 WiFi 模式继续运行，而不会崩溃。
- **设备未授权**：在手机上接受"允许 USB 调试"的提示。
- **WiFi 端口不可用**：应用会尝试一个较小的端口范围（8080-8084）；如果所有端口都被占用，应用将报告错误而不是崩溃。请释放一个端口或重新启动应用。

## 开发

```bash
# run backend tests
go test ./...

# build the Go backend
go build ./...

# build the frontend
cd frontend && npm run build
```

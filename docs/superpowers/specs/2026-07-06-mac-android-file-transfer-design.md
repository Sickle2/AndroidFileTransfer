# Mac-Android 文件传输工具设计文档

**日期：** 2026-07-06  
**状态：** 待审核

## 概述

构建一个 Mac 原生 GUI 应用，支持 Mac 与 Android 设备之间的双向文件传输，通过 WiFi 和 USB 两种方式连接。

### 核心特性

- **Mac GUI 应用**：使用 Wails（Go + React）构建原生 macOS 桌面应用
- **WiFi 传输**：Mac 开启 HTTP 服务器，Android 通过浏览器访问，无需安装 App
- **USB 传输**：同时支持 ADB 和 MTP 协议
  - ADB：面向开发者，需开启 USB 调试，可访问完整文件系统
  - MTP：面向普通用户，手机选择"文件传输"模式即可
- **基础功能**：浏览目录、上传文件、下载文件
- **发布方式**：不上架 App Store，通过公证（Notarization）直接分发

### 技术选型

| 组件 | 技术 | 理由 |
|------|------|------|
| GUI 框架 | Wails v2 | Go + React，原生性能，打包成单个 .app |
| 后端语言 | Go | 熟悉，HTTP server 成熟，跨平台编译 |
| 前端框架 | React + TypeScript | Wails 默认支持，组件化开发 |
| HTTP 服务器 | Go `net/http` | 标准库，稳定可靠 |
| ADB 集成 | `os/exec` 调用 adb 命令 | 简单直接，无需重新实现协议 |
| MTP 集成 | `github.com/hanwen/go-mtpfs` | 成熟的 Go MTP 库 |
| 二维码生成 | `github.com/skip2/go-qrcode` | 简单易用 |

---

## 架构设计

### 整体架构

```
┌─────────────────────────────────────────────┐
│              Mac App (Wails)                │
│  ┌──────────────┐   ┌────────────────────┐  │
│  │  GUI 前端    │   │    Go 后端         │  │
│  │  (React)     │◄──┤  - HTTP Server     │  │
│  │  - 设备列表  │   │  - ADB Manager     │  │
│  │  - 文件浏览  │   │  - MTP Manager     │  │
│  │  - 传输进度  │   │  - WiFi Discovery  │  │
│  └──────────────┘   └────────┬───────────┘  │
└───────────────────────────────┼─────────────┘
                                │
          ┌─────────────────────┼──────────────┐
          │                     │              │
     WiFi (HTTP)           USB ADB        USB MTP
          │                     │              │
    Android 浏览器          adb binary      go-mtpfs
```

**设计原则：**
- 三个连接通道（WiFi、ADB、MTP）相互独立
- 通过统一的 `ConnectionManager` 协调
- GUI 层只看到一个设备列表，不关心底层传输协议

---

## 核心组件

### 1. ConnectionManager（连接管理器）

**职责：** 统一设备管理和文件操作接口

```go
type Device struct {
    ID       string  // 唯一标识（IP:Port / ADB serial / MTP device ID）
    Name     string  // 显示名称（手机型号）
    Type     string  // "wifi" | "adb" | "mtp"
    Status   string  // "connected" | "transferring" | "disconnected"
    Icon     string  // 设备图标（Android logo）
}

type FileInfo struct {
    Name      string
    Path      string
    Size      int64
    IsDir     bool
    ModTime   time.Time
}

type ConnectionManager interface {
    // 设备管理
    ListDevices() []Device
    RefreshDevices()
    
    // 文件操作
    GetFileList(deviceID, path string) ([]FileInfo, error)
    Download(deviceID, remotePath, localPath string) error
    Upload(deviceID, localPath, remotePath string) error
    
    // 进度通知
    SubscribeProgress() <-chan TransferProgress
}
```

**实现：** `internal/connection/manager.go`  
- 聚合 WiFiServer、ADBManager、MTPManager 三个子模块
- 根据 deviceID 前缀路由到对应的传输通道
- 管理传输队列和进度事件分发

### 2. WiFiServer（WiFi HTTP 服务器）

**职责：** 在 Mac 本地开启 HTTP 服务器，提供文件 API 和 Android 浏览器 UI

**接口设计：**

| 端点 | 方法 | 功能 |
|------|------|------|
| `/` | GET | 返回 Android 浏览器 UI（index.html） |
| `/api/files?path=<path>` | GET | 列出指定目录的文件 |
| `/api/download?path=<path>` | GET | 下载文件（stream） |
| `/api/upload` | POST | 上传文件（multipart/form-data） |

**实现细节：**
- 启动时自动检测可用端口（8080, 8081, ...，最多尝试 5 个）
- 获取本机 IP 地址，生成二维码（`http://<IP>:<Port>`）
- 前端 UI（HTML/CSS/JS）编译时嵌入到 Go 二进制（使用 `embed`）
- 支持大文件流式传输（避免内存爆炸）

**Android 浏览器 UI：**
- 单页应用，原生 HTML + Fetch API（不引入前端框架）
- 显示 Mac 文件系统目录树
- 支持多文件上传（input type="file" multiple）
- 实时显示上传/下载进度

### 3. ADBManager

**职责：** 通过 ADB 命令与 Android 设备通信

```go
type ADBManager struct {
    adbPath string  // adb 可执行文件路径
}

func (m *ADBManager) DetectDevices() []Device
func (m *ADBManager) ListFiles(serial, path string) ([]FileInfo, error)
func (m *ADBManager) Pull(serial, remotePath, localPath string) error
func (m *ADBManager) Push(serial, localPath, remotePath string) error
```

**实现：**
- 启动时检测 `adb` 命令是否存在（`exec.LookPath("adb")`）
- 未安装则提示用户：`brew install android-platform-tools`
- 每 2 秒轮询 `adb devices` 刷新设备列表
- 使用 `adb -s <serial> shell ls -la` 获取文件列表
- 使用 `adb pull/push` 执行文件传输
- 解析命令输出获取传输进度（通过 stderr 的百分比）

**错误处理：**
- 设备状态为 `unauthorized`：提示用户在手机上点击"允许 USB 调试"
- 设备状态为 `offline`：提示重新插拔 USB 线

### 4. MTPManager

**职责：** 通过 MTP 协议访问 Android 存储

```go
type MTPManager struct {
    devices map[string]*mtp.Device  // 已连接的 MTP 设备
}

func (m *MTPManager) DetectDevices() []Device
func (m *MTPManager) ListFiles(deviceID, path string) ([]FileInfo, error)
func (m *MTPManager) ReadFile(deviceID, path string) ([]byte, error)
func (m *MTPManager) WriteFile(deviceID, path string, data []byte) error
```

**实现：**
- 使用 `github.com/hanwen/go-mtpfs` 库
- 监听 USB 设备插拔事件（通过 `libusb` 或系统通知）
- 检测到 MTP 设备后自动挂载
- 提供文件读写接口（MTP 不支持 shell 命令，需直接操作文件）

**限制：**
- MTP 只能访问 `/sdcard` 等存储区域，无法访问系统文件
- 不支持符号链接
- 某些手机厂商的 MTP 实现有 bug（如小米、华为），需兼容处理

---

## 数据流

### WiFi 模式

**连接流程：**
1. Mac App 启动 HTTP Server（默认 8080 端口）
2. 显示本机 IP 和二维码（如 `http://192.168.1.100:8080`）
3. Android 扫描二维码或手动输入 URL
4. 浏览器打开页面，显示 Mac 文件系统

**文件传输流程：**

```
下载（Android ← Mac）：
  Android 浏览器 ──GET /api/download?path=/Users/...──► WiFiServer
                 ◄──────────stream file data──────────── WiFiServer

上传（Android → Mac）：
  Android 浏览器 ──POST /api/upload + multipart──► WiFiServer
                 ◄──────────{success: true}──────── WiFiServer
```

### ADB 模式

**连接流程：**
1. 后台轮询 `adb devices`（每 2 秒）
2. 检测到新设备，解析 serial 和型号
3. 添加到设备列表（类型标记为 "adb"）

**文件传输流程：**

```
下载（Mac ← Android）：
  GUI ──► ADBManager.Pull(serial, "/sdcard/file.txt", "/Users/...")
       ──► exec: adb -s <serial> pull /sdcard/file.txt /Users/...
       ──► 解析 stderr 进度输出，通过 channel 推送进度事件

上传（Mac → Android）：
  GUI ──► ADBManager.Push(serial, "/Users/file.txt", "/sdcard/...")
       ──► exec: adb -s <serial> push /Users/file.txt /sdcard/...
```

### MTP 模式

**连接流程：**
1. App 启动时初始化 libusb
2. 监听 USB 设备插入事件
3. 检测到 MTP 设备（Class 06h），尝试挂载
4. 添加到设备列表（类型标记为 "mtp"）

**文件传输流程：**

```
下载（Mac ← Android）：
  GUI ──► MTPManager.ReadFile(deviceID, "/sdcard/file.txt")
       ──► go-mtpfs: GetFile(objectID)
       ──► libusb bulk transfer
       ──► 返回文件数据

上传（Mac → Android）：
  GUI ──► MTPManager.WriteFile(deviceID, "/sdcard/file.txt", data)
       ──► go-mtpfs: SendFile(data)
       ──► libusb bulk transfer
```

---

## 错误处理

| 场景 | 检测方式 | 用户提示 | 恢复策略 |
|------|---------|---------|---------|
| ADB 未安装 | `exec.LookPath("adb")` 返回错误 | "未检测到 ADB，请运行：brew install android-platform-tools" | 提供"重新检测"按钮 |
| ADB 设备未授权 | `adb devices` 输出 `unauthorized` | "请在手机上允许 USB 调试" | 2 秒后自动重试 |
| MTP 设备未挂载 | libusb 枚举失败 | "请在手机下拉菜单选择'文件传输'模式" | 提供"重新扫描"按钮 |
| WiFi 端口占用 | `net.Listen()` 返回 "address already in use" | 自动尝试下一个端口（8081, 8082...） | 最多尝试 5 个端口后报错 |
| 传输中断 | 文件操作返回 IO error | "传输失败：<错误信息>，已传输 X MB" | 显示"重试"按钮（不做断点续传） |
| 网络断开（WiFi） | HTTP 请求超时 | "设备已断开连接" | 从设备列表移除 |
| USB 断开 | ADB/MTP 命令返回 device not found | "设备已断开连接" | 从设备列表移除 |

**日志记录：**
- 所有错误写入日志文件：`~/Library/Logs/AndroidFileTransfer/app.log`
- 使用 `log/slog` 包，日志级别：DEBUG、INFO、ERROR
- 包含时间戳、设备 ID、操作类型、错误堆栈

---

## 项目结构

```
android-file-transfer/
├── main.go                      # Wails 应用入口
├── app.go                       # Wails App 结构，绑定前端调用
├── wails.json                   # Wails 配置
├── go.mod
├── go.sum
│
├── internal/
│   ├── connection/
│   │   ├── manager.go           # ConnectionManager 实现 + 路由逻辑
│   │   ├── adb.go               # ADBManager 实现
│   │   ├── mtp.go               # MTPManager 实现
│   │   ├── wifi.go              # WiFiServer 实现
│   │   └── progress.go          # 进度事件定义和分发
│   │
│   ├── model/
│   │   └── types.go             # Device, FileInfo, TransferProgress 结构体
│   │
│   └── util/
│       ├── network.go           # 获取本机 IP、生成二维码
│       └── logger.go            # 日志工具
│
├── frontend/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── index.html
│   └── src/
│       ├── main.tsx             # React 入口
│       ├── App.tsx              # 主界面布局
│       ├── components/
│       │   ├── DeviceList.tsx   # 左侧设备列表
│       │   ├── FileBrowser.tsx  # 右侧文件浏览器
│       │   ├── TransferQueue.tsx # 底部传输队列
│       │   └── QRCodeDisplay.tsx # WiFi 模式二维码
│       ├── hooks/
│       │   └── useDevices.ts    # 设备状态管理
│       └── styles/
│           └── app.css
│
├── android-ui/                  # Android 浏览器端 UI（嵌入到二进制）
│   ├── index.html               # 单页应用入口
│   ├── style.css
│   └── app.js                   # 原生 JS，调用 /api/* 接口
│
├── build/                       # Wails 打包输出
│   └── bin/
│       └── AndroidFileTransfer.app
│
└── docs/
    └── superpowers/
        └── specs/
            └── 2026-07-06-mac-android-file-transfer-design.md  # 本文档
```

---

## GUI 设计

### 主窗口布局

```
┌────────────────────────────────────────────────┐
│  ○ ○ ○  Android File Transfer                  │
├──────────────┬─────────────────────────────────┤
│              │                                 │
│  设备列表    │         文件浏览器              │
│              │                                 │
│ 📱 WiFi      │  /Users/sickle/Downloads        │
│ 192.168.1.5  │  ┌─────────────────────────┐   │
│              │  │ 📁 folder1              │   │
│ 📱 ADB       │  │ 📁 folder2              │   │
│ Pixel 6      │  │ 📄 file.txt      1.2 MB │   │
│              │  │ 🖼️ image.jpg     3.5 MB │   │
│ 📱 MTP       │  └─────────────────────────┘   │
│ Samsung S21  │                                 │
│              │  [下载] [上传] [刷新]           │
│              │                                 │
│ [扫描设备]   │                                 │
├──────────────┴─────────────────────────────────┤
│ 传输队列：file.txt → Pixel 6  [████░░] 76%    │
└────────────────────────────────────────────────┘
```

**交互设计：**
- 左侧设备列表：点击切换当前设备，显示连接类型图标
- 右侧文件浏览器：双击目录进入，右键菜单（下载/上传/删除）
- 底部传输队列：实时显示进度，支持多任务并行
- WiFi 模式特殊 UI：点击设备显示二维码和 IP 地址

### WiFi 模式 - 二维码界面

点击 WiFi 设备时弹出：

```
┌────────────────────────────┐
│   WiFi 连接                 │
│                            │
│   ┌──────────────┐         │
│   │              │         │
│   │   [QR Code]  │         │
│   │              │         │
│   └──────────────┘         │
│                            │
│  在 Android 浏览器打开：     │
│  http://192.168.1.100:8080 │
│                            │
│  [复制链接]  [关闭]         │
└────────────────────────────┘
```

---

## 测试策略

### 单元测试

| 模块 | 测试内容 |
|------|---------|
| ADBManager | Mock `exec.Command`，测试设备检测、命令解析 |
| MTPManager | Mock libusb，测试设备枚举、文件读写 |
| WiFiServer | 启动测试服务器，验证 API 端点返回正确数据 |
| ConnectionManager | 测试路由逻辑、进度事件分发 |

### 集成测试

1. **WiFi 模式**：Mac 开启服务器，用另一台 Mac 的浏览器访问
2. **ADB 模式**：真机连接，开启 USB 调试，测试文件上传/下载
3. **MTP 模式**：真机连接，选择"文件传输"，测试文件操作
4. **设备切换**：同时连接多个设备，切换测试
5. **错误场景**：传输中拔线、网络中断、权限拒绝

### 手动测试清单

- [ ] WiFi：显示正确的本机 IP 和二维码
- [ ] WiFi：Android 浏览器能打开页面并显示文件列表
- [ ] WiFi：上传/下载文件成功，进度显示正确
- [ ] ADB：检测设备，显示正确型号
- [ ] ADB：未授权时显示提示
- [ ] ADB：pull/push 文件成功
- [ ] MTP：检测设备，显示存储容量
- [ ] MTP：读写文件成功
- [ ] 多设备：同时连接 WiFi + ADB + MTP，切换无卡顿
- [ ] 错误恢复：传输中拔线，UI 正确显示错误并恢复

---

## 发布流程

### 打包

```bash
# 1. 编译前端
cd frontend && npm run build

# 2. Wails 打包
wails build -platform darwin/universal

# 3. 输出路径
# build/bin/AndroidFileTransfer.app
```

### 签名与公证

```bash
# 1. 代码签名（需要 Apple Developer 证书）
codesign --deep --force --verify --verbose \
  --sign "Developer ID Application: Your Name" \
  AndroidFileTransfer.app

# 2. 打包成 DMG
create-dmg AndroidFileTransfer.app --output-dir ./dist

# 3. 公证（Notarization）
xcrun notarytool submit AndroidFileTransfer.dmg \
  --apple-id your@email.com \
  --password <app-specific-password> \
  --team-id <team-id> \
  --wait

# 4. Staple 公证票据
xcrun stapler staple AndroidFileTransfer.dmg
```

### 分发

- 上传到 GitHub Releases
- 提供 DMG 下载链接
- 编写 README：系统要求、安装步骤、使用说明

---

## 实现优先级

### MVP（最小可行产品）

**必须实现（P0）：**
- WiFi 模式：HTTP Server + 二维码 + Android 浏览器 UI
- ADB 模式：设备检测 + 文件列表 + pull/push
- GUI：设备列表 + 文件浏览器 + 基础传输

**可延后（P1）：**
- MTP 模式（技术复杂度高，用户需求相对低）
- 传输进度显示（先做基础功能，后优化体验）
- 错误恢复（先 happy path，后 edge case）

### 开发顺序

1. **第 1-2 天**：搭建 Wails 项目，完成 GUI 基础布局
2. **第 3-4 天**：实现 WiFi Server + Android 浏览器 UI
3. **第 5-6 天**：实现 ADB Manager，完成设备检测和文件传输
4. **第 7 天**：集成测试，修复 bug，优化 UI
5. **第 8-9 天**（可选）：实现 MTP 模式
6. **第 10 天**：打包、签名、公证、发布

---

## 已知限制

1. **WiFi 模式安全性**：HTTP 明文传输，局域网内可被嗅探。V1 不做 HTTPS，后续可考虑自签名证书。
2. **断点续传**：V1 不支持，传输中断需重新开始。后续可记录 offset 实现续传。
3. **MTP 兼容性**：部分手机厂商（小米、华为）的 MTP 实现有 bug，可能需特殊处理。
4. **ADB 依赖**：用户需自行安装 ADB，App 不内置（避免二进制体积过大）。
5. **多语言**：V1 只支持中文，后续可通过 i18n 支持英文。

---

## 后续扩展

**V2 可能的功能：**
- 剪贴板同步（WiFi 模式通过 WebSocket 实时同步）
- 文件预览（图片、视频在浏览器内直接查看）
- 传输历史记录
- 批量操作（多选文件）
- 拖拽上传（Mac 文件拖到 GUI 自动上传）
- WiFi 自动发现（mDNS/Bonjour，手机无需扫码）

---

## 总结

本设计文档定义了一个基于 Go + Wails 的 Mac-Android 文件传输工具，支持 WiFi、ADB、MTP 三种连接方式，满足从普通用户到开发者的不同需求。技术栈选择考虑了开发效率、性能和可维护性，项目结构清晰，具备可扩展性。MVP 聚焦核心功能（WiFi + ADB），确保快速交付，后续可逐步增强。

# WiFi 共享范围控制实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 将 WiFi 模式从默认暴露 Mac 主目录改为可控共享范围：默认仅显示 Mac 端手动共享的文件/文件夹，手机端不暴露真实路径、不显示隐藏内容，上传统一进入接收目录。

**Architecture:** 在 Go 后端新增 ShareConfig/ShareManager，WiFiServer 的 `/api/files`、`/api/download`、`/api/upload` 不再直接使用真实路径，而是通过 ShareManager 解析虚拟路径。Wails 前端 WiFi 面板增加共享模式切换、共享列表管理、接收目录设置；Android 浏览器端继续调用现有 API，但只处理虚拟路径。

**Tech Stack:** Go 1.24.6, Wails v2, React 18, TypeScript 5, Vite, Go stdlib testing/httptest

## Global Constraints

- 默认共享模式：`selected`（仅共享拖拽文件/文件夹）。
- 高级模式：`directory`（共享整个文件夹），必须用户主动切换。
- 手机端永远不返回真实 Mac 绝对路径。
- 任意以 `.` 开头的文件或文件夹均不可见、不可访问、不可共享、不可上传。
- 手机上传统一进入接收目录，默认 `~/Downloads/AndroidFileTransfer`。
- 配置保存到 `~/Library/Application Support/AndroidFileTransfer/config.json`。
- 用户可见文字使用中文。
- 所有 Go 命令使用：`GOROOT=/Users/sickle/go/go1.24.6 GOPROXY=https://goproxy.cn PATH="/Users/sickle/go/go1.24.6/bin:$PATH" go ...`
- 使用 codegraph 进行代码定位和影响分析，修改前优先用 `codegraph_explore` 查看相关符号。

---

## 文件结构变化

| 文件 | 责任 |
|------|------|
| `internal/model/share.go` | ShareMode、SharedItem、ShareConfig 等共享范围类型 |
| `internal/connection/share_manager.go` | 共享配置加载/保存、虚拟路径解析、隐藏文件过滤、上传文件名校验 |
| `internal/connection/share_manager_test.go` | ShareManager 路径安全、虚拟路径、隐藏文件、配置持久化测试 |
| `internal/connection/wifi.go` | WiFi HTTP handler 改为通过 ShareManager 处理 files/download/upload |
| `internal/connection/wifi_test.go` | 更新 WiFi API 测试，覆盖虚拟路径和隐藏文件过滤 |
| `internal/connection/manager.go` | Manager 持有 ShareManager，并暴露共享配置相关方法 |
| `app.go` | Wails 绑定新增 Get/Set share config、添加/移除共享项等方法 |
| `frontend/src/App.tsx` | WiFi 面板从二维码-only 扩展为共享设置面板 |
| `frontend/src/components/WiFiSharePanel.tsx` | 新增 WiFi 共享范围 UI：模式切换、共享列表、接收目录、二维码 |
| `frontend/src/styles/app.css` | WiFiSharePanel 样式 |
| `android-ui/app.js` | 适配虚拟路径，默认根路径 `/`，不假设真实路径 |

---

### Task 1: 定义共享模型与 ShareManager 骨架

**Files:**
- Create: `internal/model/share.go`
- Create: `internal/connection/share_manager.go`
- Create: `internal/connection/share_manager_test.go`

**Interfaces:**
- Produces: `model.ShareMode`, `model.SharedItem`, `model.ShareConfig`
- Produces: `NewShareManager() (*ShareManager, error)`, `Config() model.ShareConfig`, `Save() error`

- [ ] 定义 `ShareModeSelected = "selected"`、`ShareModeDirectory = "directory"`。
- [ ] 定义 `SharedItem`，真实 `Path` 带 `json:"-"`，防止返回给手机端。
- [ ] 定义 `ShareConfig`，包含 `Mode`、`RootDir`、`UploadDir`、`SharedItems`。
- [ ] 实现默认配置：mode=selected，rootDir=`~/Downloads`，uploadDir=`~/Downloads/AndroidFileTransfer`，sharedItems=[]。
- [ ] 实现配置路径：`~/Library/Application Support/AndroidFileTransfer/config.json`。
- [ ] 实现加载配置；配置不存在时创建默认配置并保存。
- [ ] 实现保存配置。
- [ ] 写测试覆盖：默认配置、保存后重新加载、真实 Path 不会 JSON 输出给手机端。
- [ ] 运行 `go test ./internal/connection/... ./internal/model/...`。
- [ ] Commit: `feat: add WiFi share config model and manager`

---

### Task 2: 实现虚拟路径解析与隐藏文件安全规则

**Files:**
- Modify: `internal/connection/share_manager.go`
- Modify: `internal/connection/share_manager_test.go`

**Interfaces:**
- Produces: `AddSharedPaths(paths []string) error`
- Produces: `RemoveSharedItem(id string) error`
- Produces: `ClearSharedItems() error`
- Produces: `ResolveVirtualPath(vpath string) (realPath string, info os.FileInfo, err error)`
- Produces: `ListVirtualDir(vpath string) ([]model.FileInfo, error)`
- Produces: `ValidateUploadName(name string) error`

- [ ] 实现 `isHiddenName(name string) bool`：任意 path segment 以 `.` 开头即隐藏。
- [ ] `AddSharedPaths`：拒绝隐藏文件/文件夹；生成稳定唯一 ID（如 `item-<timestamp/hash>`）；自动保存配置。
- [ ] `RemoveSharedItem` / `ClearSharedItems`：修改列表并保存。
- [ ] selected 模式根路径 `/` 或空路径返回共享项列表，路径为 `/shared/<id>`。
- [ ] selected 模式 `/shared/<id>` 映射到共享文件/文件夹；文件夹子路径必须限制在该文件夹内。
- [ ] directory 模式根路径 `/` 返回 rootDir 列表，路径为 `/browse/<relative>`。
- [ ] directory 模式 `/browse/...` 映射到 rootDir 内真实路径。
- [ ] 所有列表结果过滤隐藏项，所有直接解析隐藏路径返回 403 类错误。
- [ ] 所有 `model.FileInfo.Path` 返回虚拟路径，不包含 `/Users/...`。
- [ ] `ValidateUploadName` 拒绝空名、包含 `/` 或 `\`、以 `.` 开头、clean 后变化的文件名。
- [ ] 写测试覆盖：虚拟路径不泄露真实路径、隐藏项过滤、直接访问隐藏项失败、`..` 越界失败、上传文件名校验。
- [ ] 运行 `go test ./internal/connection/...`。
- [ ] Commit: `feat: add virtual WiFi share path resolver`

---

### Task 3: WiFiServer 接入 ShareManager

**Files:**
- Modify: `internal/connection/wifi.go`
- Modify: `internal/connection/wifi_test.go`
- Modify: `internal/connection/manager.go`

**Interfaces:**
- WiFiServer 新增 `shareMgr *ShareManager`。
- `NewWiFiServer` 或 setter 接收 ShareManager。
- `/api/files`、`/api/download`、`/api/upload` 全部通过 ShareManager。

- [ ] 修改 WiFiServer 构造/字段，让它持有 ShareManager。
- [ ] `handleFiles` 调用 `shareMgr.ListVirtualDir(path)`，返回虚拟路径 JSON。
- [ ] `handleDownload` 调用 `shareMgr.ResolveVirtualPath(path)`，只允许下载文件，不允许下载目录。
- [ ] `handleUpload` 忽略客户端 path，使用 `shareMgr.UploadDir()`，校验文件名后保存到接收目录。
- [ ] 上传目录不存在时自动创建。
- [ ] 删除或停用旧的 `resolvePath/rootDir` 真实路径暴露逻辑，保留必要兼容测试时要明确不会返回真实路径。
- [ ] 更新 WiFi tests：selected 模式默认空列表、加入共享项后可列出、下载使用虚拟路径、上传进入 uploadDir、隐藏文件不可见不可下载、JSON 不含真实路径。
- [ ] 运行 `go test ./internal/connection/...`。
- [ ] Commit: `feat: route WiFi HTTP APIs through share manager`

---

### Task 4: Wails 后端绑定共享配置方法

**Files:**
- Modify: `internal/connection/manager.go`
- Modify: `app.go`
- Modify: `main.go`（如构造参数需要调整）

**Interfaces:**
- Produces bound methods:
  - `GetShareConfig() model.ShareConfig`
  - `SetShareMode(mode string) error`
  - `SetRootDir(path string) error`
  - `AddSharedPaths(paths []string) error`
  - `RemoveSharedItem(id string) error`
  - `ClearSharedItems() error`
  - `SetUploadDir(path string) error`

- [ ] Manager 持有 `*ShareManager` 并暴露委托方法。
- [ ] App 绑定新增上述方法。
- [ ] 保持 `GetWiFiAddress` / `GetWiFiQRCode` 现有行为不变。
- [ ] rootDir 选择和 uploadDir 选择仅保存配置，不直接泄露给手机端。
- [ ] 运行 `go build ./...`。
- [ ] 运行 `wails generate` 或 `wails build` 以更新 `frontend/wailsjs` 绑定。
- [ ] Commit: `feat: expose WiFi share configuration bindings`

---

### Task 5: Mac 前端 WiFiSharePanel

**Files:**
- Create: `frontend/src/components/WiFiSharePanel.tsx`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/styles/app.css`
- Modify: `frontend/wailsjs/...`（由 Wails 生成）

**Interfaces:**
- Consumes Wails bindings from Task 4。
- Produces WiFi 面板：二维码 + 共享模式 + 共享列表 + 接收目录设置。

- [ ] 新建 `WiFiSharePanel`，接收 QR 地址/二维码或内部自行调用 `GetWiFiAddress/GetWiFiQRCode`。
- [ ] 展示模式切换：默认 selected，高级 directory。
- [ ] selected 模式展示共享列表、移除、清空。
- [ ] 提供“添加文件”“添加文件夹”按钮作为 MVP，调用 Wails 文件/目录选择能力后 `AddSharedPaths`。
- [ ] 若能在 Wails/macOS 下可靠获取拖拽真实路径，则增加拖拽区域；否则保留按钮并在 UI 文案中写“拖拽支持后续增强”。
- [ ] directory 模式展示 rootDir 和“选择文件夹”。
- [ ] 展示 uploadDir 和“选择接收目录”。
- [ ] 切换到 directory 模式时展示一次中文风险提示。
- [ ] App.tsx 中 WiFi 设备选中时渲染 `WiFiSharePanel`，非 WiFi 设备继续 FileBrowser。
- [ ] 所有用户可见文案中文。
- [ ] 运行 `cd frontend && npm run build`。
- [ ] Commit: `feat: add WiFi share scope panel`

---

### Task 6: Android 浏览器 UI 适配虚拟路径与上传规则

**Files:**
- Modify: `android-ui/app.js`
- Modify: `android-ui/index.html`（如需要）
- Modify: `android-ui/style.css`（如需要）

**Interfaces:**
- Consumes unchanged API `/api/files`、`/api/download`、`/api/upload`，但 path 全是虚拟路径。

- [ ] 默认 `loadFiles('/')` 或空路径，且不要假设真实路径。
- [ ] 面包屑基于虚拟路径构建，不显示真实路径。
- [ ] selected 模式首页标题可显示“共享文件”。
- [ ] directory 模式首页可显示“共享文件夹”。
- [ ] 下载使用虚拟路径。
- [ ] 上传不再把当前浏览路径当作保存路径；仍可传 path 但后端会忽略，前端文案说明“上传到 Mac 接收目录”。
- [ ] 上传失败时展示后端错误，特别是隐藏文件名/非法文件名拒绝。
- [ ] 保留现有 XSS 防护和 fetch ok 检查。
- [ ] 用 `node --check android-ui/app.js` 验证语法。
- [ ] Commit: `feat: adapt Android UI to virtual WiFi paths`

---

### Task 7: 端到端安全验证与文档更新

**Files:**
- Modify: `README.md`
- Modify: `docs/superpowers/specs/2026-07-08-wifi-share-scope-design.md`（如实现中有小调整）
- Optional: add focused integration tests under `internal/connection/`

**Validation:**
- Go tests: `go test ./...`
- Vet: `go vet ./...`
- Go build: `go build ./...`
- Frontend build: `cd frontend && npm run build`
- Wails build: `wails build -platform darwin/universal`

- [ ] 增加 README 中 WiFi 共享范围说明：默认仅共享拖拽/添加项，高级模式共享整个文件夹，上传进入接收目录。
- [ ] 验证 API 不返回真实 `/Users/...` 路径。
- [ ] 验证隐藏文件不可见、不可下载、不可上传。
- [ ] 验证 selected 模式空列表时手机端显示友好提示。
- [ ] 验证 directory 模式仍过滤隐藏项。
- [ ] 验证配置重启后保留（可通过启动/加载 ShareManager 测试覆盖）。
- [ ] 运行全部验证命令。
- [ ] Commit: `chore: verify WiFi share scope and update docs`

---

## Review Checklist

- 手机端 JSON 中不出现真实 Mac 路径。
- 所有 `.` 开头 path segment 均不可见且不可直接访问。
- 上传只能进 uploadDir。
- directory 模式需要用户主动切换。
- selected 模式是默认。
- Wails 绑定和 TS 类型匹配。
- 现有 ADB 功能不受影响。

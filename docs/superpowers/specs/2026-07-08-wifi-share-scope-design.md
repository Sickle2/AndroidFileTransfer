# WiFi 共享范围控制设计文档

**日期：** 2026-07-08  
**状态：** 已确认，待实现

## 背景

当前 WiFi 模式下，手机扫码后会进入 Mac 端 WiFi Server 暴露的根目录。现有默认根目录是用户主目录，手机端可以看到较大范围的本机文件夹。这个行为适合早期调试，但不适合作为默认产品行为，因为它会暴露过多本机文件结构。

本次改造目标是让 Mac 端明确控制手机端可见范围，并把默认行为改成更安全的“仅共享拖拽文件/文件夹”。

---

## 目标

WiFi 模式新增共享范围控制：

1. 默认模式：仅共享 Mac 端手动拖拽的文件/文件夹。
2. 手机端只看到拖拽共享内容。
3. 手机端不能看到或访问任何以 `.` 开头的隐藏文件/文件夹。
4. 手机上传文件统一进入 Mac 端接收目录。
5. “共享整个文件夹”作为高级模式，需要用户主动切换。
6. 手机端永远不暴露 Mac 真实绝对路径。

---

## 非目标

本次不做：

- WiFi 访问认证。
- HTTPS。
- 断点续传。
- MTP 模式。
- 多用户权限系统。
- 手机端编辑/删除 Mac 文件。

---

## 产品行为

### 默认模式：仅共享拖拽文件/文件夹

应用启动后，WiFi 共享范围默认是“仅共享拖拽文件/文件夹”。

手机扫码后：

- 首页只显示用户在 Mac 端拖入共享列表的项目。
- 不显示 Mac 用户主目录。
- 不显示未共享文件。
- 不返回真实 Mac 路径。
- 不能通过构造路径访问共享范围外内容。

Mac 端允许拖入：

- 文件。
- 文件夹。
- 多个文件/文件夹。

Mac 端拒绝拖入：

- 以 `.` 开头的隐藏文件。
- 以 `.` 开头的隐藏文件夹。

拒绝提示：

```text
隐藏文件或隐藏文件夹不能共享
```

### 共享文件夹内部隐藏内容

如果用户拖入一个普通文件夹，该文件夹可以加入共享列表。但手机端浏览其内部内容时，必须过滤所有以 `.` 开头的文件或文件夹。

示例：

```text
Photos/
  a.jpg
  .secret.jpg
  .private/
```

手机端只看到：

```text
a.jpg
```

不能看到或访问：

```text
.secret.jpg
.private/
```

### 高级模式：共享整个文件夹

用户可以主动切换到“共享整个文件夹”模式。

Mac 端显示：

```text
共享整个文件夹（高级）
当前共享目录：/path/to/folder
[选择文件夹]
```

切换到该模式时显示风险提示：

```text
该模式会允许手机浏览所选文件夹下的内容，请确认当前网络可信。
```

该模式下仍然必须：

- 限制访问范围在 rootDir 内。
- 过滤隐藏文件和隐藏文件夹。
- 不向手机端暴露真实绝对路径。

### 上传规则

手机上传文件统一进入接收目录。

默认接收目录：

```text
~/Downloads/AndroidFileTransfer
```

规则：

- 上传目录不存在时自动创建。
- 手机端不能指定任意 Mac 路径。
- 手机端不能上传到浏览中的任意目录。
- 上传文件名不能包含路径分隔符。
- 上传文件名不能以 `.` 开头。
- 上传 `../../x.txt` 等路径穿越文件名时拒绝。

---

## 路径模型

现有 WiFi API 需要从“暴露真实路径”改为“虚拟路径”。

### 模式

```go
type ShareMode string

const (
    ShareModeSelected  ShareMode = "selected"
    ShareModeDirectory ShareMode = "directory"
)
```

### 共享项

```go
type SharedItem struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Path  string `json:"-"`
    IsDir bool   `json:"isDir"`
    Size  int64  `json:"size"`
}
```

`Path` 是真实 Mac 路径，只能保存在本机配置中，不能返回给手机端。

### 配置

```go
type ShareConfig struct {
    Mode        ShareMode    `json:"mode"`
    RootDir     string       `json:"rootDir"`
    UploadDir   string       `json:"uploadDir"`
    SharedItems []SharedItem `json:"sharedItems"`
}
```

配置保存在：

```text
~/Library/Application Support/AndroidFileTransfer/config.json
```

---

## 虚拟路径规则

### 仅共享拖拽模式

首页路径：

```text
/
```

返回共享项：

```text
/shared/item-1
/shared/item-2
```

如果 `item-2` 是文件夹，进入其中的文件：

```text
/shared/item-2/a.jpg
/shared/item-2/subdir/b.pdf
```

后端解析：

```text
/shared/item-2/subdir/b.pdf
→ item-2 对应真实路径 + subdir/b.pdf
```

必须检查：

- `item-2` 存在。
- `item-2` 是文件夹时才允许进入子路径。
- 解析后的路径仍在 item-2 的真实目录内。
- 任意路径段不能以 `.` 开头。
- 真实目标不能是隐藏文件/隐藏文件夹。

### 共享整个文件夹模式

首页路径：

```text
/
```

内部虚拟路径：

```text
/browse/Documents/a.pdf
/browse/Photos/a.jpg
```

后端解析：

```text
/browse/Documents/a.pdf
→ rootDir/Documents/a.pdf
```

必须检查：

- 解析后的路径在 rootDir 内。
- 任意路径段不能以 `.` 开头。
- 真实目标不能是隐藏文件/隐藏文件夹。

---

## API 行为

继续保留现有 API：

```text
GET  /api/files?path=...
GET  /api/download?path=...
POST /api/upload
```

### GET /api/files

默认 `path` 为空或 `/` 时表示虚拟根。

#### 拖拽模式首页返回

```json
[
  {
    "name": "report.pdf",
    "path": "/shared/item-1",
    "size": 123456,
    "isDir": false
  },
  {
    "name": "Photos",
    "path": "/shared/item-2",
    "size": 0,
    "isDir": true
  }
]
```

#### 共享整个文件夹模式返回

```json
[
  {
    "name": "Documents",
    "path": "/browse/Documents",
    "size": 0,
    "isDir": true
  }
]
```

### GET /api/download

手机端传虚拟路径，后端解析为真实路径并下载。

如果路径非法、越界、隐藏、未共享，返回 403。

### POST /api/upload

上传统一进入 `UploadDir`。

忽略客户端传入的浏览路径，不允许手机端指定真实保存路径。

上传成功返回：

```json
{"success":true}
```

上传失败返回对应 HTTP 错误。

---

## Mac 端 UI

WiFi 面板增加共享设置区域。

### 仅共享拖拽模式

```text
共享范围
(●) 仅共享拖拽文件/文件夹
( ) 共享整个文件夹（高级）

将文件或文件夹拖到这里
手机扫码后只能看到这些内容

已共享 3 项
📄 report.pdf      [移除]
📁 Photos          [移除]
📦 archive.zip     [移除]

手机上传接收目录：
~/Downloads/AndroidFileTransfer
[选择接收目录]
```

### 共享整个文件夹模式

```text
共享范围
( ) 仅共享拖拽文件/文件夹
(●) 共享整个文件夹（高级）

当前共享目录：
/path/to/folder
[选择文件夹]

手机上传接收目录：
~/Downloads/AndroidFileTransfer
[选择接收目录]
```

### 二维码展示

保留当前 WiFi 面板中的二维码和地址。

---

## Wails 绑定方法

新增：

```go
GetShareConfig() ShareConfig
SetShareMode(mode string) error
SetRootDir(path string) error
AddSharedPaths(paths []string) error
RemoveSharedItem(id string) error
ClearSharedItems() error
SetUploadDir(path string) error
```

前端需要使用 Wails 文件/目录选择能力实现：

- 添加共享文件。
- 添加共享文件夹。
- 选择完整共享目录。
- 选择上传接收目录。

拖拽文件/文件夹如果 Wails 无法直接拿到真实路径，则先使用按钮选择文件/文件夹作为 MVP；拖拽作为增强项。实现时需要验证当前 Wails 版本在 macOS 下能否从 drop event 获取文件路径。

---

## 持久化

配置保存到：

```text
~/Library/Application Support/AndroidFileTransfer/config.json
```

默认配置：

```json
{
  "mode": "selected",
  "rootDir": "~/Downloads",
  "uploadDir": "~/Downloads/AndroidFileTransfer",
  "sharedItems": []
}
```

启动时加载配置。

如果共享项真实路径不存在：

- Mac 端列表中标记“文件不存在”，或自动移除。
- MVP 建议自动移除并记录日志，减少手机端无效项。

---

## 安全规则

### 隐藏路径判断

任意路径段只要以 `.` 开头，即认为隐藏。

例如：

```text
.env
.git/config
Photos/.private/a.jpg
```

均禁止。

### 文件名规则

上传文件名必须满足：

- 非空。
- 不包含 `/` 或 `\`。
- 不以 `.` 开头。
- 清理后仍等于原文件名。

否则拒绝。

### 真实路径边界

任何真实路径解析后必须满足：

- 拖拽模式：在对应共享文件夹内，或等于共享文件本身。
- 完整文件夹模式：在 rootDir 内。
- 不允许 `..` 越界。

---

## 验收标准

### 拖拽模式

- 默认就是拖拽模式。
- 手机端首页只显示拖拽共享项。
- 手机端 JSON 不包含 `/Users/...`。
- 请求 `/api/files?path=/Users/sickle` 返回 403。
- 请求 `/api/download?path=../../secret.txt` 返回 403。
- 请求隐藏文件 `/shared/item-1/.env` 返回 403。
- 拖入 `.env` 时 Mac 端拒绝共享。

### 共享文件夹模式

- 用户主动切换后才能启用。
- 手机端只能浏览用户选择的 rootDir。
- 手机端看不到 `.DS_Store`、`.git` 等隐藏项。
- 请求 rootDir 外路径返回 403。
- API 不返回真实绝对路径。

### 上传

- 手机上传文件进入接收目录。
- 上传 `.env` 被拒绝。
- 上传 `../../x.txt` 被拒绝。
- 上传成功后 Mac 接收目录能看到文件。

### 持久化

- 退出应用后重启，共享模式、共享项、接收目录仍保留。
- 不存在的共享项自动移除或不显示给手机端。

---

## 风险与实现注意点

1. Wails 前端拖拽是否能拿到真实路径需要验证。若拿不到，MVP 改用“添加文件/添加文件夹”按钮。
2. 共享文件夹内部过滤隐藏项必须同时应用在列表和下载接口，不能只隐藏列表。
3. 手机端路径不能再使用真实路径，需要同步改 Android UI 的路径处理。
4. 配置文件里保存真实路径是本机行为，不能通过 API 泄漏。
5. 共享整个文件夹模式虽然高级，但也不能返回真实绝对路径。

# MostFileViewer 新建窗口功能 — 详细设计文档

---

## 1. 需求描述

### 1.1 功能目标

在标题栏「文件」下拉菜单中新增「新建窗口」选项，点击后：

1. 打开一个新窗口，展示首页（空状态 hero 页面）
2. 用户可在新窗口中选择文件或文件夹进行预览
3. 当用户在新窗口中打开的**文件或文件夹**已经在**其他窗口**中打开时：
   - 关闭新窗口
   - 聚焦并显示原来已打开该路径的窗口

### 1.2 交互流程

```
用户点击「文件」→「新建窗口」
        │
        ▼
  后端创建新窗口 (URL: "/", 展示首页)
        │
        ▼
  用户在新窗口中选择文件/文件夹 或 拖入文件/文件夹
        │
        ▼
  前端调用 CheckPathOpened(path) 检查路径是否已在其他窗口打开
        │
        ├── 未打开 ──→ 正常加载预览，注册路径
        │
        └── 已打开 ──→ FocusWindow(原窗口ID) + CloseCurrentWindow()
```

### 1.3 约束条件

- 多窗口之间**状态隔离**：一个窗口打开的文件夹路径不影响另一个窗口的文件访问校验
- 文件拖放事件**按窗口分发**：在窗口 A 拖入文件，只有窗口 A 响应
- macOS 上关闭一个窗口不应退出整个应用（需调整 `ApplicationShouldTerminateAfterLastWindowClosed`）
- 窗口关闭时需清理后端注册的状态，避免内存泄漏和去重误判

---

## 2. 现有架构分析

### 2.1 后端结构

**`main.go`**：

- 创建单个窗口，注册文件拖放事件
- 拖放事件通过全局 `application.Get().Event.Emit("files-dropped", items)` 广播给所有窗口
- macOS 设置 `ApplicationShouldTerminateAfterLastWindowClosed: true`

**`app.go`**：

- `App` 结构体持有全局共享状态：

```go
type App struct {
    currentRoot  string              // 当前打开的根目录（全局唯一）
    allowedFiles map[string]struct{} // 允许访问的文件白名单（全局唯一）
}
```

- 所有文件操作方法（`LoadFolderTree`、`ReadFile`、`SaveFile` 等）直接读写 `a.currentRoot`
- 路径校验 `isPathWithinRoot(a.currentRoot, cleanPath)` 依赖全局 `currentRoot`
- service 方法**未接收** `context.Context` 参数，无法区分调用来源窗口

### 2.2 前端结构

**`TitleBar.vue`**：

- 文件下拉菜单有「选择文件」「选择文件夹」两个选项
- 通过 `emit` 向父组件 `App.vue` 传递事件
- 窗口控制（最小化/最大化/关闭）通过 `@wailsio/runtime` 的 `Window` API

**`App.vue`**：

- `selectedFolder` 为空时展示 hero 首页
- `handleSelectFile` / `handleSelectFolder` / `handleFilesDropped` 处理文件打开
- 监听全局 `files-dropped` 事件

### 2.3 Wails v3 多窗口能力

| 能力 | 支持情况 | 说明 |
| --- | --- | --- |
| 创建多窗口 | ✅ | `app.Window.NewWithOptions(options)` |
| 窗口命名与获取 | ✅ | `WindowManager.GetByName(name)` / `GetByID(id)` |
| 窗口聚焦/关闭 | ✅ | `win.Focus()` / `win.Close()` / 前端 `Window.Get(name).Focus()` |
| Service 方法获取调用窗口 | ✅ | 方法首参为 `context.Context` 时，框架注入 `WindowKey` |
| 按窗口发送事件 | ✅ | `win.EmitEvent(name, data)` 仅对该窗口发送 |

---

## 3. 架构设计

### 3.1 总体架构

```
┌──────────────────────────────────────────────────────────┐
│                      Go Backend                           │
│                                                           │
│  App (单例 Service)                                       │
│  ├── states:     sync.Map[windowID → *windowState]       │
│  ├── pathOwners: sync.Map[canonicalPath → windowID]      │
│  │                                                        │
│  ├── 文件操作方法 (ctx context.Context, ...)              │
│  │   └── 从 ctx 提取 windowID → 取对应 windowState        │
│  │                                                        │
│  ├── NewWindow()                  → 创建新窗口            │
│  ├── RegisterOpenPath(ctx, path)  → 注册当前窗口打开的路径 │
│  ├── CheckPathOpened(ctx, path)   → 查询路径是否已被打开   │
│  ├── FocusWindow(windowID)        → 聚焦指定窗口          │
│  └── cleanupWindowState(windowID) → 窗口关闭时清理状态     │
│                                                           │
├──────────────────────────────────────────────────────────┤
│                  Wails Bindings (自动生成)                 │
├──────────────────────────────────────────────────────────┤
│                     Vue Frontend                          │
│                                                           │
│  TitleBar.vue                                             │
│  └── 文件菜单 → 新建窗口 (emit "new-window")              │
│                                                           │
│  App.vue                                                  │
│  ├── handleNewWindow() → App.NewWindow()                  │
│  ├── handleSelectFile()                                   │
│  │   └── 打开前去重检查 → 注册路径                         │
│  ├── handleSelectFolder()                                 │
│  │   └── 打开前去重检查 → 注册路径                         │
│  └── handleFilesDropped()                                 │
│      └── 打开前去重检查 → 注册路径                         │
└──────────────────────────────────────────────────────────┘
```

### 3.2 核心设计决策

#### 决策 1：状态隔离方案 — per-window state map

**问题**：`currentRoot` 和 `allowedFiles` 全局共享，多窗口会互相覆盖。

**方案**：将状态按窗口 ID 隔离存储：

```go
type windowState struct {
    currentRoot  string
    allowedFiles map[string]struct{}
    openPaths    map[string]struct{} // 当前窗口已打开的文件/文件夹路径集合
    mu           sync.Mutex
}

type App struct {
    states     sync.Map // windowID(uint) → *windowState
    pathOwners sync.Map // canonicalPath(string) → windowID(uint)
}
```

所有文件操作方法增加 `ctx context.Context` 首参，从中提取 `WindowKey` 获取窗口 ID，再取对应的 `windowState`。

#### 决策 2：去重粒度

| 场景 | 去重 key | 说明 |
| --- | --- | --- |
| 打开文件夹 | 文件夹绝对路径 | 两个窗口不能打开同一文件夹 |
| 打开单文件 | 文件绝对路径 | 两个窗口不能打开同一文件 |
| 已有工作区添加 tab | 文件绝对路径 | 需检查是否已在其他窗口打开；同一窗口内已打开则只切换 tab |

窗口打开路径登记规则：

- 文件夹模式：登记文件夹路径
- 单文件模式：登记文件路径本身（不是父目录）
- 多 tab 模式：每个已打开 tab 的文件路径都登记到当前窗口
- 关闭 tab 或窗口回到首页时，需要注销对应路径，避免后续去重误判

后端维护两类数据：

- `windowState.openPaths`：当前窗口拥有的路径集合，用于窗口关闭时批量清理
- `pathOwners`：全局路径索引，用于快速判断某个 canonical path 是否已被其他窗口占用

#### 决策 3：去重检查时序

```
用户选择文件/文件夹 path
        │
        ▼
  CheckPathOpened(ctx, path)  ← 排除自身窗口
        │
        ├── 返回 0（未打开）──→ 正常加载 → RegisterOpenPath(ctx, path)
        │
        └── 返回 windowID（已打开）──→ FocusWindow(windowID)
                                        → Window.Close() 关闭当前窗口
```

**注意**：`CheckPathOpened` 排除当前窗口自身，避免刚注册就误判。

#### 决策 4：拖放事件隔离

将 `main.go` 中的全局广播改为按窗口发送：

```go
// 修改前：application.Get().Event.Emit("files-dropped", items)
// 修改后：win.EmitEvent("files-dropped", items)
```

同时抽取窗口创建+拖放事件注册逻辑为可复用函数，供 `NewWindow()` 调用。

#### 决策 5：路径规范化

去重比较不能只依赖 `filepath.Abs`。所有进入 `pathOwners` 的路径都应先经过统一规范化：

1. `filepath.Abs`
2. `filepath.Clean`
3. 尽量执行 `filepath.EvalSymlinks`，用于处理符号链接、junction 等指向同一位置的路径
4. Windows 下对路径做大小写归一化，避免 `C:\Docs\a.txt` 和 `c:\docs\A.txt` 被当成不同文件

路径校验与去重应复用同一个 canonical path helper，避免“校验认为同一路径，去重认为不同路径”的不一致。

#### 决策 6：macOS 行为调整

```go
// 修改前
Mac: application.MacOptions{
    ApplicationShouldTerminateAfterLastWindowClosed: true,
}

// 修改后：多窗口时不应在关闭一个窗口后退出
Mac: application.MacOptions{
    ApplicationShouldTerminateAfterLastWindowClosed: false,
}
```

---

## 4. 详细设计

### 4.1 后端改动

#### 4.1.1 `app.go` — 数据结构重构

```go
type windowState struct {
    currentRoot  string
    allowedFiles map[string]struct{}
    openPaths    map[string]struct{}
    mu           sync.Mutex
}

type App struct {
    states     sync.Map // windowID(uint) → *windowState
    pathOwners sync.Map // canonicalPath(string) → windowID(uint)
}

func NewApp() *App {
    return &App{}
}

// 从 context 提取窗口 ID
func windowIDFromCtx(ctx context.Context) uint {
    if win, ok := ctx.Value(application.WindowKey).(application.Window); ok {
        return win.ID()
    }
    return 0
}

// 获取或创建窗口状态
func (a *App) getOrCreateState(ctx context.Context) *windowState {
    id := windowIDFromCtx(ctx)
    if id == 0 {
        // 降级：无窗口上下文时返回临时状态（不应出现在正常流程中）
        return &windowState{allowedFiles: make(map[string]struct{})}
    }
    actual, _ := a.states.LoadOrStore(id, &windowState{
        allowedFiles: make(map[string]struct{}),
        openPaths:    make(map[string]struct{}),
    })
    return actual.(*windowState)
}
```

#### 4.1.2 `app.go` — 文件操作方法签名变更

所有涉及 `currentRoot` / `allowedFiles` 的方法增加 `ctx context.Context` 首参：

| 方法 | 变更前签名 | 变更后签名 |
| --- | --- | --- |
| `SelectFile` | `() (string, error)` | `(ctx context.Context) (string, error)` |
| `SelectFolder` | `() (string, error)` | `(ctx context.Context) (string, error)` |
| `LoadFolderTree` | `(root string) ([]FileTreeNode, error)` | `(ctx context.Context, root string) ([]FileTreeNode, error)` |
| `LoadFolderChildren` | `(path string) ([]FileTreeNode, error)` | `(ctx context.Context, path string) ([]FileTreeNode, error)` |
| `ReadFile` | `(path string) (*FileContent, error)` | `(ctx context.Context, path string) (*FileContent, error)` |
| `ReadFileWithEncoding` | `(path, encoding string) (*FileContent, error)` | `(ctx context.Context, path, encoding string) (*FileContent, error)` |
| `ReadFileChunk` | `(path string, offset int64, size int) (*FileChunk, error)` | `(ctx context.Context, path string, offset int64, size int) (*FileChunk, error)` |
| `SaveFile` | `(path, content, encoding string) error` | `(ctx context.Context, path, content, encoding string) error` |
| `ShowInFileManager` | `(path string) error` | `(ctx context.Context, path string) error` |

内部方法 `readFile`、`validateFilePath`、`validateFileManagerPath` 同步增加 `ctx` 参数，内部使用 `a.getOrCreateState(ctx)` 替代直接访问 `a.currentRoot` / `a.allowedFiles`。

`SelectFile` / `SelectFolder` 增加 `ctx` 后，文件选择对话框应通过 `ctx.Value(application.WindowKey)` 获取调用窗口，并调用 `AttachToWindow(win)`，确保多窗口下弹窗归属正确。

#### 4.1.3 `app.go` — 方法内部改造示例

以 `LoadFolderTree` 为例：

```go
// 修改前
func (a *App) LoadFolderTree(root string) ([]FileTreeNode, error) {
    // ...
    a.currentRoot = cleanRoot
    a.resetAllowedFiles()
    return nodes, nil
}

// 修改后
func (a *App) LoadFolderTree(ctx context.Context, root string) ([]FileTreeNode, error) {
    // ...
    state := a.getOrCreateState(ctx)
    state.mu.Lock()
    state.currentRoot = cleanRoot
    state.allowedFiles = make(map[string]struct{})
    state.mu.Unlock()
    return nodes, nil
}
```

以 `validateFilePath` 为例：

```go
// 修改前
func (a *App) validateFilePath(path string) (string, os.FileInfo, error) {
    // ...
    if strings.TrimSpace(a.currentRoot) == "" {
        a.currentRoot = filepath.Dir(cleanPath)
    }
    if !isPathWithinRoot(a.currentRoot, cleanPath) && !a.isAllowedFile(cleanPath) {
        return "", nil, errors.New("当前文件不在已打开文件夹范围内")
    }
    // ...
}

// 修改后
func (a *App) validateFilePath(ctx context.Context, path string) (string, os.FileInfo, error) {
    // ...
    state := a.getOrCreateState(ctx)
    state.mu.Lock()
    defer state.mu.Unlock()

    if strings.TrimSpace(state.currentRoot) == "" {
        state.currentRoot = filepath.Dir(cleanPath)
    }
    if !isPathWithinRoot(state.currentRoot, cleanPath) && !isAllowedFile(state, cleanPath) {
        return "", nil, errors.New("当前文件不在已打开文件夹范围内")
    }
    // ...
}
```

`allowFile` / `resetAllowedFiles` / `isAllowedFile` 改为操作 `windowState`：

```go
func allowFile(state *windowState, path string) {
    if state.allowedFiles == nil {
        state.allowedFiles = make(map[string]struct{})
    }
    state.allowedFiles[path] = struct{}{}
}

func isAllowedFile(state *windowState, path string) bool {
    _, ok := state.allowedFiles[path]
    return ok
}
```

#### 4.1.4 新增方法 — 窗口管理

```go
// NewWindow 创建一个新窗口，展示首页
func (a *App) NewWindow() {
    app := application.Get()
    win := createMainWindow(app)
    registerFileDropHandler(win)
    win.Show()
}

// RegisterOpenPath 注册当前窗口打开的文件/文件夹路径
func (a *App) RegisterOpenPath(ctx context.Context, path string) error {
    canonicalPath, err := canonicalizePath(path)
    if err != nil {
        return err
    }
    id := windowIDFromCtx(ctx)
    if id == 0 {
        return errors.New("无法获取当前窗口")
    }

    state := a.getOrCreateState(ctx)
    state.mu.Lock()
    if state.openPaths == nil {
        state.openPaths = make(map[string]struct{})
    }
    state.openPaths[canonicalPath] = struct{}{}
    state.mu.Unlock()

    a.pathOwners.Store(canonicalPath, id)
    return nil
}

// UnregisterOpenPath 注销当前窗口不再打开的路径（如关闭 tab 或回到首页）
func (a *App) UnregisterOpenPath(ctx context.Context, path string) error {
    canonicalPath, err := canonicalizePath(path)
    if err != nil {
        return err
    }
    id := windowIDFromCtx(ctx)
    if id == 0 {
        return errors.New("无法获取当前窗口")
    }

    state := a.getOrCreateState(ctx)
    state.mu.Lock()
    delete(state.openPaths, canonicalPath)
    state.mu.Unlock()

    if owner, ok := a.pathOwners.Load(canonicalPath); ok && owner.(uint) == id {
        a.pathOwners.Delete(canonicalPath)
    }
    return nil
}

// CheckPathOpened 检查路径是否已在其他窗口打开
// 返回已打开该路径的窗口 ID，若未打开则返回 0
func (a *App) CheckPathOpened(ctx context.Context, path string) (uint, error) {
    canonicalPath, err := canonicalizePath(path)
    if err != nil {
        return 0, fmt.Errorf("解析路径失败: %w", err)
    }

    currentID := windowIDFromCtx(ctx)
    owner, ok := a.pathOwners.Load(canonicalPath)
    if !ok {
        return 0, nil
    }
    ownerID := owner.(uint)
    if ownerID == currentID {
        return 0, nil
    }
    return ownerID, nil
}

// FocusWindow 聚焦指定窗口
func (a *App) FocusWindow(windowID uint) error {
    app := application.Get()
    win, exists := app.Window.GetByID(windowID)
    if !exists {
        return errors.New("窗口不存在")
    }
    win.Show()
    win.Focus()
    return nil
}

// cleanupWindowState 清理窗口关闭后的状态
func (a *App) cleanupWindowState(windowID uint) {
    if value, ok := a.states.Load(windowID); ok {
        state := value.(*windowState)
        state.mu.Lock()
        for path := range state.openPaths {
            if owner, ok := a.pathOwners.Load(path); ok && owner.(uint) == windowID {
                a.pathOwners.Delete(path)
            }
        }
        state.mu.Unlock()
    }
    a.states.Delete(windowID)
}
```

**注意**：`app.Window.NewWithOptions` 在当前 Wails v3 alpha.74 中已经会走内部 `runOrDeferToAppRun` 逻辑，业务代码不应调用未导出的 `app.runOrDeferToAppRun`。新窗口创建后如需立即显示，调用 `win.Show()` 即可。

#### 4.1.5 `main.go` — 抽取窗口创建逻辑

```go
func main() {
    app := application.New(application.Options{
        Name:        "MostFileViewer",
        Description: "文件夹预览器 - 浏览目录并预览 Word、Excel 文件",
        Services: []application.Service{
            application.NewService(NewApp()),
        },
        Assets: application.AssetOptions{
            Handler: application.AssetFileServerFS(assets),
        },
        Mac: application.MacOptions{
            ApplicationShouldTerminateAfterLastWindowClosed: false, // 修改：支持多窗口
        },
    })

    win := createMainWindow(app)
    registerFileDropHandler(win)

    err := app.Run()
    if err != nil {
        log.Fatal(err)
    }
}

// createMainWindow 创建主窗口（复用逻辑）
func createMainWindow(app *application.App) *application.WebviewWindow {
    return app.Window.NewWithOptions(application.WebviewWindowOptions{
        Title:            "MostFileViewer",
        Width:            1024,
        Height:           768,
        Frameless:        true,
        BackgroundColour: application.NewRGB(243, 246, 251),
        URL:              "/",
        EnableFileDrop:   true,
    })
}

// registerFileDropHandler 注册文件拖放事件处理器（按窗口发送事件）
func registerFileDropHandler(win *application.WebviewWindow) {
    win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
        files := event.Context().DroppedFiles()
        if len(files) == 0 {
            return
        }

        items := make([]dropItem, 0, len(files))
        for _, file := range files {
            info, err := os.Stat(file)
            if err != nil {
                continue
            }
            items = append(items, dropItem{
                Path:  file,
                IsDir: info.IsDir(),
            })
        }

        if len(items) > 0 {
            win.EmitEvent("files-dropped", items) // 修改：按窗口发送
        }
    })
}
```

#### 4.1.6 窗口关闭事件 — 清理状态

在 `registerFileDropHandler` 中追加窗口关闭事件监听：

```go
func registerWindowHandlers(app *application.App, win *application.WebviewWindow, appInstance *App) {
    // 文件拖放
    win.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
        // ... 同上
    })

    // 窗口关闭时清理状态
    win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
        appInstance.cleanupWindowState(win.ID())
    })
}
```

### 4.2 前端改动

#### 4.2.1 `TitleBar.vue` — 新增菜单项

在「选择文件夹」下方新增「新建窗口」选项：

```html
<button
    class="dropdown-menu__item"
    @click.stop="handleSelectFolder"
>
    <!-- 选择文件夹图标 -->
    选择文件夹
</button>

<!-- 新增：新建窗口 -->
<div class="dropdown-menu__divider"></div>
<button
    class="dropdown-menu__item"
    @click.stop="handleNewWindow"
>
    <svg
        xmlns="http://www.w3.org/2000/svg"
        fill="none"
        viewBox="0 0 24 24"
        stroke-width="1.5"
        stroke="currentColor"
        class="dropdown-menu__icon"
    >
        <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M3.75 3.75v4.5m0-4.5h4.5m-4.5 0L9 8.25M3.75 20.25v-4.5m0 4.5h4.5m-4.5 0L9 15.75M20.25 3.75h-4.5m4.5 0v4.5m0-4.5L15 8.25M20.25 20.25h-4.5m4.5 0v-4.5m0 4.5L15 15.75"
        />
    </svg>
    新建窗口
</button>
```

脚本部分新增：

```javascript
const emit = defineEmits([
    "select-file",
    "select-folder",
    "toggle-sidebar",
    "new-window", // 新增
]);

function handleNewWindow() {
    closeMenu();
    emit("new-window");
}
```

样式部分新增分隔线：

```css
.dropdown-menu__divider {
    height: 1px;
    background: var(--border-subtle);
    margin: 4px 8px;
}
```

#### 4.2.2 `App.vue` — 处理新建窗口事件

模板中 `TitleBar` 增加 `@new-window` 监听：

```html
<TitleBar
    :show-sidebar-toggle="isActualFolderPreview"
    :sidebar-open="sidebarOpen"
    @select-folder="handleSelectFolder"
    @select-file="handleSelectFile"
    @toggle-sidebar="toggleSidebar"
    @new-window="handleNewWindow"
/>
```

脚本中新增导入和处理函数：

```javascript
import { Window } from "@wailsio/runtime";

async function handleNewWindow() {
    try {
        await App.NewWindow();
    } catch (error) {
        // silently ignore
    }
}
```

#### 4.2.3 `App.vue` — 去重检查逻辑

抽取通用的去重检查函数：

```javascript
/**
 * 检查路径是否已在其他窗口打开
 * @param {string} path - 文件/文件夹路径
 * @returns {Promise<boolean>} true 表示已处理（已聚焦原窗口并关闭当前窗口），false 表示未重复
 */
async function checkAndRedirectIfOpened(path) {
    try {
        const existingWindowID = await App.CheckPathOpened(path);
        if (existingWindowID && existingWindowID > 0) {
            // 已在其他窗口打开，聚焦原窗口并关闭当前窗口
            await App.FocusWindow(existingWindowID);
            await Window.Close();
            return true;
        }
    } catch (error) {
        // 检查失败时不阻断正常流程
    }
    return false;
}

/**
 * 注册当前窗口打开的路径
 */
async function registerOpenPath(path) {
    try {
        await App.RegisterOpenPath(path);
    } catch (error) {
        // silently ignore
    }
}
```

#### 4.2.4 `App.vue` — 改造 `handleSelectFile`

```javascript
async function handleSelectFile() {
    globalError.value = "";

    try {
        const filePath = await App.SelectFile();
        if (!filePath) {
            return;
        }

        // 去重检查：如果已在其他窗口打开，直接关闭当前窗口
        if (await checkAndRedirectIfOpened(filePath)) {
            return;
        }

        if (selectedFolder.value) {
            await addFileToWorkspace(filePath);
        } else {
            await openFileInWorkspace(filePath);
            // 首次打开文件时注册路径
            await registerOpenPath(filePath);
        }
    } catch (error) {
        // silently ignore
    }
}
```

#### 4.2.5 `App.vue` — 改造 `handleSelectFolder`

```javascript
async function handleSelectFolder() {
    globalError.value = "";

    try {
        const folder = await App.SelectFolder();
        if (!folder) {
            return;
        }

        // 去重检查
        if (await checkAndRedirectIfOpened(folder)) {
            return;
        }

        if (!(await saveDirtyTabs())) {
            return;
        }

        const tree = await App.LoadFolderTree(folder);
        selectedFolder.value = folder;
        treeData.value = tree;
        sidebarOpen.value = true;
        clearAllAutoSaveDebounceTimers();
        releaseAllTabPayloads();
        await nextTick();
        openTabs.value = [];
        activeTabPath.value = "";

        // 注册路径
        await registerOpenPath(folder);
    } catch (error) {
        // silently ignore
    }
}
```

#### 4.2.6 `App.vue` — 改造 `handleFilesDropped`

```javascript
async function handleFilesDropped(event) {
    globalError.value = "";

    const items = event.data;
    if (!Array.isArray(items) || items.length === 0) {
        return;
    }

    const item = items[0];
    if (!item?.path) {
        return;
    }

    // 去重检查
    if (await checkAndRedirectIfOpened(item.path)) {
        return;
    }

    if (item.isDir) {
        await handleOpenFolder(item.path);
        // 注册路径
        await registerOpenPath(item.path);
    } else {
        if (selectedFolder.value) {
            await addFileToWorkspace(item.path);
        } else {
            await openFileInWorkspace(item.path);
            await registerOpenPath(item.path);
        }
    }
}
```

#### 4.2.7 `App.vue` — 路径登记与注销

除 `registerOpenPath` 外，前端需要在 tab 关闭、工作区切换、返回首页时调用注销接口，确保后端去重索引与真实 UI 状态一致。

```javascript
async function unregisterOpenPath(path) {
    try {
        await App.UnregisterOpenPath(path);
    } catch (error) {
        // silently ignore
    }
}
```

- `openFileNode` 成功打开文件后登记该 tab 路径
- `handleCloseTab` 关闭 tab 后注销该文件路径
- `handleOpenFolder` / `handleSelectFolder` 切换工作区前注销旧工作区的已打开路径
- 单文件模式关闭最后一个 tab 回到首页时，必须注销该单文件路径

#### 4.2.8 `App.vue` — 改造 `handleOpenFolder`

```javascript
async function handleOpenFolder(folderPath) {
    try {
        if (!(await saveDirtyTabs())) {
            return;
        }

        const tree = await App.LoadFolderTree(folderPath);
        selectedFolder.value = folderPath;
        treeData.value = tree;
        sidebarOpen.value = true;
        clearAllAutoSaveDebounceTimers();
        releaseAllTabPayloads();
        await nextTick();
        openTabs.value = [];
        activeTabPath.value = "";
    } catch (error) {
        // silently ignore
    }
}
```

#### 4.2.9 前端导入变更

```javascript
// 修改前
import { Events } from "@wailsio/runtime";

// 修改后
import { Events, Window } from "@wailsio/runtime";
```

---

## 5. 文件变更清单

| 文件 | 变更类型 | 改动内容 |
| --- | --- | --- |
| `app.go` | 重构 + 新增 | 状态隔离重构；9 个方法签名变更；新增 6 个窗口/路径管理方法；新增路径 canonical helper |
| `main.go` | 重构 | 抽取 `createMainWindow` / `registerFileDropHandler`；拖放事件改按窗口发送；macOS 配置调整；窗口关闭清理 |
| `TitleBar.vue` | 新增 | 新增「新建窗口」菜单项 + 分隔线 + emit 事件 |
| `App.vue` | 改造 | 新增 `handleNewWindow`；抽取去重检查逻辑；改造 3 个文件打开入口；补充 tab/工作区路径注销 |
| `bindings/` | 自动生成 | 后端方法签名变更后重新生成前端绑定 |

---

## 6. 边界情况与注意事项

### 6.1 窗口关闭时状态清理

窗口关闭时必须清理 `states` 和 `pathOwners` 中对应窗口的数据，否则：

- 内存泄漏：关闭的窗口状态永远留在 map 中
- 去重误判：已关闭窗口的路径仍被认为「已打开」，导致新窗口无法打开同路径

如果后续增加“关闭窗口前确认保存”的拦截逻辑，清理动作不能发生在可取消的关闭事件之前；应确保窗口真正关闭后再清理，或在关闭被取消时恢复路径登记。

### 6.2 单文件 vs 文件夹的去重粒度

| 模式 | 登记值 | 示例 |
| --- | --- | --- |
| 文件夹模式 | 文件夹绝对路径 | `C:\docs\project` |
| 单文件模式 | 文件绝对路径 | `C:\docs\readme.md` |
| 多 tab 模式 | 每个 tab 的文件绝对路径 | `C:\docs\a.md`、`C:\docs\b.md` |

**注意**：当前架构中单文件模式 `selectedFolder` 设为父目录，但去重应使用文件路径本身。`RegisterOpenPath` 调用时传入的是文件路径而非 `selectedFolder`。

### 6.3 多 tab 去重与注销

用户在已有工作区中通过「选择文件」添加新 tab 时，需要先调用 `CheckPathOpened` 检查该文件是否已在其他窗口打开；如果返回的是当前窗口自身，则只切换到已有 tab，不触发关闭。

关闭 tab、切换文件夹、单文件模式关闭最后一个 tab 返回首页时，都必须调用 `UnregisterOpenPath`。否则后端仍会认为该路径已打开，导致后续新窗口无法打开同一路径。

### 6.4 竞态条件

两个窗口同时打开同一文件/文件夹的极端情况：

- `pathOwners` 的检查与登记不是事务操作
- 最坏情况：两个窗口都通过了 `CheckPathOpened` 检查，都注册了路径
- 影响：两个窗口打开了同一文件，用户可手动关闭其中一个
- **不做强一致锁**，避免引入复杂度，接受极低概率的竞态

如果后续希望彻底避免该竞态，可将 `CheckPathOpened` 与 `RegisterOpenPath` 合并为后端原子方法，例如 `TryRegisterOpenPath(ctx, path) (existingWindowID uint, registered bool, err error)`。

### 6.5 macOS 平台行为

`ApplicationShouldTerminateAfterLastWindowClosed` 改为 `false` 后：

- 关闭最后一个窗口不会退出应用
- 用户需通过系统菜单栏或 `Cmd+Q` 退出
- 这是多窗口应用的标准行为

### 6.6 重新生成绑定

后端方法签名变更后（加 `context.Context`），Wails 绑定生成器会自动处理：

- `context.Context` 参数不会暴露给前端
- 前端调用方式不变（参数列表不包含 ctx）
- 执行 `wails generate` 或对应的 task 命令重新生成

### 6.7 Windows 任务栏

多个窗口会在 Windows 任务栏显示多个图标，这是预期行为。每个窗口可独立最小化/恢复。

### 6.8 窗口标题

当前所有窗口标题相同（"MostFileViewer"）。可后续优化为显示当前打开的文件夹/文件名，但不在本次需求范围内。

### 6.9 Wails API 注意事项

- `app.Window.NewWithOptions` 已经负责将窗口交给 Wails 内部生命周期管理，不应调用未导出的 `app.runOrDeferToAppRun`
- 窗口关闭拦截使用 `RegisterHook(events.Common.WindowClosing, ...)`；普通通知使用 `OnWindowEvent`
- 多窗口下文件选择框应 `AttachToWindow` 到调用窗口，避免弹窗出现在错误窗口前或焦点归属异常

### 6.10 路径 canonical 化

所有路径进入 `pathOwners` 前必须使用统一 helper 规范化。特别需要覆盖：

- Windows 大小写不敏感路径
- 尾部分隔符差异
- 符号链接、junction、快捷方式/别名解析能力范围
- 相对路径与绝对路径混用

---

## 7. 测试计划

### 7.1 功能测试

| 测试项 | 步骤 | 预期结果 |
| --- | --- | --- |
| 新建窗口-基本 | 点击「文件」→「新建窗口」 | 新窗口打开，展示首页 hero |
| 新建窗口-多窗口 | 连续新建多个窗口 | 每个窗口独立展示首页 |
| 多窗口-文件预览隔离 | 窗口 A 打开文件夹 X，窗口 B 打开文件夹 Y | 两窗口各自正常预览，互不影响 |
| 多窗口-文件保存隔离 | 窗口 A 编辑文件，窗口 B 编辑另一文件 | 各自独立保存，不串扰 |
| 去重-文件夹 | 窗口 A 打开文件夹 X，窗口 B 新建窗口后打开文件夹 X | 窗口 B 关闭，窗口 A 聚焦 |
| 去重-单文件 | 窗口 A 打开文件 F，窗口 B 新建窗口后打开文件 F | 窗口 B 关闭，窗口 A 聚焦 |
| 去重-排除自身 | 窗口 A 打开文件夹 X 后选择文件 | 正常添加 tab，不误判为重复 |
| 去重-tab 跨窗口 | 窗口 A 以 tab 打开文件 F，窗口 B 打开文件 F | 窗口 B 关闭，窗口 A 聚焦 |
| 去重-添加 tab 命中其他窗口 | 窗口 A 打开文件 F，窗口 B 在已有工作区中选择文件 F | 窗口 A 聚焦，窗口 B 不新增重复 tab |
| 去重-关闭 tab 注销 | 窗口 A 打开文件 F 后关闭该 tab，窗口 B 打开文件 F | 窗口 B 正常打开，不误判为已存在 |
| 去重-返回首页注销 | 单文件模式打开文件 F，关闭最后一个 tab 回到首页，再新窗口打开 F | 新窗口正常打开 |
| 去重-路径 canonical | Windows 下用大小写不同路径或 symlink/junction 指向同一路径打开 | 能识别为同一路径并聚焦原窗口 |
| 拖放-隔离 | 窗口 A 拖入文件 | 只有窗口 A 响应，窗口 B 无反应 |
| 拖放-去重 | 窗口 A 已打开文件夹 X，窗口 B 拖入文件夹 X | 窗口 B 关闭，窗口 A 聚焦 |
| 窗口关闭-状态清理 | 打开窗口 B 后关闭，再新窗口打开同路径 | 正常打开，不误判为已存在 |
| 窗口关闭-多窗口不退出 | 打开两个窗口，关闭其中一个 | 另一窗口正常使用，应用不退出 |
| 对话框归属 | 从窗口 B 点击选择文件/文件夹 | 文件选择框显示并聚焦在窗口 B 前方 |

### 7.2 回归测试

| 测试项 | 说明 |
| --- | --- |
| 单窗口文件预览 | 确认重构后单窗口场景所有功能正常 |
| 文件树加载与展开 | 确认 `LoadFolderChildren` 正常 |
| 文件读取与编码切换 | 确认 `ReadFile` / `ReadFileWithEncoding` 正常 |
| 文件分块读取 | 确认 `ReadFileChunk` 正常 |
| 文件保存 | 确认 `SaveFile` 正常 |
| 在文件管理器中显示 | 确认 `ShowInFileManager` 正常 |
| 自动保存 | 确认防抖/定时保存不受影响 |
| 文件拖放 | 确认单窗口下拖放功能正常 |
| Wails API 编译检查 | 确认 `NewWindow` 不调用未导出的 `runOrDeferToAppRun`，窗口创建与关闭 hook 可编译 |

---

## 8. 实施计划

### 第一期：后端状态隔离重构

> 目标：将 `App` 的全局状态改为 per-window 隔离，为多窗口奠定基础。

1. 重构 `app.go` 数据结构（`windowState.openPaths` + `pathOwners`）
2. 所有文件操作方法增加 `context.Context` 参数
3. 内部校验方法改用 `getOrCreateState(ctx)`
4. `main.go` 抽取窗口创建逻辑
5. 重新生成前端绑定
6. 单窗口回归测试

### 第二期：新建窗口 + 路径去重

> 目标：实现完整的「新建窗口」功能和路径去重。

1. 后端新增 `NewWindow` / `RegisterOpenPath` / `UnregisterOpenPath` / `CheckPathOpened` / `FocusWindow` / `cleanupWindowState`
2. `main.go` 拖放事件改按窗口发送 + 窗口关闭清理
3. 前端 `TitleBar.vue` 新增菜单项
4. 前端 `App.vue` 实现去重检查逻辑，并在 tab/工作区生命周期中同步登记和注销路径
5. 多窗口功能测试

---

## 9. 工作量估算

| 模块 | 工作量 | 风险 |
| --- | --- | --- |
| `app.go` 状态隔离重构 | 1 天 | 高（触及所有文件操作） |
| `main.go` 重构 | 0.5 天 | 中 |
| 新增窗口管理方法 | 0.5 天 | 中 |
| 前端 `TitleBar.vue` | 0.5 天 | 低 |
| 前端 `App.vue` 去重逻辑 | 0.5 天 | 中 |
| 绑定重新生成 | 0.5 天 | 低 |
| 测试 | 1 天 | - |
| **合计** | **约 4.5 天** | |

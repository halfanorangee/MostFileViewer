# MostFileViewer 明暗主题切换功能 — 详细设计文档

---

## 1. 需求描述

### 1.1 功能目标

在标题栏右侧（窗口控制按钮组左侧）新增一个主题切换按钮，点击后在**明色主题**与**暗色主题**之间切换，并满足：

1. 切换即时生效，全应用所有界面同步更新
2. 用户选择持久化，下次启动自动恢复
3. 首次启动时跟随系统偏好（`prefers-color-scheme`）
4. 多窗口之间主题同步（在窗口 A 切换，窗口 B 同步更新）
5. 切换无白屏闪烁（FOUC）

### 1.2 交互流程

```
应用启动
    │
    ▼
index.html 内联脚本读取 localStorage['mfv-theme']
    │
    ├── 无存储值 → 读取系统 prefers-color-scheme → 设置 data-theme
    │
    └── 有存储值（light / dark）→ 直接设置 data-theme
    │
    ▼
Vue 应用挂载，useTheme 组合式函数接管状态
    │
    ▼
用户点击标题栏主题按钮
    │
    ▼
切换 data-theme 属性 → CSS 变量级联更新 → 全局重绘
    │
    ▼
写入 localStorage → storage 事件广播至其他窗口 → 多窗口同步
```

### 1.3 约束条件

- **零硬编码颜色**：经全局扫描，所有 `.vue` 组件均通过 CSS 变量引用颜色，暗色主题只需覆写 `theme.css` 中的变量，无需改动组件样式
- **CodeMirror 主题**：`CodePreview.vue` 的 `EditorView.theme()` 已全部引用 `--cm-*` 变量，覆写变量即可
- **Excel 文档色不变**：`composables/excel/constants.js` 中的 `THEME_COLORS` 是 OOXML 文档内置调色板，代表电子表格文档自身的颜色，**不随应用主题变化**
- **Wails 窗口背景色**：`main.go` 中 `BackgroundColour` 在窗口创建时固定，切换主题时需后端配合更新

---

## 2. 现有架构分析

### 2.1 主题系统现状

**`theme.css`**：

- 所有 UI 颜色集中在 `:root` 下定义，约 90 个 CSS 自定义属性
- 文件头注释明确预留了暗色主题入口：

```css
/**
 * 当前阶段仅定义浅色主题（:root），为未来暗色主题预留
 * [data-theme="dark"] 覆写入口。
 */
```

- 变量组织清晰，按语义分类：文字、背景、边框、主题色、状态色、交互态透明色、阴影、工具栏、渐变、CodeMirror、Excel、布局尺寸、layui 对齐

**组件层**：

- 全局扫描确认：所有 `.vue` 文件的 `<style>` 段落中**无任何硬编码十六进制颜色或 rgb/rgba 值**，全部通过 `var(--token)` 引用
- `CodePreview.vue` 的 CodeMirror 主题配置中所有颜色均引用 `--cm-*` 变量
- `composables/excel/styles.js` 的 `getDefaultBorderColor()` 优先读取 `--excel-border-default` 变量，仅在其不可用时回退到硬编码 `#1f2937`

### 2.2 标题栏结构

**`TitleBar.vue`**：

```
┌─────────────────────────────────────────────────────────┐
│  title-bar                                              │
│  ┌─────────────────────────┐  ┌──────────────────────┐  │
│  │  title-bar__drag        │  │  title-bar__controls │  │
│  │  (标题 + 文件菜单 +     │  │  (最小化 最大化 关闭)│  │
│  │   侧边栏开关)           │  │                      │  │
│  └─────────────────────────┘  └──────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

右侧 `.title-bar__controls` 区域当前包含三个窗口控制按钮，主题切换按钮将插入到最小化按钮**之前**。

### 2.3 应用初始化链路

```
index.html
  └── <script src="./src/main.js" type="module">
        ├── import "./theme.css"     ← CSS 变量定义
        ├── import "./style.css"     ← 全局样式（引用 CSS 变量）
        └── createApp(App).mount("#app")
              └── App.vue
                    ├── TitleBar.vue
                    ├── FileTree.vue
                    └── PreviewTabs.vue → Code/Excel/Word/PPT/PDF/Image
```

当前 `index.html` 无任何主题初始化逻辑，`<html>` 标签无 `data-theme` 属性。

### 2.4 后端现状

- `app.go` 和 `main.go` 中**无任何主题相关代码**
- `createMainWindow()` 中 `BackgroundColour: application.NewRGB(243, 246, 251)` 为明色背景，在暗色主题下需更新
- 多窗口架构已就绪（`NewWindow()` → `createMainWindow()`），新窗口会复用同一背景色

---

## 3. 架构设计

### 3.1 总体架构

```
┌──────────────────────────────────────────────────────────┐
│                      主题系统分层                          │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │  第一层：CSS 变量定义 (theme.css)                    │ │
│  │  ├── :root                    → 明色主题（现有）     │ │
│  │  └── [data-theme="dark"]      → 暗色主题（新增）     │ │
│  └─────────────────────────────────────────────────────┘ │
│                          ↑ var() 引用                     │
│  ┌─────────────────────────────────────────────────────┐ │
│  │  第二层：DOM 属性驱动 (data-theme on <html>)         │ │
│  │  切换 data-theme → CSS 变量级联 → 全局重绘           │ │
│  └─────────────────────────────────────────────────────┘ │
│                          ↑ setAttribute                   │
│  ┌─────────────────────────────────────────────────────┐ │
│  │  第三层：状态管理 (composables/useTheme.js)          │ │
│  │  ├── currentTheme ref ('light' | 'dark')            │ │
│  │  ├── toggle() / setTheme()                          │ │
│  │  ├── 系统偏好监听 (matchMedia)                      │ │
│  │  ├── localStorage 持久化                            │ │
│  │  └── storage 事件 → 多窗口同步                       │ │
│  └─────────────────────────────────────────────────────┘ │
│                          ↑ 调用                            │
│  ┌─────────────────────────────────────────────────────┐ │
│  │  第四层：UI 触发 (TitleBar.vue)                      │ │
│  │  └── 主题切换按钮 → toggle()                         │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │  FOUC 防护层 (index.html 内联脚本)                   │ │
│  │  Vue 挂载前同步设置 data-theme，避免闪烁             │ │
│  └─────────────────────────────────────────────────────┘ │
│                                                           │
│  ┌─────────────────────────────────────────────────────┐ │
│  │  后端配合 (app.go / main.go)                         │ │
│  │  ├── SetWindowBackground(theme) → 更新窗口背景色      │ │
│  │  └── BroadcastThemeChange(theme) → 多窗口广播        │ │
│  └─────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

### 3.2 核心设计决策

#### 决策 1：主题切换机制 — `data-theme` 属性 + CSS 变量覆写

**问题**：如何以最低成本实现全局主题切换？

**方案**：在 `<html>` 元素上设置 `data-theme` 属性，在 `theme.css` 中通过 `[data-theme="dark"]` 选择器覆写暗色变量值。

**理由**：

- 项目已建立完善的 CSS 变量体系，组件零硬编码颜色
- CSS 变量级联天然支持主题切换，浏览器自动重绘，性能最优
- `theme.css` 注释已预留此入口，符合既有设计意图
- 无需任何 JS 遍历 DOM 修改样式

#### 决策 2：主题模式 — 二态切换（light / dark）

**问题**：是否需要 `auto`（跟随系统）三态模式？

**方案**：内部支持 `auto` 作为默认初始值，但 UI 按钮仅在 `light` ↔ `dark` 之间显式切换。首次启动无存储偏好时使用 `auto`（跟随系统），用户点击按钮后变为显式选择。

**理由**：

- 二态切换按钮 UX 简单直觉（太阳/月亮图标互换）
- 首次跟随系统满足"开箱即用"预期
- 用户一旦手动选择，以用户选择为准（不再跟随系统）
- 避免 UI 上展示三态按钮的复杂度

#### 决策 3：FOUC 防护 — index.html 内联同步脚本

**问题**：Vue 应用通过 `type="module"` 异步加载，挂载前页面会以默认（明色）渲染，暗色用户会看到白屏闪烁。

**方案**：在 `index.html` 的 `<head>` 中插入一段**同步内联脚本**（非 module），在 CSS 和 Vue 加载前就读取 localStorage 并设置 `data-theme`。

```html
<head>
    <script>
        (function () {
            try {
                var stored = localStorage.getItem('mfv-theme');
                var theme = stored;
                if (!theme || theme === 'auto') {
                    theme = window.matchMedia('(prefers-color-scheme: dark)').matches
                        ? 'dark' : 'light';
                }
                document.documentElement.setAttribute('data-theme', theme);
            } catch (e) {}
        })();
    </script>
</head>
```

**理由**：

- 同步脚本在解析阶段立即执行，早于 CSS 应用和 Vue 挂载
- `data-theme` 属性设置后，后续加载的 `theme.css` 直接命中正确的变量集
- `try/catch` 防止 localStorage 不可用时阻断页面

#### 决策 4：多窗口同步 — localStorage storage 事件 + Wails 事件双保险

**问题**：在窗口 A 切换主题，窗口 B 如何同步？

**方案**：

1. **主通道**：`localStorage` 的 `storage` 事件。同源的多窗口共享 localStorage，当窗口 A 修改 `mfv-theme` 时，窗口 B 的 `window` 对象会触发 `storage` 事件
2. **备通道**：Wails `Events.Emit` 广播。若 webview 环境下 `storage` 事件不可靠，通过后端 `App.BroadcastThemeChange(theme)` 向所有窗口发送事件

**理由**：

- `storage` 事件是浏览器原生跨窗口通信机制，零后端改动
- Wails 事件作为备通道提升可靠性
- 两者可共存，通过去重逻辑避免重复处理

#### 决策 5：窗口背景色 — 后端方法动态更新

**问题**：`BackgroundColour` 在窗口创建时固定为明色 RGB，暗色主题下窗口创建/恢复时会短暂闪现白色。

**方案**：后端新增 `SetWindowBackground(ctx, theme)` 方法，前端在切换主题时调用，动态设置当前窗口背景色。`NewWindow()` 创建新窗口时也根据当前主题选择背景色。

**理由**：

- 窗口背景色在 webview 未渲染区域可见（如窗口恢复动画期间）
- 暗色主题下白色背景闪烁体验差
- 后端方法可通过 `ctx` 获取当前窗口，精确更新

---

## 4. 详细设计

### 4.1 前端改动

#### 4.1.1 新增 `composables/useTheme.js` — 主题状态管理

```javascript
import { ref, onMounted, onBeforeUnmount } from "vue";

const STORAGE_KEY = "mfv-theme";

// 模块级单例状态，所有组件共享同一实例
const currentTheme = ref("light");
let initialized = false;

function resolveSystemTheme() {
    return window.matchMedia("(prefers-color-scheme: dark)").matches
        ? "dark"
        : "light";
}

function applyTheme(theme) {
    currentTheme.value = theme;
    document.documentElement.setAttribute("data-theme", theme);
}

function persistTheme(theme) {
    try {
        localStorage.setItem(STORAGE_KEY, theme);
    } catch (e) {
        // localStorage 不可用时静默降级
    }
}

/**
 * 主题状态管理组合式函数（模块级单例）
 *
 * 调用 useTheme() 的多个组件共享同一份 currentTheme 状态。
 * 初始化逻辑只在首次调用时执行，读取 index.html 内联脚本已设置的 data-theme 属性。
 */
export function useTheme() {
    if (!initialized) {
        initialized = true;

        // 读取内联脚本已设置的 data-theme（避免与 FOUC 脚本冲突）
        const applied = document.documentElement.getAttribute("data-theme");
        if (applied === "light" || applied === "dark") {
            currentTheme.value = applied;
        }

        // 监听其他窗口的 storage 变化（多窗口同步主通道）
        window.addEventListener("storage", onStorageChange);
    }

    function onStorageChange(event) {
        if (event.key === STORAGE_KEY && event.newValue) {
            const theme = event.newValue;
            if (theme === "light" || theme === "dark") {
                applyTheme(theme);
            }
        }
    }

    function toggle() {
        const next = currentTheme.value === "dark" ? "light" : "dark";
        applyTheme(next);
        persistTheme(next);
        // 触发后端窗口背景色更新 + 多窗口广播（见 4.2.2）
        notifyBackendThemeChange(next);
    }

    function setTheme(theme) {
        if (theme !== "light" && theme !== "dark") return;
        applyTheme(theme);
        persistTheme(theme);
        notifyBackendThemeChange(theme);
    }

    const isDark = computed(() => currentTheme.value === "dark");

    return { currentTheme, isDark, toggle, setTheme };
}

// 后端通知（在 4.2 节实现后启用）
async function notifyBackendThemeChange(theme) {
    try {
        const { App } = await import("../bindings/MostFileViewer");
        await App.SetWindowBackground(theme);
        await App.BroadcastThemeChange(theme);
    } catch (e) {
        // 后端不可用时静默降级，前端主题切换仍正常
    }
}
```

> **注意**：`computed` 需从 `vue` 导入。上面代码省略了 `import { computed }`，实际实现时需补全。

#### 4.1.2 `TitleBar.vue` — 新增主题切换按钮

在 `.title-bar__controls` 区域，最小化按钮**之前**插入主题切换按钮：

```html
<div class="title-bar__controls">
    <!-- 新增：主题切换按钮 -->
    <button
        class="title-bar__btn title-bar__btn--icon"
        :title="isDark ? '切换到浅色主题' : '切换到深色主题'"
        :aria-label="isDark ? '切换到浅色主题' : '切换到深色主题'"
        @mousedown.stop
        @click.stop="handleToggleTheme"
    >
        <!-- 暗色模式下显示太阳图标（点击切换到浅色） -->
        <svg
            v-if="isDark"
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="1.5"
            stroke="currentColor"
            class="title-bar__icon"
        >
            <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z"
            />
        </svg>
        <!-- 浅色模式下显示月亮图标（点击切换到深色） -->
        <svg
            v-else
            xmlns="http://www.w3.org/2000/svg"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="1.5"
            stroke="currentColor"
            class="title-bar__icon"
        >
            <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M21.752 15.002A9.718 9.718 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z"
            />
        </svg>
    </button>

    <!-- 现有：最小化按钮 -->
    <button class="title-bar__btn" title="最小化" ...>
        ...
    </button>
    ...
</div>
```

脚本部分新增：

```javascript
import { useTheme } from "../composables/useTheme";

const { isDark, toggle: toggleTheme } = useTheme();

function handleToggleTheme() {
    toggleTheme();
}
```

> 按钮复用现有的 `.title-bar__btn` 和 `.title-bar__btn--icon` 样式类，hover 效果与其它标题栏按钮一致，无需额外 CSS。

#### 4.1.3 `index.html` — FOUC 防护内联脚本

```html
<!doctype html>
<html lang="en">
    <head>
        <meta charset="UTF-8" />
        <meta content="width=device-width, initial-scale=1.0" name="viewport" />
        <title>MostFileViewer</title>
        <!-- FOUC 防护：在 CSS 和 Vue 加载前同步设置主题 -->
        <script>
            (function () {
                try {
                    var stored = localStorage.getItem("mfv-theme");
                    var theme = stored;
                    if (!theme || theme === "auto") {
                        theme = window.matchMedia(
                            "(prefers-color-scheme: dark)",
                        ).matches
                            ? "dark"
                            : "light";
                    }
                    document.documentElement.setAttribute("data-theme", theme);
                } catch (e) {}
            })();
        </script>
    </head>
    <body>
        <div id="app"></div>
        <script src="./src/main.js" type="module"></script>
    </body>
</html>
```

#### 4.1.4 `theme.css` — 新增暗色主题变量覆写

在文件末尾 `:root { ... }` 块之后，新增 `[data-theme="dark"]` 覆写块。**仅覆写有变化的变量**，未列出的变量自动继承 `:root` 的值。

```css
/* ================================================================
 * 暗色主题
 * 通过 <html data-theme="dark"> 激活，覆写 :root 中的明色变量
 * ================================================================ */
[data-theme="dark"] {
    /* ---- 文字颜色 ---- */
    --text-primary: #e6edf3;
    --text-heading: #f0f6fc;
    --text-active: #ffffff;
    --text-secondary: #c9d1d9;
    --text-toolbar: #8b949e;
    --text-muted: #7d8590;
    --text-caption: #8b949e;
    --text-disabled: #484f58;
    --text-notice: #db6d28;

    /* ---- 背景颜色 ---- */
    --bg-app: #0d1117;
    --bg-surface: #161b22;
    --bg-titlebar: #161b22;
    --bg-preview: #21262d;
    --bg-image-viewer: #21262d;
    --bg-dropzone: #21262d;
    --bg-toolbar: #161b22;
    --bg-statusbar: #161b22;
    --bg-panel: #1c2128;
    --bg-hover: rgba(255, 255, 255, 0.06);
    --bg-active: rgba(56, 139, 253, 0.15);
    --bg-header: #21262d;
    --bg-active-line: rgba(255, 255, 255, 0.04);
    --bg-active-line-gutter: rgba(255, 255, 255, 0.04);

    /* ---- 边框颜色 ---- */
    --border-default: #30363d;
    --border-strong: #3d444d;
    --border-subtle: #21262d;
    --border-cell: #30363d;
    --border-gutter: #21262d;
    --border-input: #3d444d;
    --border-dashed: #3d444d;
    --border-toolbar: rgba(139, 148, 158, 0.28);
    --border-toolbar-pdf: rgba(139, 148, 158, 0.35);
    --border-toolbar-image: rgba(230, 237, 243, 0.1);

    /* ---- 主题色（蓝色系，暗色下提亮） ---- */
    --accent-primary-rgb: 56, 139, 253;
    --accent-hover: #58a6ff;
    --accent-active: #1f6feb;
    --accent-deep: #79c0ff;

    /* ---- 状态色 ---- */
    --status-error: #f85149;
    --status-error-text: #ff7b72;
    --color-danger: #f85149;
    --status-warning: #d29922;
    --status-info-rgb: 56, 139, 253;
    --status-search-match-bg: #4d3a1a;
    --status-search-match-border: #d29922;
    --status-search-selected-bg: rgba(56, 139, 253, 0.2);

    /* ---- 交互态透明色 ---- */
    --hover-overlay: rgba(255, 255, 255, 0.06);

    /* ---- 阴影（暗色下加深） ---- */
    --shadow-dropdown: 0 8px 24px rgba(0, 0, 0, 0.4);
    --shadow-page: 0 18px 40px rgba(0, 0, 0, 0.5);

    /* ---- 工具栏半透明背景 ---- */
    --bg-toolbar-ppt: rgba(22, 27, 34, 0.86);
    --bg-toolbar-pdf: rgba(22, 27, 34, 0.9);
    --bg-toolbar-image: rgba(22, 27, 34, 0.92);

    /* ---- 渐变背景 ---- */
    --bg-word-preview:
        radial-gradient(
            circle at top,
            rgba(255, 255, 255, 0.04),
            transparent 32%
        ),
        var(--bg-preview);

    /* ---- layui-vue 中性色对齐 ---- */
    --global-neutral-color-4: #30363d;
    --global-neutral-color-5: #3d444d;
    --global-neutral-color-7: #484f58;
}
```

> **说明**：
> - CodeMirror 专用变量（`--cm-*`）全部通过 `var()` 引用其他变量（如 `--cm-bg: var(--bg-surface)`），无需单独覆写，自动跟随
> - `--excel-border-default: var(--text-primary)` 自动跟随文字色变浅
> - `--cm-search-checkbox-check-color: #ffffff` 保持不变（复选框勾选标记始终白色）
> - `--global-primary-color` 等通过 `var()` 引用 `--accent-primary`，自动跟随

#### 4.1.5 `App.vue` — 监听后端主题广播（多窗口同步备通道）

```javascript
import { Events } from "@wailsio/runtime";
import { useTheme } from "./composables/useTheme";

const { setTheme } = useTheme();

onMounted(() => {
    // ... 现有代码 ...

    // 监听后端主题广播（多窗口同步备通道）
    removeThemeChangeListener = Events.On("theme-changed", (event) => {
        const theme = event.data;
        if (theme === "light" || theme === "dark") {
            // 直接应用，不回写 localStorage（避免循环触发 storage 事件）
            document.documentElement.setAttribute("data-theme", theme);
            // 同步更新 ref 状态
            // useTheme 内部已监听，此处仅更新 DOM 作为备通道
        }
    });
});

onBeforeUnmount(() => {
    // ... 现有代码 ...
    removeThemeChangeListener?.();
});
```

### 4.2 后端改动

#### 4.2.1 `app.go` — 新增主题相关方法

```go
// 主题对应的窗口背景色
var themeBackgrounds = map[string][3]uint8{
    "light": {243, 246, 251},
    "dark":  {13, 17, 23},
}

// SetWindowBackground 设置当前调用窗口的背景色
func (a *App) SetWindowBackground(ctx context.Context, theme string) error {
    win, ok := ctx.Value(application.WindowKey).(application.Window)
    if !ok {
        return errors.New("无法获取当前窗口")
    }

    rgb, exists := themeBackgrounds[strings.ToLower(theme)]
    if !exists {
        return fmt.Errorf("不支持的主题: %s", theme)
    }

    win.SetBackgroundColour(application.NewRGB(rgb[0], rgb[1], rgb[2]))
    return nil
}

// BroadcastThemeChange 向所有窗口广播主题变更
func (a *App) BroadcastThemeChange(theme string) error {
    app := application.Get()
    app.Event.Emit("theme-changed", theme)
    return nil
}

// GetPreferredTheme 返回当前应使用的主题（供新窗口创建时使用）
// 优先级：前端传入 > 默认 light
// 注：后端不读取 localStorage，由前端在 NewWindow 前通过 SetWindowBackground 设置
```

> **注意**：`win.SetBackgroundColour` 的具体 API 名称需根据 Wails v3 实际版本确认。若该 API 不存在，可通过 `win.SetProperty` 或重新创建窗口的替代方案处理。本设计假设 Wails v3 提供运行时修改窗口背景色的能力；如不可行，则降级为仅在窗口创建时设置背景色（接受切换瞬间的短暂闪烁）。

#### 4.2.2 `main.go` — 新窗口创建时使用正确背景色

```go
// createMainWindow 创建主窗口，backgroundTheme 指定初始背景色
func createMainWindow(app *application.App, backgroundTheme string) *application.WebviewWindow {
    rgb := themeBackgrounds[backgroundTheme]
    if rgb == [3]uint8{} {
        rgb = [3]uint8{243, 246, 251} // 默认明色
    }

    return app.Window.NewWithOptions(application.WebviewWindowOptions{
        Title:            "MostFileViewer",
        Width:            1024,
        Height:           768,
        Frameless:        true,
        BackgroundColour: application.NewRGB(rgb[0], rgb[1], rgb[2]),
        URL:              "/",
        EnableFileDrop:   true,
    })
}
```

`NewWindow()` 方法调整为接收当前主题：

```go
func (a *App) NewWindow() {
    app := application.Get()
    // 新窗口默认使用明色背景；前端挂载后会通过 FOUC 脚本设置正确主题
    // 并通过 SetWindowBackground 校正背景色
    win := createMainWindow(app, "light")
    registerWindowHandlers(win, a)
    win.Show()
}
```

> **说明**：由于后端无法读取前端 localStorage，新窗口初始以明色背景创建。前端 `index.html` 内联脚本会在 webview 渲染前设置正确主题，视觉上无感知。`SetWindowBackground` 在前端初始化后调用以校正窗口原生背景色。

---

## 5. 文件变更清单

| 文件 | 变更类型 | 改动内容 |
| --- | --- | --- |
| `frontend/src/composables/useTheme.js` | **新增** | 主题状态管理组合式函数（单例）：currentTheme、toggle、setTheme、系统偏好监听、localStorage 持久化、storage 事件多窗口同步 |
| `frontend/src/components/TitleBar.vue` | 修改 | `.title-bar__controls` 新增主题切换按钮（太阳/月亮图标）；导入 useTheme |
| `frontend/src/theme.css` | 修改 | 新增 `[data-theme="dark"]` 暗色变量覆写块 |
| `frontend/index.html` | 修改 | `<head>` 新增 FOUC 防护内联脚本 |
| `frontend/src/App.vue` | 修改 | onMounted 中监听 `theme-changed` 后端事件（多窗口同步备通道） |
| `app.go` | 修改 | 新增 `SetWindowBackground`、`BroadcastThemeChange` 方法；新增 `themeBackgrounds` 映射 |
| `main.go` | 修改 | `createMainWindow` 增加背景色参数；`NewWindow` 适配 |
| `bindings/` | 自动生成 | 后端新增方法后重新生成前端绑定 |

---

## 6. 边界情况与注意事项

### 6.1 FOUC（白屏闪烁）防护

暗色主题用户最敏感的体验问题是启动时的白屏闪烁。防护链路：

```
index.html 内联同步脚本（最早）
    → data-theme 属性设置
    → theme.css 加载时直接命中 [data-theme="dark"] 变量
    → Vue 挂载时 DOM 已是暗色
```

**关键点**：内联脚本必须是**同步执行**（非 `type="module"`），且放在 `<head>` 中 CSS 引用之前（本项目中 CSS 通过 `main.js` 的 import 引入，晚于内联脚本）。

### 6.2 多窗口主题同步

| 通道 | 机制 | 可靠性 | 延迟 |
| --- | --- | --- | --- |
| 主通道 | `localStorage` `storage` 事件 | 浏览器原生，同源窗口间可靠 | 即时 |
| 备通道 | Wails `Events.Emit` 广播 | Wails 框架保证 | 即时 |

**去重逻辑**：备通道收到事件时仅更新 DOM 和 ref，**不回写 localStorage**，避免触发 storage 事件形成循环。主通道（storage 事件）同理，仅更新 DOM 不回写。

### 6.3 系统主题偏好变化

当用户未手动选择主题（首次启动，localStorage 为空或值为 `auto`）时，应跟随系统主题变化：

- `useTheme` 初始化时若检测到 `auto` 模式，注册 `matchMedia('(prefers-color-scheme: dark)')` 的 `change` 监听器
- 系统主题变化时自动切换并更新 DOM（但不写入 localStorage，保持 `auto` 状态）
- 用户一旦手动点击切换按钮，写入显式 `light`/`dark`，不再跟随系统

### 6.4 CodeMirror 主题切换

CodeMirror 的主题通过 `EditorView.theme()` 静态配置，但所有颜色值引用 CSS 变量（如 `var(--cm-bg)`）。切换 `data-theme` 后：

- CSS 变量值变化 → CodeMirror DOM 元素继承新变量值 → 自动重绘
- **无需重新创建编辑器实例**，无需调用任何 CodeMirror API
- 搜索面板、行号、选区高亮等全部自动跟随

### 6.5 Excel 预览主题适配

| 项目 | 是否随主题变化 | 说明 |
| --- | --- | --- |
| 单元格内容色（文档指定） | ❌ | 文档自身颜色，不应改变 |
| 单元格边框色（文档指定） | ❌ | 文档自身颜色 |
| 默认网格线边框 | ✅ | `--excel-border-default: var(--text-primary)`，自动跟随 |
| 行列头背景 | ✅ | `--bg-header`，自动跟随 |
| 工具栏背景 | ✅ | `--bg-toolbar`，自动跟随 |

`composables/excel/styles.js` 的 `getDefaultBorderColor()` 优先读取 CSS 变量，暗色下自动获取新的 `--excel-border-default` 值。JS 中硬编码的 `#1f2937` 回退值仅在 `getComputedStyle` 不可用时生效，正常流程不会命中。

### 6.6 layui-vue 组件暗色适配

`theme.css` 中已对齐部分 layui 全局变量（`--global-*`）。暗色覆写中补充了中性色变量（`--global-neutral-color-4/5/7`）。可能存在的遗留：

- layui 弹窗、下拉等组件若有内部硬编码颜色，需额外覆写
- 建议在暗色主题测试阶段逐一检查 layui 组件表现，按需补充覆写

### 6.7 图片预览背景

图片预览区 `--bg-image-viewer` 在暗色下为深色背景。对于透明背景的 PNG/SVG，深色背景可能影响观感。可考虑：

- 保持现状（深色背景，与多数图片查看器一致）
- 或为图片预览区提供独立的浅色切换开关（不在本期范围）

### 6.8 窗口背景色 API 确认

`win.SetBackgroundColour()` 为 Wails v3 假设 API。实施前需确认：

1. 当前 Wails v3 版本是否支持运行时修改窗口背景色
2. 若不支持，降级方案：仅在 `createMainWindow` 时设置背景色，切换主题时接受 webview 区域外的短暂色差（webview 内部已通过 CSS 正确切换，影响极小）

### 6.9 拖拽区与空状态

`--bg-dropzone` 在暗色下为深色，虚线边框 `--border-dashed` 同步加深。hero 首页的拖拽区在暗色下视觉正常，无需额外处理。

### 6.10 主题色对比度

暗色主题的强调色从 `#2563eb`（明色蓝）调整为 `#388bfd`（暗色蓝，更亮），确保在深色背景上的对比度满足 WCAG AA 标准（≥4.5:1）。各状态色（error/warning/info）同步提亮。

---

## 7. 测试计划

### 7.1 功能测试

| 测试项 | 步骤 | 预期结果 |
| --- | --- | --- |
| 基本切换 | 点击标题栏主题按钮 | 全应用立即切换明/暗，图标随之变化 |
| 持久化-明 | 切换到明色，关闭重开 | 启动即为明色，无闪烁 |
| 持久化-暗 | 切换到暗色，关闭重开 | 启动即为暗色，无白屏闪烁 |
| 首次启动-系统明 | 系统为明色，清除 localStorage | 启动为明色 |
| 首次启动-系统暗 | 系统为暗色，清除 localStorage | 启动为暗色，无白屏闪烁 |
| 多窗口同步 | 窗口 A 切换主题 | 窗口 B 同步切换 |
| 多窗口-新窗口 | 窗口 A 为暗色，新建窗口 | 新窗口启动即为暗色，无闪烁 |
| CodePreview | 暗色下打开代码文件 | 编辑器背景、行号、语法高亮、搜索面板均为暗色 |
| ExcelPreview | 暗色下打开 xlsx | 网格线、行列头、工具栏为暗色；单元格内容色不变 |
| WordPreview | 暗色下打开 docx | 预览区背景为暗色，文档页面投影正常 |
| ImagePreview | 暗色下打开图片 | 预览区背景为暗色，工具栏正常 |
| PdfPreview | 暗色下打开 pdf | 工具栏为暗色，PDF 内容不变 |
| 标题栏按钮 | 暗色下 hover 各按钮 | hover 态正常，关闭按钮 hover 为红色 |
| 文件菜单 | 暗色下打开文件下拉菜单 | 菜单面板背景、边框、hover 态正常 |
| 文件树 | 暗色下浏览文件树 | 节点文字、hover/active 态、图标正常 |
| Tab 标签 | 暗色下多 Tab 切换 | Tab 文字、active 态、关闭按钮正常 |
| 拖拽区 | 暗色下 hero 页拖拽 | 虚线边框、图标、文字正常 |
| 状态色 | 暗色下触发 dirty/saving/error | 各状态色在暗色背景下清晰可辨 |

### 7.2 回归测试

| 测试项 | 说明 |
| --- | --- |
| 明色主题完整回归 | 切换回明色后所有功能与改动前一致 |
| 文件预览各类型 | 明/暗主题下均能正常预览所有支持格式 |
| 文件编辑与保存 | 明/暗主题下代码编辑、自动保存、编码切换正常 |
| 多窗口去重 | 主题切换不影响多窗口路径去重逻辑 |
| 窗口控制 | 主题按钮不影响最小化/最大化/关闭按钮功能 |
| 性能 | 快速连续切换主题无明显卡顿 |

### 7.3 边界测试

| 测试项 | 步骤 | 预期结果 |
| --- | --- | --- |
| localStorage 不可用 | 禁用 localStorage 后切换主题 | 主题切换正常，仅本次会话有效 |
| 系统主题运行时变化 | 未手动选择时切换系统主题 | 应用跟随系统切换 |
| 手动后不再跟随系统 | 手动切换后改变系统主题 | 应用保持用户选择 |
| 极端窗口尺寸 | 小窗口下切换主题 | 按钮不溢出，布局正常 |

---

## 8. 实施计划

### 第一期：前端主题切换（核心）

> 目标：实现明暗主题切换功能，前端自包含可用。

1. `theme.css` 新增 `[data-theme="dark"]` 暗色变量覆写块
2. 新增 `composables/useTheme.js` 主题状态管理
3. `TitleBar.vue` 新增主题切换按钮
4. `index.html` 新增 FOUC 防护内联脚本
5. 前端功能测试 + 暗色视觉走查

### 第二期：多窗口同步 + 后端配合

> 目标：多窗口主题同步，窗口背景色动态更新。

1. `App.vue` 监听后端 `theme-changed` 事件
2. `app.go` 新增 `SetWindowBackground` / `BroadcastThemeChange` 方法
3. `main.go` `createMainWindow` 适配背景色参数
4. 重新生成前端绑定
5. 多窗口同步测试 + 窗口背景色测试

### 第三期：打磨优化（可选）

> 目标：完善细节体验。

1. layui-vue 组件暗色覆写补全
2. 图片预览区暗色体验优化
3. 主题切换过渡动画（可选 `transition: background-color 0.2s, color 0.2s`）
4. 系统主题偏好实时跟随（`auto` 模式）

---

## 9. 工作量估算

| 模块 | 工作量 | 风险 |
| --- | --- | --- |
| `theme.css` 暗色变量定义 | 0.5 天 | 低（变量体系已完善） |
| `useTheme.js` 状态管理 | 0.5 天 | 低 |
| `TitleBar.vue` 按钮 | 0.5 天 | 低 |
| `index.html` FOUC 脚本 | 0.5 天 | 低 |
| `App.vue` 事件监听 | 0.5 天 | 低 |
| 后端方法 + 窗口背景色 | 0.5 天 | 中（需确认 Wails API） |
| 暗色视觉走查与调色 | 1 天 | 中（需逐组件验证） |
| 测试 | 0.5 天 | - |
| **合计** | **约 4 天** | |

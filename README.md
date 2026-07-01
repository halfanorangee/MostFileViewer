# MostFileViewer · 万能预览器

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go)](https://go.dev/)
[![Wails](https://img.shields.io/badge/Wails-v3_alpha-FF3850?logo=wails)](https://wails.io/)
[![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

一款跨平台桌面文件预览工具，无需打开多个应用即可快速预览 Word、Excel、PowerPoint、PDF、图片、代码等多种文件格式。基于 Wails v3 + Vue 3 + Go 构建。

> 🚧 目前为 **v0.0.1** 预览版，功能迭代中。

---

## 功能特色

| 功能 | 说明 |
|------|------|
| **📄 文档预览** | 支持 Word (.docx)、Excel (.xlsx/.xls/.csv)、PowerPoint (.pptx) 渲染 |
| **📖 PDF 预览** | 内嵌 PDF 渲染，支持缩放 |
| **🖼️ 图片预览** | 支持常见图片格式 (JPG, PNG, BMP, GIF, WebP, SVG, TIFF, ICO) |
| **📝 代码预览** | 基于 CodeMirror 6 的语法高亮，支持 15+ 语言，**支持编辑并保存** |
| **🌐 HTML 预览** | 独立的单网页文件渲染，如同在浏览器中打开 |
| **🔤 编码检测** | 自动检测 UTF-8 / UTF-16 / GBK / Big5 / ISO-8859-1，可手动切换，编码感知保存 |
| **📂 文件夹浏览** | 延迟加载的目录树 + 虚拟滚动，支持 VSCode 风格文件图标 |
| **📑 多标签页** | 同一窗口内多文件切换浏览 |
| **🪟 多窗口** | 支持打开多个独立窗口，路径跨窗口去重、焦点切换 |
| **💾 会话恢复** | 自动记忆已打开的文件，下次启动自动恢复 |
| **🌙 主题切换** | 亮色 / 暗色主题，启动时防 FOUC 闪烁 |
| **📂 拖拽打开** | 支持从系统文件管理器拖入文件/文件夹 |
| **🔍 在文件管理器中显示** | 右键菜单快速定位文件 |
| **✍️ 文本编辑** | 代码文件内联编辑，原子写入保存（临时文件 + 重命名） |

---

## 支持的文件类型

| 类别 | 格式 |
|------|------|
| 文档 | `.docx`（Word）、`.xlsx` / `.xls` / `.csv`（Excel）、`.pptx`（PowerPoint） |
| 文档 | `.pdf`（PDF） |
| 代码 | `.js` / `.ts` / `.go` / `.py` / `.java` / `.c` / `.cpp` / `.rs` / `.vue` / `.css` / `.html` / `.json` / `.xml` / `.yaml` / `.md` 等 |
| 图片 | `.jpg` / `.png` / `.bmp` / `.gif` / `.webp` / `.svg` / `.tiff` / `.ico` |
| 网页 | `.html` / `.htm`（独立渲染） |

---

## 截图

> (待补充)

---

## 快速开始

### 前置要求

- **Go** 1.25+
- **Node.js** 18+ / **Bun**
- **Wails v3 CLI**

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

- **Windows:** WebView2 运行时（Windows 11 自带，Windows 10 需安装）
- **macOS:** 内置 WebKit
- **Linux:** 需安装 WebKitGTK

### 开发模式（热重载）

```bash
# 使用 Wails CLI
wails3 dev

# 或使用 Task runner
task dev
```

### 构建

```bash
# 生产构建
wails3 build

# 或使用 Task runner
task build
```

构建产物：`bin/MostFileViewer.exe`（请根据平台确认）

### 打包安装程序

```bash
task package
```

产物：`bin/MostFileViewer-amd64-installer.exe`

## 项目结构

```text
├── main.go                  # 入口
├── app.go                   # Go 后端核心（约 1400 行）
├── go.mod / go.sum          # Go 模块依赖
├── Taskfile.yml             # 根 Task runner 任务配置
│
├── build/                   # 构建配置与平台资源
│   ├── config.yml           # Wails 项目配置
│   ├── appicon.png          # 应用图标源文件
│   ├── windows/             # Windows 构建资源（图标、Manifest、NSIS 安装脚本）
│   ├── darwin/              # macOS 构建资源
│   └── linux/               # Linux 构建资源
│
├── frontend/                # Vue 3 前端
│   ├── index.html           # 入口 HTML（含主题防闪脚本）
│   ├── vite.config.js       # Vite 配置
│   ├── package.json         # NPM 依赖
│   └── src/
│       ├── main.js              # Vue 启动入口
│       ├── App.vue              # 根组件（布局、标签页、拖放）
│       ├── style.css            # 全局样式
│       ├── theme.css            # 亮/暗主题定义（CSS 自定义属性）
│       ├── components/
│       │   ├── CodePreview.vue      # 代码预览/编辑（CodeMirror 6）
│       │   ├── ExcelPreview.vue     # Excel/CSV 预览
│       │   ├── PdfPreview.vue       # PDF 预览
│       │   ├── PptPreview.vue       # PPT 预览
│       │   ├── WordPreview.vue      # Word 预览
│       │   ├── ImagePreview.vue     # 图片预览
│       │   ├── FileTree.vue         # 目录树（侧边栏）
│       │   ├── FileTreeNode.vue     # 树节点（延迟加载子节点）
│       │   ├── PreviewTabs.vue      # 顶部标签栏
│       │   ├── TitleBar.vue         # 自定义标题栏
│       │   └── FileIcon.vue         # VSCode 风格文件图标
│       └── composables/
│           └── useTheme.js          # 主题管理
│
├── filemanager_windows.go   # Windows 资源管理器打开
├── filemanager_other.go     # macOS/Linux 资源管理器打开
└── bin/                     # 构建输出目录
```

---

## 技术栈

### 后端 (Go)

| 依赖 | 用途 |
|------|------|
| **Wails v3** (alpha.74) | 桌面应用框架（WebView2 / WebKit） |
| **golang.org/x/text** | 文本编码检测与转换（GBK、Big5、UTF-16 等） |
| **golang.org/x/sys** | Windows COM 接口调用 |

### 前端 (JavaScript)

| 依赖 | 用途 |
|------|------|
| **Vue 3** + **Vite 5** | 前端框架与构建 |
| **LayUI Vue** | UI 组件库 |
| **CodeMirror 6** (+15 语言包) | 代码编辑器与语法高亮 |
| **@wailsio/runtime** | Wails JS 运行时桥接 |
| **docx-preview** | Word 文档渲染 |
| **exceljs** + **papaparse** | Excel / CSV 渲染 |
| **pptx-preview** | PowerPoint 渲染 |
| **@tanstack/vue-virtual** | 虚拟滚动（大目录树） |
| **vscode-icons-js** | VSCode 风格文件图标 |

---

## 编码支持

针对中文用户场景，内置了完善的编码检测与转换能力：

| 编码 | 检测方式 |
|------|----------|
| UTF-8 | BOM 检测 + 有效性校验 |
| UTF-16 LE/BE | BOM + Null Byte 模式 |
| GBK / GB2312 / GB18030 | 评分式检测（偏好 CJK 字符）|
| Big5 | 评分式检测（偏好 CJK 字符）|
| ISO-8859-1 | 回退方案 |

支持在预览时手动切换编码并重新加载，编辑保存时自动将内容按原编码写出。

---

## 会话持久化

打开的标签页和窗口状态保存在：

**Windows:** `%APPDATA%/MostFileViewer/session.json`

下次启动自动恢复上次关闭时的浏览状态。

---

## 贡献

目前为个人项目，欢迎提 Issue 或 PR。

---

## 许可证

本项目暂无显式许可证声明。如有需要请联系作者。

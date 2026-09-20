package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/fsnotify/fsnotify"
	"github.com/wailsapp/wails/v3/pkg/application"
	textencoding "golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	textunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

// windowState 保存单个窗口的隔离状态
type windowState struct {
	currentRoot    string
	allowedFiles   map[string]struct{}
	openPaths      map[string]struct{}           // 当前窗口已打开的文件/文件夹路径集合
	internalWrites map[string]internalWriteState // 应用自身正在执行或刚完成的保存路径
	mu             sync.Mutex
	watcher        *fsnotify.Watcher   // 当前工作区的 fsnotify 监听器；nil 表示未启动或启动失败
	watchedDirs    map[string]struct{} // 已加入 watcher 的目录（绝对路径），单调增长；窗口关闭/工作区切换时清空
}

type internalWriteState struct {
	active     int
	quietUntil time.Time
}

type App struct {
	states          sync.Map // windowID(uint) → *windowState
	pathOwners      sync.Map // canonicalPath(string) → windowID(uint)
	sessionMu       sync.Mutex
	restoreSessions map[uint]*WindowSession
	windowSessions  map[uint]WindowSession
	activeWindows   map[uint]struct{}
	wailsApp        *application.App // 由 main.go 在 app.Run 之前注入，用于按窗口 EmitEvent
	mediaServer     *mediaServer     // 由 main.go 在 app.Run 之前注入，用于给视频/音频分配 Range 流式 token
}

type FileTreeNode struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Type      string         `json:"type"`
	Extension string         `json:"extension"`
	Loaded    bool           `json:"loaded"`
	HasChild  bool           `json:"hasChild"`
	Children  []FileTreeNode `json:"children,omitempty"`
}

// FsChangePayload 是按窗口推送的「父目录级批量变化」事件，per-parent 100ms 防抖合并后发出。
// 前端最低开销反应是「重扫 parent」，不需要为 50 个事件分别 diff。
type FsChangePayload struct {
	Parent  string         `json:"parent"`
	Changes []FsChangeItem `json:"changes"`
}

type FsChangeItem struct {
	Path  string `json:"path"`
	Op    string `json:"op"`    // create/remove/rename/write/chmod，合并时取最高优先级
	IsDir bool   `json:"isDir"` // flush 时由 os.Stat 决定
}

const internalWriteQuietPeriod = 750 * time.Millisecond

func normalizeWatchPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	normalized := filepath.Clean(absPath)
	if runtime.GOOS == "windows" {
		normalized = strings.ToLower(normalized)
	}
	return normalized
}

// fileRemovedPayload 是独立事件，给 tab 联动专用：避免在 fs-change 处理完后还要二次扫所有 tab。
type fileRemovedPayload struct {
	Path string `json:"path"`
}

type FileInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	IsDir     bool   `json:"isDir"`
	Extension string `json:"extension"`
	ModTime   string `json:"modTime"`
	Mode      string `json:"mode"`
	IsSymlink bool   `json:"isSymlink"`
}

type FileContent struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Extension string `json:"extension"`
	Size      int64  `json:"size"`
	Base64    string `json:"base64"`
	Content   string `json:"content"`
	Encoding  string `json:"encoding"`
}

type FileChunk struct {
	Base64 string `json:"base64"`
	Size   int    `json:"size"`
}

type SessionTab struct {
	Path string `json:"path"`
}

// SessionSplitNode 描述分屏布局树的一个节点：
// Type 为 "pane" 时是叶子（Tabs 为该 pane 内的 tab 顺序，Active 为其激活项），
// Type 为 "split" 时是分支（Direction 为 row/column，Ratio 为第一个子节点的占比）。
type SessionSplitNode struct {
	Type      string             `json:"type"`
	ID        string             `json:"id,omitempty"`
	Tabs      []string           `json:"tabs,omitempty"`
	Active    string             `json:"active,omitempty"`
	Direction string             `json:"direction,omitempty"`
	Ratio     float64            `json:"ratio,omitempty"`
	Children  []SessionSplitNode `json:"children,omitempty"`
}

// SessionPreviewLayout 是预览区分屏布局的持久化结构。
type SessionPreviewLayout struct {
	Root          *SessionSplitNode `json:"root,omitempty"`
	FocusedPaneID string            `json:"focusedPaneId,omitempty"`
}

type WindowSession struct {
	HasSession    bool                  `json:"hasSession,omitempty"`
	Mode          string                `json:"mode"`
	RootPath      string                `json:"rootPath"`
	OpenTabs      []SessionTab          `json:"openTabs"`
	ActivePath    string                `json:"activePath"`
	SidebarOpen   bool                  `json:"sidebarOpen"`
	LeftPaneWidth int                   `json:"leftPaneWidth"`
	Layout        *SessionPreviewLayout `json:"layout,omitempty"`
}

type AppSession struct {
	Version   int             `json:"version"`
	Windows   []WindowSession `json:"windows"`
	UpdatedAt string          `json:"updatedAt"`
}

var revealPathInFileManager = openInFileManager

// openMediaWithSystemCaller 通过包级变量间接调用平台实现，测试中可替换为 stub，
// 避免单测真实启动外部播放器。
var openMediaWithSystemCaller = openMediaWithSystem

func NewApp() *App {
	return &App{
		restoreSessions: make(map[uint]*WindowSession),
		windowSessions:  make(map[uint]WindowSession),
		activeWindows:   make(map[uint]struct{}),
	}
}

// windowIDFromCtx 从 context 提取窗口 ID
func windowIDFromCtx(ctx context.Context) uint {
	if win, ok := ctx.Value(application.WindowKey).(application.Window); ok {
		return win.ID()
	}
	return 0
}

// getOrCreateState 获取或创建窗口状态
func (a *App) getOrCreateState(ctx context.Context) *windowState {
	id := windowIDFromCtx(ctx)
	if id == 0 {
		// 降级：无窗口上下文时返回临时状态（不应出现在正常流程中）
		return &windowState{allowedFiles: make(map[string]struct{})}
	}
	actual, _ := a.states.LoadOrStore(id, &windowState{
		allowedFiles:   make(map[string]struct{}),
		openPaths:      make(map[string]struct{}),
		internalWrites: make(map[string]internalWriteState),
	})
	return actual.(*windowState)
}

// beginInternalWrite marks a save initiated by this application. fsnotify may
// report the replace as rename/remove/create; those events must not be treated
// as an external move of the open tab.
func (a *App) beginInternalWrite(ctx context.Context, path string) func() {
	state := a.getOrCreateState(ctx)
	key := normalizeWatchPath(path)
	state.mu.Lock()
	if state.internalWrites == nil {
		state.internalWrites = make(map[string]internalWriteState)
	}
	current := state.internalWrites[key]
	current.active++
	current.quietUntil = time.Time{}
	state.internalWrites[key] = current
	state.mu.Unlock()

	return func() {
		state.mu.Lock()
		if current, ok := state.internalWrites[key]; ok {
			if current.active > 0 {
				current.active--
			}
			if current.active == 0 {
				current.quietUntil = time.Now().Add(internalWriteQuietPeriod)
			}
			state.internalWrites[key] = current
		}
		state.mu.Unlock()
	}
}

func (a *App) isInternalWrite(windowID uint, path string) bool {
	if windowID == 0 {
		return false
	}
	value, ok := a.states.Load(windowID)
	if !ok {
		return false
	}
	state := value.(*windowState)
	key := normalizeWatchPath(path)
	now := time.Now()
	state.mu.Lock()
	defer state.mu.Unlock()

	for candidate, write := range state.internalWrites {
		if write.active == 0 && now.After(write.quietUntil) {
			delete(state.internalWrites, candidate)
		}
	}
	write, ok := state.internalWrites[key]
	return ok && (write.active > 0 || write.quietUntil.After(now))
}

func isAtomicTempPath(path string) bool {
	name := strings.ToLower(filepath.Base(path))
	return strings.Contains(name, ".tmp-") || strings.Contains(name, ".bak-")
}

// canonicalizePath 对路径进行统一规范化，用于去重比较
func canonicalizePath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	cleaned := filepath.Clean(absPath)

	// 尽量解析符号链接/junction
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		cleaned = resolved
	}

	// Windows 下路径大小写不敏感，做归一化
	if runtime.GOOS == "windows" {
		cleaned = strings.ToLower(cleaned)
	}

	return cleaned, nil
}

func (a *App) SelectFile(ctx context.Context) (string, error) {
	dialog := application.Get().Dialog.OpenFile().
		CanChooseDirectories(false).
		CanChooseFiles(true).
		SetTitle("选择需要预览的文件")

	// 多窗口下将对话框归属到调用窗口
	if win, ok := ctx.Value(application.WindowKey).(application.Window); ok {
		dialog.AttachToWindow(win)
	}

	selectedPath, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("打开文件选择框失败: %w", err)
	}
	if selectedPath == "" {
		return "", errors.New("已取消选择文件")
	}

	cleanPath, err := filepath.Abs(selectedPath)
	if err != nil {
		return "", fmt.Errorf("解析文件路径失败: %w", err)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", fmt.Errorf("文件不存在或无法访问: %w", err)
	}
	if info.IsDir() {
		return "", errors.New("所选路径不是文件")
	}

	state := a.getOrCreateState(ctx)
	allowFile(state, cleanPath)
	return cleanPath, nil
}

func (a *App) SelectFolder(ctx context.Context) (string, error) {
	dialog := application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("选择需要预览的文件夹")

	// 多窗口下将对话框归属到调用窗口
	if win, ok := ctx.Value(application.WindowKey).(application.Window); ok {
		dialog.AttachToWindow(win)
	}

	selectedPath, err := dialog.PromptForSingleSelection()
	if err != nil {
		return "", fmt.Errorf("打开文件夹选择框失败: %w", err)
	}
	if selectedPath == "" {
		return "", errors.New("已取消选择文件夹")
	}

	cleanPath, err := filepath.Abs(selectedPath)
	if err != nil {
		return "", fmt.Errorf("解析文件夹路径失败: %w", err)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", fmt.Errorf("文件夹不存在或无法访问: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("所选路径不是文件夹")
	}

	return cleanPath, nil
}

func (a *App) LoadFolderTree(ctx context.Context, root string) ([]FileTreeNode, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("文件夹路径不能为空")
	}

	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("解析文件夹路径失败: %w", err)
	}

	info, err := os.Stat(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("文件夹不存在或无法访问: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("所选路径不是文件夹")
	}

	nodes, err := scanFolderTree(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("扫描文件夹失败: %w", err)
	}

	state := a.getOrCreateState(ctx)
	state.mu.Lock()
	sameRoot := normalizeWatchPath(state.currentRoot) == normalizeWatchPath(cleanRoot)
	if !sameRoot {
		state.currentRoot = cleanRoot
		state.allowedFiles = make(map[string]struct{})
	}
	state.mu.Unlock()

	// 切换工作区时重建 watcher；同一根目录刷新只重扫树，保留已展开目录的监听与白名单。
	if !sameRoot {
		a.resetWatcher(state, cleanRoot)
		if err := a.addWatchDir(state, cleanRoot); err != nil {
			log.Printf("fswatch: Add 根目录失败 (window=%d, root=%s): %v", windowIDFromCtx(ctx), cleanRoot, err)
		}
	}
	return nodes, nil
}

func (a *App) LoadFolderChildren(ctx context.Context, path string) ([]FileTreeNode, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析文件夹路径失败: %w", err)
	}

	state := a.getOrCreateState(ctx)
	state.mu.Lock()
	if strings.TrimSpace(state.currentRoot) == "" {
		state.currentRoot = cleanPath
	}
	root := state.currentRoot
	state.mu.Unlock()

	if !isPathWithinRoot(root, cleanPath) {
		return nil, errors.New("当前文件夹不在已打开文件夹范围内")
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("文件夹不存在或无法访问: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("当前路径不是文件夹")
	}

	nodes, err := scanFolderLevel(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("扫描文件夹失败: %w", err)
	}

	// 首次展开时把该目录加入 watcher：已加则跳过；Add 失败仅 log，不阻断 children 返回。
	if err := a.addWatchDir(state, cleanPath); err != nil {
		log.Printf("fswatch: Add 子目录失败 (window=%d, path=%s): %v", windowIDFromCtx(ctx), cleanPath, err)
	}
	return nodes, nil
}

// WatchFolder 由前端 FileTreeNode 首次展开时调用，请求把指定目录加入本窗口的 watcher。
// watching==false 本版本不实现：watch 集合单调增长，折叠不摘除（避免重新展开时再走 fs 同步）。
// 路径必须走 filepath.Abs 清洗，避免斜杠方向差异导致 watcher 内部去重失效。
func (a *App) WatchFolder(ctx context.Context, path string, watching bool) error {
	if !watching {
		return nil
	}
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析路径失败: %w", err)
	}

	state := a.getOrCreateState(ctx)
	state.mu.Lock()
	if strings.TrimSpace(state.currentRoot) == "" {
		state.currentRoot = cleanPath
	}
	root := state.currentRoot
	state.mu.Unlock()

	if !isPathWithinRoot(root, cleanPath) {
		return errors.New("当前文件夹不在已打开文件夹范围内")
	}

	return a.addWatchDir(state, cleanPath)
}

// addWatchDir 把目录加入窗口 watcher。已加则直接返回 nil；watcher 为 nil（启动失败）也直接返回 nil。
// 不持有 state.mu 进入（调用方需在锁外调用），避免与 consumeWatcherEvents 互锁。
func (a *App) addWatchDir(state *windowState, dir string) error {
	if state == nil {
		return nil
	}
	state.mu.Lock()
	watcher := state.watcher
	if watcher == nil {
		state.mu.Unlock()
		return nil
	}
	if _, ok := state.watchedDirs[dir]; ok {
		state.mu.Unlock()
		return nil
	}
	state.mu.Unlock()

	if err := watcher.Add(dir); err != nil {
		return err
	}

	state.mu.Lock()
	if state.watchedDirs == nil {
		state.watchedDirs = make(map[string]struct{})
	}
	// 二次检查：避免并发 Add 后重复写入
	if _, exists := state.watchedDirs[dir]; !exists {
		state.watchedDirs[dir] = struct{}{}
	}
	state.mu.Unlock()
	return nil
}

// resetWatcher 关闭旧 watcher 并重建一个新的；启动消费 goroutine。
// 用于 LoadFolderTree 成功路径（切换工作区或首次打开）；不持有 state.mu。
func (a *App) resetWatcher(state *windowState, newRoot string) {
	if state == nil {
		return
	}

	state.mu.Lock()
	oldWatcher := state.watcher
	state.watcher = nil
	state.watchedDirs = nil
	state.mu.Unlock()

	// Close 旧 watcher：幂等；Close 后 Events channel 关闭，消费 goroutine 自然退出。
	if oldWatcher != nil {
		_ = oldWatcher.Close()
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("fswatch: NewWatcher 失败 (root=%s): %v", newRoot, err)
		return
	}

	state.mu.Lock()
	state.watcher = watcher
	state.watchedDirs = make(map[string]struct{})
	state.mu.Unlock()

	// 找 windowID 启动消费循环：state 在 a.states 中，需要扫一遍拿 key
	windowID := uint(0)
	a.states.Range(func(key, value any) bool {
		if value.(*windowState) == state {
			if id, ok := key.(uint); ok {
				windowID = id
			}
			return false
		}
		return true
	})
	if windowID == 0 {
		// 兜底：拿不到 windowID 时不强启动，避免静默 goroutine
		log.Printf("fswatch: 找不到 windowID，放弃启动消费循环 (root=%s)", newRoot)
		return
	}

	log.Printf("fswatch started (window=%d, root=%s)", windowID, newRoot)
	go a.consumeWatcherEvents(windowID, watcher)
}

// consumeWatcherEvents 读取 watcher 的 Events/Errors，per-parent 100ms 防抖后按窗口 EmitEvent。
// 同一 parent 内多事件按 op 优先级合并（Remove > Rename > Create > Write > Chmod）。
// op=remove && !isDir 时额外 emit file-removed 给 tab 联动专用。
// watcher.Events 关闭时 goroutine 自然 return。
func (a *App) consumeWatcherEvents(windowID uint, watcher *fsnotify.Watcher) {
	// defer 兜底日志：Close 后 Events 关闭，循环退出
	defer log.Printf("fswatch closed (window=%d)", windowID)

	const debounce = 100 * time.Millisecond
	timers := make(map[string]*time.Timer)
	pending := make(map[string]map[string]string) // parent -> path -> op

	opPriority := func(op string) int {
		switch op {
		case "remove":
			return 5
		case "rename":
			return 4
		case "create":
			return 3
		case "write":
			return 2
		case "chmod":
			return 1
		default:
			return 0
		}
	}

	toOp := func(op fsnotify.Op) string {
		switch {
		case op&fsnotify.Create != 0:
			return "create"
		case op&fsnotify.Remove != 0:
			return "remove"
		case op&fsnotify.Rename != 0:
			return "rename"
		case op&fsnotify.Write != 0:
			return "write"
		case op&fsnotify.Chmod != 0:
			return "chmod"
		default:
			return "unknown"
		}
	}

	flush := func(parent string) {
		items, ok := pending[parent]
		if !ok || len(items) == 0 {
			delete(timers, parent)
			return
		}
		delete(pending, parent)
		delete(timers, parent)

		changes := make([]FsChangeItem, 0, len(items))
		for path, op := range items {
			info, err := os.Stat(path)
			isDir := err == nil && info.IsDir()
			// 文件已不存在时，fsnotify 报 Remove，isDir=false 即可（info 拿不到）
			changes = append(changes, FsChangeItem{
				Path:  path,
				Op:    op,
				IsDir: isDir,
			})
		}

		if a.wailsApp == nil {
			return
		}
		win, exists := a.wailsApp.Window.GetByID(windowID)
		if !exists {
			return
		}

		win.EmitEvent("fs-change", FsChangePayload{
			Parent:  parent,
			Changes: changes,
		})
		for _, item := range changes {
			if item.Op == "remove" && !item.IsDir {
				win.EmitEvent("file-removed", fileRemovedPayload{Path: item.Path})
			}
		}
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				// channel 关闭：把还没 flush 的全部 flush 出去
				for parent, t := range timers {
					t.Stop()
					flush(parent)
				}
				return
			}
			// 原子保存会在同一目录创建 .tmp-* / .bak-* 文件；这些是实现细节，
			// 不应刷新文件树，也不应参与外部改名/删除判断。
			if isAtomicTempPath(event.Name) {
				continue
			}
			parent := filepath.Dir(event.Name)
			op := toOp(event.Op)
			if a.isInternalWrite(windowID, event.Name) {
				// 自身保存即使被 Windows 报告为 rename/remove，也只是 write。
				op = "write"
			}

			// 防抖：如果已有 timer，停止并重置
			if t, ok := timers[parent]; ok {
				t.Stop()
			}
			if pending[parent] == nil {
				pending[parent] = make(map[string]string)
			}
			// 同一 path 在窗口内多次事件：保留优先级最高者
			cur, exists := pending[parent][event.Name]
			if !exists || opPriority(op) > opPriority(cur) {
				pending[parent][event.Name] = op
			}
			timers[parent] = time.AfterFunc(debounce, func() {
				flush(parent)
			})
		case err, ok := <-watcher.Errors:
			if !ok {
				for parent, t := range timers {
					t.Stop()
					flush(parent)
				}
				return
			}
			log.Printf("fswatch error (window=%d): %v", windowID, err)
		}
	}
}

func (a *App) ReadFile(ctx context.Context, path string) (*FileContent, error) {
	return a.readFile(ctx, path, "")
}

func (a *App) ReadFileWithEncoding(ctx context.Context, path string, encoding string) (*FileContent, error) {
	return a.readFile(ctx, path, encoding)
}

func (a *App) readFile(ctx context.Context, path string, requestedEncoding string) (*FileContent, error) {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return nil, err
	}

	extension := strings.ToLower(filepath.Ext(info.Name()))
	content := &FileContent{
		Name:      info.Name(),
		Path:      cleanPath,
		Extension: extension,
		Size:      info.Size(),
	}

	if isOfficePreviewExtension(extension) || isImagePreviewExtension(extension) || isPdfPreviewExtension(extension) || isMediaPreviewExtension(extension) {
		return content, nil
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var encoding string
	if strings.TrimSpace(requestedEncoding) != "" {
		encoding, err = normalizeTextEncoding(requestedEncoding)
		if err != nil {
			return nil, err
		}
	} else {
		encoding = detectTextEncoding(data)
	}

	if extension == ".csv" {
		content.Base64 = base64.StdEncoding.EncodeToString(data)
		content.Encoding = encoding
		return content, nil
	}

	text, err := decodeText(data, encoding)
	if err != nil {
		return nil, fmt.Errorf("解码文件内容失败: %w", err)
	}
	content.Content = text
	content.Encoding = encoding
	return content, nil
}

func (a *App) ReadFileChunk(ctx context.Context, path string, offset int64, size int) (*FileChunk, error) {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return nil, err
	}
	if offset < 0 {
		return nil, errors.New("读取偏移量不能为负数")
	}
	if size <= 0 {
		return nil, errors.New("读取大小必须大于 0")
	}
	const maxChunkSize = 2 * 1024 * 1024
	if size > maxChunkSize {
		size = maxChunkSize
	}
	if offset >= info.Size() {
		return &FileChunk{}, nil
	}

	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	buffer := make([]byte, size)
	n, err := file.ReadAt(buffer, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("读取文件分块失败: %w", err)
	}

	return &FileChunk{
		Base64: base64.StdEncoding.EncodeToString(buffer[:n]),
		Size:   n,
	}, nil
}

// isMediaPreviewExtension 判断给定后缀是否走音视频 Range 流式预览。
// 命中后前端不再把整文件读入 ArrayBuffer，而是请求后端签发一个 /media/<token>
// 的临时 URL，由 <video>/<audio> 直接走 HTTP Range 拉取片段。
var mediaPreviewExtensions = map[string]struct{}{
	".mp3": {}, ".wav": {}, ".ogg": {}, ".oga": {}, ".flac": {}, ".m4a": {}, ".aac": {}, ".wma": {},
	".mp4": {}, ".webm": {}, ".mov": {}, ".avi": {}, ".mkv": {}, ".flv": {}, ".wmv": {},
}

func isMediaPreviewExtension(extension string) bool {
	_, ok := mediaPreviewExtensions[strings.ToLower(extension)]
	return ok
}

// MediaTokenResult 是 RegisterMediaToken 的返回值。前端拿到 token 后拼成
// `/media/<token>` 即可交给 <video>/<audio>；ModifiedAt 供前端在重新加载时
// 判断文件是否已被外部替换。
type MediaTokenResult struct {
	Token      string    `json:"token"`
	URL        string    `json:"url"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modifiedAt"`
}

// RegisterMediaToken 给指定文件签发一个短时 token，并把拼好的 URL 返回给前端。
// 仅允许音视频后缀：避免被滥用做任意文件代理。
func (a *App) RegisterMediaToken(ctx context.Context, path string) (MediaTokenResult, error) {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return MediaTokenResult{}, err
	}
	extension := strings.ToLower(filepath.Ext(info.Name()))
	if !isMediaPreviewExtension(extension) {
		return MediaTokenResult{}, errors.New("该文件类型不支持走媒体流式预览")
	}
	if a.mediaServer == nil {
		return MediaTokenResult{}, errors.New("媒体服务未初始化")
	}

	token, err := a.mediaServer.Register(cleanPath, info.Size(), info.ModTime().UnixNano(), windowIDFromCtx(ctx))
	if err != nil {
		return MediaTokenResult{}, err
	}
	mediaURL, err := a.mediaServer.URL(token)
	if err != nil {
		return MediaTokenResult{}, err
	}
	return MediaTokenResult{
		Token:      token,
		URL:        mediaURL,
		Size:       info.Size(),
		ModifiedAt: info.ModTime(),
	}, nil
}

// RevokeMediaToken 回收当前窗口名下的一个媒体 token。重复回收幂等（返回成功）；
// token 存在但属于其他窗口时返回权限错误。
func (a *App) RevokeMediaToken(ctx context.Context, token string) error {
	if a.mediaServer == nil {
		return errors.New("媒体服务未初始化")
	}
	if !isMediaTokenFormat(token) {
		return errors.New("无效的媒体 token")
	}
	_, foreign := a.mediaServer.Revoke(token, windowIDFromCtx(ctx))
	if foreign {
		return errors.New("不能回收其他窗口的媒体 token")
	}
	return nil
}

// isMediaTokenFormat 校验 token 形如 128bit 十六进制（32 个小写/大写十六进制字符）。
func isMediaTokenFormat(token string) bool {
	if len(token) != 32 {
		return false
	}
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}
	return true
}

// OpenMediaWithSystem 以系统默认关联程序打开已授权的媒体文件。
// WebView 无法播放的编码（MKV/AVI 等）由系统播放器兜底；只接收文件路径，
// 不把 loopback token URL 扩散给外部进程。
func (a *App) OpenMediaWithSystem(ctx context.Context, path string) error {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return err
	}
	extension := strings.ToLower(filepath.Ext(info.Name()))
	if !isMediaPreviewExtension(extension) {
		return errors.New("仅支持用系统播放器打开音视频文件")
	}
	if err := openMediaWithSystemCaller(cleanPath); err != nil {
		return fmt.Errorf("使用系统默认程序打开失败: %w", err)
	}
	return nil
}

// subtitleExtensions 是支持外挂加载的字幕文件扩展名（小写，含点）。
var subtitleExtensions = map[string]struct{}{
	".vtt": {},
	".srt": {},
	".ass": {},
	".ssa": {},
}

// maxSubtitleFileSize 限制字幕文件大小，避免把超大文件整体读入内存。
const maxSubtitleFileSize = 10 << 20 // 10MB

// ListSiblingSubtitles 查找与媒体文件同目录、同主文件名（含语言后缀变体，
// 如 movie.chs.vtt）的外挂字幕文件。仅返回当前窗口授权范围内的普通
// 文件并把结果加入白名单；没有匹配时返回空列表。
func (a *App) ListSiblingSubtitles(ctx context.Context, path string) ([]string, error) {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return nil, err
	}
	if !isMediaPreviewExtension(strings.ToLower(filepath.Ext(info.Name()))) {
		return nil, errors.New("仅支持为音视频文件查找同名字幕")
	}

	base := strings.TrimSuffix(info.Name(), filepath.Ext(info.Name()))
	dir := filepath.Dir(cleanPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("扫描同名字幕失败: %w", err)
	}

	state := a.getOrCreateState(ctx)
	var subtitlePaths []string
	for _, entry := range entries {
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if _, ok := subtitleExtensions[ext]; !ok {
			continue
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		if stem != base && !strings.HasPrefix(stem, base+".") {
			continue
		}
		subtitlePath := filepath.Join(dir, name)

		state.mu.Lock()
		withinRoot := isPathWithinRoot(state.currentRoot, subtitlePath)
		allowed := withinRoot || isAllowedFile(state, subtitlePath)
		if allowed {
			allowFile(state, subtitlePath)
		}
		state.mu.Unlock()
		if !allowed {
			continue
		}

		subtitleInfo, statErr := entry.Info()
		if statErr != nil || !subtitleInfo.Mode().IsRegular() || subtitleInfo.Size() > maxSubtitleFileSize {
			continue
		}
		subtitlePaths = append(subtitlePaths, subtitlePath)
	}

	sort.Strings(subtitlePaths)
	return subtitlePaths, nil
}

// PickSubtitleFile 弹出系统文件选择框，让用户挑选本地字幕文件。
// 用户取消时返回空串；选中后将文件加入当前窗口白名单，随后可经 ReadFile
// 读取内容（读取链路自带编码检测，GBK 等编码的文本字幕也能正确解码）。
func (a *App) PickSubtitleFile(ctx context.Context) (string, error) {
	dialog := application.Get().Dialog.OpenFile().
		CanChooseDirectories(false).
		CanChooseFiles(true).
		SetTitle("选择字幕文件").
		AddFilter("字幕文件 (*.vtt;*.srt;*.ass;*.ssa)", "*.vtt;*.srt;*.ass;*.ssa").
		AddFilter("所有文件", "*.*")

	// 多窗口下将对话框归属到调用窗口
	if win, ok := ctx.Value(application.WindowKey).(application.Window); ok {
		dialog.AttachToWindow(win)
	}

	selectedPath, err := dialog.PromptForSingleSelection()
	if err != nil {
		// Wails v3 在用户主动关闭/取消对话框时返回 cfd.ErrorCancelled（文本为 "cancelled by user"）。
		// cfd 包位于 internal/ 路径，外部模块无法 import，精确字符串匹配是当前唯一可行的检测方式。
		// 该 sentinel 在 wails/v3 多 alpha 版本下文本保持稳定。
		if err.Error() == "cancelled by user" {
			return "", nil
		}
		return "", fmt.Errorf("打开文件选择框失败: %w", err)
	}
	if selectedPath == "" {
		return "", nil
	}

	cleanPath, err := filepath.Abs(selectedPath)
	if err != nil {
		return "", fmt.Errorf("解析字幕路径失败: %w", err)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", fmt.Errorf("字幕文件不存在或无法访问: %w", err)
	}
	if info.IsDir() || !info.Mode().IsRegular() {
		return "", errors.New("所选路径不是普通文件")
	}
	if info.Size() > maxSubtitleFileSize {
		return "", errors.New("字幕文件过大（超过 10MB）")
	}

	ext := strings.ToLower(filepath.Ext(info.Name()))
	if _, ok := subtitleExtensions[ext]; !ok {
		return "", fmt.Errorf("不支持的字幕格式: %s（仅支持 .vtt / .srt / .ass / .ssa）", ext)
	}

	state := a.getOrCreateState(ctx)
	state.mu.Lock()
	allowFile(state, cleanPath)
	state.mu.Unlock()
	return cleanPath, nil
}

func isMatroskaPreviewExtension(ext string) bool {
	switch ext {
	case ".mkv", ".webm":
		return true
	default:
		return false
	}
}

// ListEmbeddedSubtitles 列出 MKV/WebM 容器内可提取的文本字幕轨（S_TEXT/UTF8、
// WebVTT、ASS/SSA）。Chromium 只会把内嵌 WebVTT 暴露为 textTracks，常见的
// SRT/ASS 轨需要在后端解析容器。图像字幕（PGS/VobSub）无法在应用内渲染，不列入。
// 非 Matroska 文件或解析失败时返回空列表，不阻断播放。
func (a *App) ListEmbeddedSubtitles(ctx context.Context, path string) ([]EmbeddedSubtitleTrack, error) {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return nil, err
	}
	if !isMatroskaPreviewExtension(strings.ToLower(filepath.Ext(info.Name()))) {
		return []EmbeddedSubtitleTrack{}, nil
	}
	tracks, err := listMatroskaSubtitleTracks(cleanPath)
	if err != nil {
		log.Printf("mkv: 列出内嵌字幕失败: %v", err)
		return []EmbeddedSubtitleTrack{}, nil
	}
	if tracks == nil {
		return []EmbeddedSubtitleTrack{}, nil
	}
	return tracks, nil
}

// ReadEmbeddedSubtitle 把指定 TrackNumber 的内嵌文本字幕提取为 WebVTT。
// 仅允许当前窗口已授权的 MKV/WebM 文件；图像轨或空轨返回可展示的错误。
func (a *App) ReadEmbeddedSubtitle(ctx context.Context, path string, trackNumber int) (string, error) {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return "", err
	}
	if !isMatroskaPreviewExtension(strings.ToLower(filepath.Ext(info.Name()))) {
		return "", errors.New("仅支持从 MKV/WebM 提取内嵌字幕")
	}
	return extractMatroskaSubtitle(cleanPath, trackNumber)
}

// SaveFile 把已存在的文本文件按指定编码原子写回。
func (a *App) SaveFile(ctx context.Context, path string, content string, encoding string) error {
	cleanPath, info, err := a.validateFilePath(ctx, path)
	if err != nil {
		return err
	}

	encoding, err = normalizeTextEncoding(encoding)
	if err != nil {
		return err
	}

	// 仅 UTF-8 编码需要回看 BOM；其他编码不依赖原文件内容，无需读取
	var existingHeader []byte
	if encoding == "utf-8" {
		existingHeader, err = readFileHeader(cleanPath, 3)
		if err != nil {
			return fmt.Errorf("读取原文件失败: %w", err)
		}
	}

	encodedContent, err := encodeText(content, encoding, existingHeader)
	if err != nil {
		return fmt.Errorf("编码文件内容失败: %w", err)
	}

	endInternalWrite := a.beginInternalWrite(ctx, cleanPath)
	defer endInternalWrite()
	if err := writeFileAtomically(cleanPath, encodedContent, info.Mode().Perm()); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	return nil
}

// SaveFileAs 弹「另存为」对话框把内容写入用户选择的新位置。用于新建空白 tab
// 的首次落地：suggestedName 提供默认文件名，defaultDirectory 提供默认打开目录
// （传空表示由对话框自行决定）。用户取消时返回空字符串与 nil 错误；
// 写入失败或路径异常时返回非 nil 错误，前端据此在状态栏显示 saveError。
func (a *App) SaveFileAs(ctx context.Context, suggestedName string, content string, encoding string, defaultDirectory string) (string, error) {
	if strings.TrimSpace(suggestedName) == "" {
		return "", errors.New("建议文件名不能为空")
	}

	normalizedEncoding, err := normalizeTextEncoding(encoding)
	if err != nil {
		return "", err
	}

	dialog := application.Get().Dialog.SaveFile().
		SetMessage("另存为 - 选择新文件的保存位置").
		SetFilename(suggestedName)
	if strings.TrimSpace(defaultDirectory) != "" {
		dialog.SetDirectory(defaultDirectory)
	}
	if win, ok := ctx.Value(application.WindowKey).(application.Window); ok {
		dialog.AttachToWindow(win)
	}

	selectedPath, err := dialog.PromptForSingleSelection()
	if err != nil {
		// Wails v3 在用户主动关闭/取消对话框时返回 cfd.ErrorCancelled（文本为 "cancelled by user"）。
		// cfd 包位于 internal/ 路径，外部模块无法 import，精确字符串匹配是当前唯一可行的检测方式。
		// 该 sentinel 在 wails/v3 多 alpha 版本下文本保持稳定。
		if err.Error() == "cancelled by user" {
			return "", nil
		}
		return "", fmt.Errorf("打开保存对话框失败: %w", err)
	}
	if selectedPath == "" {
		// 用户取消：返回空串表示「未落地」，不视为错误
		return "", nil
	}

	cleanPath, err := filepath.Abs(selectedPath)
	if err != nil {
		return "", fmt.Errorf("解析保存路径失败: %w", err)
	}

	// 新建文件使用 0644 默认权限；若目标已存在则沿用原权限，避免改变用户既有设置
	perm := os.FileMode(0o644)
	if info, statErr := os.Stat(cleanPath); statErr == nil && !info.IsDir() {
		perm = info.Mode().Perm()
	}

	encodedContent, err := encodeText(content, normalizedEncoding, nil)
	if err != nil {
		return "", fmt.Errorf("编码文件内容失败: %w", err)
	}

	endInternalWrite := a.beginInternalWrite(ctx, cleanPath)
	defer endInternalWrite()
	if err := writeFileAtomically(cleanPath, encodedContent, perm); err != nil {
		return "", fmt.Errorf("保存文件失败: %w", err)
	}

	return cleanPath, nil
}

// readFileHeader 只读取文件前 n 个字节，避免为了识别 BOM 而把整个文件加载到内存中。
// 文件比 n 短时返回实际读取到的部分（不视为错误）。
func readFileHeader(path string, n int) ([]byte, error) {
	if n <= 0 {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	buf := make([]byte, n)
	read, err := io.ReadFull(file, buf)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	return buf[:read], nil
}

func (a *App) ShowInFileManager(ctx context.Context, path string) error {
	cleanPath, info, err := a.validateFileManagerPath(ctx, path)
	if err != nil {
		return err
	}

	if err := revealPathInFileManager(cleanPath, info.IsDir()); err != nil {
		return fmt.Errorf("在文件管理器中显示失败: %w", err)
	}
	return nil
}

func (a *App) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("文件路径不能为空")
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析文件路径失败: %w", err)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在或无法访问: %w", err)
	}

	isSymlink := false
	if lstat, err := os.Lstat(cleanPath); err == nil {
		isSymlink = lstat.Mode()&os.ModeSymlink != 0
	}

	return &FileInfo{
		Name:      info.Name(),
		Path:      cleanPath,
		Size:      info.Size(),
		IsDir:     info.IsDir(),
		Extension: strings.ToLower(filepath.Ext(info.Name())),
		ModTime:   info.ModTime().Format("2006-01-02 15:04:05"),
		Mode:      info.Mode().String(),
		IsSymlink: isSymlink,
	}, nil
}

func (a *App) validateFilePath(ctx context.Context, path string) (string, os.FileInfo, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil, errors.New("文件路径不能为空")
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("解析文件路径失败: %w", err)
	}

	state := a.getOrCreateState(ctx)
	state.mu.Lock()
	defer state.mu.Unlock()

	if strings.TrimSpace(state.currentRoot) == "" {
		state.currentRoot = filepath.Dir(cleanPath)
	}
	if !isPathWithinRoot(state.currentRoot, cleanPath) && !isAllowedFile(state, cleanPath) {
		return "", nil, errors.New("当前文件不在已打开文件夹范围内")
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", nil, fmt.Errorf("文件不存在或无法访问: %w", err)
	}
	if info.IsDir() {
		return "", nil, errors.New("当前路径是文件夹，不能直接预览")
	}

	return cleanPath, info, nil
}

func (a *App) validateFileManagerPath(ctx context.Context, path string) (string, os.FileInfo, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil, errors.New("文件路径不能为空")
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("解析文件路径失败: %w", err)
	}

	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", nil, fmt.Errorf("文件不存在或无法访问: %w", err)
	}

	state := a.getOrCreateState(ctx)
	state.mu.Lock()
	defer state.mu.Unlock()

	if strings.TrimSpace(state.currentRoot) == "" {
		if info.IsDir() {
			state.currentRoot = cleanPath
		} else {
			state.currentRoot = filepath.Dir(cleanPath)
		}
	}
	if !isPathWithinRoot(state.currentRoot, cleanPath) && !isAllowedFile(state, cleanPath) {
		return "", nil, errors.New("当前文件不在已打开文件夹范围内")
	}

	return cleanPath, info, nil
}

func isOfficePreviewExtension(extension string) bool {
	switch extension {
	case ".docx", ".xlsx", ".xls", ".xlsm", ".xltx", ".xltm", ".pptx", ".pptm", ".ppsx", ".ppsm":
		return true
	default:
		return false
	}
}

func isPdfPreviewExtension(extension string) bool {
	switch extension {
	case ".pdf":
		return true
	default:
		return false
	}
}

func isImagePreviewExtension(extension string) bool {
	switch extension {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg", ".ico", ".avif":
		return true
	default:
		return false
	}
}

func writeFileAtomically(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}

	tmpName := tmp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	if err := replaceFileAtomically(path, tmpName); err != nil {
		return err
	}

	removeTemp = false
	return nil
}

func scanFolderLevel(root string) ([]FileTreeNode, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	nodes := make([]FileTreeNode, 0, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(root, entry.Name())
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		node := FileTreeNode{
			Name:      entry.Name(),
			Path:      fullPath,
			Extension: extension,
		}

		if entry.IsDir() {
			node.Type = "folder"
			node.Loaded = false
			node.HasChild = folderHasChildren(fullPath)
		} else {
			node.Type = "file"
			node.Loaded = true
		}

		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "folder"
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})

	return nodes, nil
}

func folderHasChildren(path string) bool {
	entries, err := os.ReadDir(path)
	return err == nil && len(entries) > 0
}

// scanFolderTree 递归扫描整棵文件夹树，初始化时一次性加载全部节点。
// 为避免超深目录导致栈溢出或耗时过长，限制递归深度。
func scanFolderTree(root string) ([]FileTreeNode, error) {
	return scanFolderTreeDepth(root, 0)
}

// maxTreeScanDepth 递归扫描的最大深度，超过后对应文件夹回退为懒加载。
const maxTreeScanDepth = 64

func scanFolderTreeDepth(root string, depth int) ([]FileTreeNode, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	nodes := make([]FileTreeNode, 0, len(entries))
	for _, entry := range entries {
		fullPath := filepath.Join(root, entry.Name())
		extension := strings.ToLower(filepath.Ext(entry.Name()))
		node := FileTreeNode{
			Name:      entry.Name(),
			Path:      fullPath,
			Extension: extension,
		}

		if entry.IsDir() {
			node.Type = "folder"
			if depth >= maxTreeScanDepth {
				// 超过深度限制，保持懒加载
				node.Loaded = false
				node.HasChild = folderHasChildren(fullPath)
			} else {
				children, err := scanFolderTreeDepth(fullPath, depth+1)
				if err != nil {
					// 无法访问的子目录（权限等）保持未加载，不中断整棵树
					node.Loaded = false
					node.HasChild = false
				} else {
					node.Children = children
					node.Loaded = true
					node.HasChild = len(children) > 0
				}
			}
		} else {
			node.Type = "file"
			node.Loaded = true
		}

		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "folder"
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})

	return nodes, nil
}

func detectTextEncoding(data []byte) string {
	if bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}) {
		return "utf-8"
	}
	if bytes.HasPrefix(data, []byte{0xFF, 0xFE}) {
		return "utf-16le"
	}
	if bytes.HasPrefix(data, []byte{0xFE, 0xFF}) {
		return "utf-16be"
	}
	if encoding := detectUTF16ByNullPattern(data); encoding != "" {
		return encoding
	}
	if utf8.Valid(data) {
		return "utf-8"
	}

	bestEncoding := "gbk"
	bestScore := -1 << 30
	for _, encoding := range []string{"gbk", "big5", "iso-8859-1"} {
		text, err := decodeText(data, encoding)
		if err != nil {
			continue
		}
		if score := scoreDecodedText(text); score > bestScore {
			bestEncoding = encoding
			bestScore = score
		}
	}
	return bestEncoding
}

func detectUTF16ByNullPattern(data []byte) string {
	if len(data) < 4 {
		return ""
	}

	limit := len(data)
	if limit > 1024 {
		limit = 1024
	}
	var evenNulls, oddNulls int
	for i := 0; i < limit; i++ {
		if data[i] != 0x00 {
			continue
		}
		if i%2 == 0 {
			evenNulls++
		} else {
			oddNulls++
		}
	}

	pairs := limit / 2
	if pairs == 0 {
		return ""
	}
	if oddNulls > pairs/4 && evenNulls < oddNulls/4 {
		return "utf-16le"
	}
	if evenNulls > pairs/4 && oddNulls < evenNulls/4 {
		return "utf-16be"
	}
	return ""
}

func scoreDecodedText(text string) int {
	score := 0
	for _, r := range text {
		switch {
		case r == utf8.RuneError:
			score -= 100
		case r == '\t' || r == '\n' || r == '\r':
			score += 2
		case unicode.IsControl(r):
			score -= 20
		case unicode.Is(unicode.Han, r):
			score += 8
		case unicode.IsPrint(r):
			score += 3
		default:
			score--
		}
	}
	return score
}

func normalizeTextEncoding(encoding string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "utf8", "utf-8":
		return "utf-8", nil
	case "gbk", "gb18030":
		return "gbk", nil
	case "gb2312", "gb-2312":
		return "gb2312", nil
	case "big5", "big-5":
		return "big5", nil
	case "utf-16le", "utf16le", "utf-16-le":
		return "utf-16le", nil
	case "utf-16be", "utf16be", "utf-16-be":
		return "utf-16be", nil
	case "iso-8859-1", "iso8859-1", "latin1", "latin-1":
		return "iso-8859-1", nil
	default:
		return "", fmt.Errorf("不支持的编码格式: %s", encoding)
	}
}

func encoderForTextEncoding(encoding string) textencoding.Encoding {
	switch encoding {
	case "gbk", "gb2312":
		return simplifiedchinese.GB18030
	case "big5":
		return traditionalchinese.Big5
	case "iso-8859-1":
		return charmap.ISO8859_1
	default:
		return nil
	}
}

func encodeText(content string, encoding string, originalData []byte) ([]byte, error) {
	encoding, err := normalizeTextEncoding(encoding)
	if err != nil {
		return nil, err
	}

	var transformer transform.Transformer
	var bom []byte

	switch encoding {
	case "utf-16le":
		transformer = textunicode.UTF16(textunicode.LittleEndian, textunicode.IgnoreBOM).NewEncoder()
		bom = []byte{0xFF, 0xFE}
	case "utf-16be":
		transformer = textunicode.UTF16(textunicode.BigEndian, textunicode.IgnoreBOM).NewEncoder()
		bom = []byte{0xFE, 0xFF}
	case "utf-8":
		encoded := []byte(content)
		if bytes.HasPrefix(originalData, []byte{0xEF, 0xBB, 0xBF}) {
			return append([]byte{0xEF, 0xBB, 0xBF}, encoded...), nil
		}
		return encoded, nil
	default:
		textEncoding := encoderForTextEncoding(encoding)
		if textEncoding == nil {
			return nil, fmt.Errorf("不支持的编码格式: %s", encoding)
		}
		transformer = textEncoding.NewEncoder()
	}

	encoded, _, err := transform.Bytes(transformer, []byte(content))
	if err != nil {
		return nil, err
	}
	if len(bom) > 0 {
		return append(bom, encoded...), nil
	}
	return encoded, nil
}

func decodeText(data []byte, encoding string) (string, error) {
	encoding, err := normalizeTextEncoding(encoding)
	if err != nil {
		return "", err
	}

	trimmed := data
	var decoder transform.Transformer

	switch encoding {
	case "utf-16le":
		if bytes.HasPrefix(trimmed, []byte{0xFF, 0xFE}) {
			trimmed = trimmed[2:]
		}
		decoder = textunicode.UTF16(textunicode.LittleEndian, textunicode.IgnoreBOM).NewDecoder()
	case "utf-16be":
		if bytes.HasPrefix(trimmed, []byte{0xFE, 0xFF}) {
			trimmed = trimmed[2:]
		}
		decoder = textunicode.UTF16(textunicode.BigEndian, textunicode.IgnoreBOM).NewDecoder()
	case "utf-8":
		if bytes.HasPrefix(trimmed, []byte{0xEF, 0xBB, 0xBF}) {
			trimmed = trimmed[3:]
		}
		return string(trimmed), nil
	default:
		textEncoding := encoderForTextEncoding(encoding)
		if textEncoding == nil {
			return "", fmt.Errorf("不支持的编码格式: %s", encoding)
		}
		decoder = textEncoding.NewDecoder()
	}

	decoded, _, err := transform.Bytes(decoder, trimmed)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// allowFile 将文件路径加入窗口的白名单
func allowFile(state *windowState, path string) {
	if state.allowedFiles == nil {
		state.allowedFiles = make(map[string]struct{})
	}
	state.allowedFiles[path] = struct{}{}
}

// isAllowedFile 检查文件路径是否在窗口的白名单中
func isAllowedFile(state *windowState, path string) bool {
	_, ok := state.allowedFiles[path]
	return ok
}

func isPathWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ============================================================================
// 主题管理方法
// ============================================================================

// themeBackgrounds 主题对应的窗口背景色 RGB
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

// BroadcastThemeChange 向所有窗口广播主题变更（多窗口同步备通道）
func (a *App) BroadcastThemeChange(theme string) error {
	app := application.Get()
	app.Event.Emit("theme-changed", theme)
	return nil
}

// ============================================================================
// 窗口管理方法
// ============================================================================

// NewWindow 创建一个新窗口，展示首页
func (a *App) NewWindow() {
	app := application.Get()
	// 新窗口默认使用明色背景；前端挂载后会通过 FOUC 脚本设置正确主题
	// 并通过 SetWindowBackground 校正背景色
	win := createMainWindow(app, "light")
	registerWindowHandlers(win, a)
	a.registerWindow(win.ID())
	win.Show()
}

func (a *App) registerWindow(windowID uint) {
	if windowID == 0 {
		return
	}
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	a.activeWindows[windowID] = struct{}{}
}

func (a *App) handleWindowClosing(windowID uint) error {
	if windowID == 0 {
		return nil
	}

	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()

	if _, ok := a.activeWindows[windowID]; !ok {
		delete(a.restoreSessions, windowID)
		delete(a.windowSessions, windowID)
		return writeAppSession(a.snapshotAppSessionLocked())
	}

	isLastWindow := len(a.activeWindows) <= 1
	if isLastWindow {
		lastSession, hasLastSession := a.windowSessions[windowID]
		for id := range a.restoreSessions {
			if id != windowID {
				delete(a.restoreSessions, id)
			}
		}
		for id := range a.windowSessions {
			if id != windowID {
				delete(a.windowSessions, id)
			}
		}
		if hasLastSession && sessionHasContent(lastSession) {
			a.windowSessions[windowID] = lastSession
		} else {
			delete(a.windowSessions, windowID)
		}
	} else {
		delete(a.restoreSessions, windowID)
		delete(a.windowSessions, windowID)
	}

	delete(a.activeWindows, windowID)
	return writeAppSession(a.snapshotAppSessionLocked())
}

func (a *App) loadStartupSession() []WindowSession {
	session, err := readAppSession()
	if err != nil {
		return nil
	}
	cleaned, changed := sanitizeAppSession(session)
	if changed {
		_ = writeAppSession(cleaned)
	}
	return cleaned.Windows
}

func (a *App) assignRestoreSession(windowID uint, session WindowSession) {
	if windowID == 0 {
		return
	}
	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	copied := normalizeWindowSession(session)
	restoreSession := copied
	restoreSession.HasSession = true
	a.restoreSessions[windowID] = &restoreSession
	a.windowSessions[windowID] = copied
}

func (a *App) ConsumeRestoreSession(ctx context.Context) (WindowSession, error) {
	id := windowIDFromCtx(ctx)
	if id == 0 {
		return WindowSession{}, errors.New("无法获取当前窗口")
	}

	a.sessionMu.Lock()
	session, ok := a.restoreSessions[id]
	if ok {
		delete(a.restoreSessions, id)
	}
	a.sessionMu.Unlock()

	if !ok || session == nil || !sessionHasContent(*session) {
		return WindowSession{}, nil
	}
	session.HasSession = true
	return *session, nil
}

func (a *App) SaveWindowSession(ctx context.Context, session WindowSession) error {
	id := windowIDFromCtx(ctx)
	if id == 0 {
		return errors.New("无法获取当前窗口")
	}

	cleaned := normalizeWindowSession(session)
	cleaned, _ = sanitizeWindowSession(cleaned)

	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	if sessionHasContent(cleaned) {
		a.windowSessions[id] = cleaned
	} else {
		delete(a.windowSessions, id)
	}
	appSession := a.snapshotAppSessionLocked()

	return writeAppSession(appSession)
}

func (a *App) ClearWindowSession(ctx context.Context) error {
	id := windowIDFromCtx(ctx)
	if id == 0 {
		return errors.New("无法获取当前窗口")
	}

	a.sessionMu.Lock()
	defer a.sessionMu.Unlock()
	delete(a.restoreSessions, id)
	delete(a.windowSessions, id)
	appSession := a.snapshotAppSessionLocked()

	return writeAppSession(appSession)
}

func readAppSession() (AppSession, error) {
	path, err := appSessionPath()
	if err != nil {
		return AppSession{Version: 1}, err
	}

	data, err := os.ReadFile(path)
	if isPathNotExist(err) {
		return AppSession{Version: 1}, nil
	}
	if err != nil {
		return AppSession{Version: 1}, err
	}

	var session AppSession
	if err := json.Unmarshal(data, &session); err != nil {
		return AppSession{Version: 1}, err
	}
	if session.Version == 0 {
		session.Version = 1
	}
	return session, nil
}

func writeAppSession(session AppSession) error {
	path, err := appSessionPath()
	if err != nil {
		return err
	}
	if len(session.Windows) == 0 {
		if err := os.Remove(path); err != nil && !isPathNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	session.Version = 1
	session.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0600)
}

func isPathNotExist(err error) bool {
	return err != nil && (errors.Is(err, os.ErrNotExist) || os.IsNotExist(err))
}

func appSessionPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "MostFileViewer", "session.json"), nil
}

func sanitizeAppSession(session AppSession) (AppSession, bool) {
	cleaned := AppSession{
		Version:   1,
		Windows:   make([]WindowSession, 0, len(session.Windows)),
		UpdatedAt: session.UpdatedAt,
	}
	changed := session.Version != 1

	for _, windowSession := range session.Windows {
		cleanWindow, windowChanged := sanitizeWindowSession(windowSession)
		if windowChanged {
			changed = true
		}
		if sessionHasContent(cleanWindow) {
			cleaned.Windows = append(cleaned.Windows, cleanWindow)
		} else {
			changed = true
		}
	}

	if len(cleaned.Windows) != len(session.Windows) {
		changed = true
	}
	return cleaned, changed
}

func sanitizeWindowSession(session WindowSession) (WindowSession, bool) {
	normalized := normalizeWindowSession(session)
	changed := !windowSessionsEqual(session, normalized)

	if normalized.Mode == "folder" {
		info, err := os.Stat(normalized.RootPath)
		if err != nil || !info.IsDir() {
			return WindowSession{}, true
		}
	} else if normalized.Mode == "file" {
		if normalized.RootPath != "" {
			if info, err := os.Stat(normalized.RootPath); err != nil || !info.IsDir() {
				normalized.RootPath = ""
				changed = true
			}
		}
	}

	tabs := make([]SessionTab, 0, len(normalized.OpenTabs))
	seen := make(map[string]struct{}, len(normalized.OpenTabs))
	activeExists := false
	for _, tab := range normalized.OpenTabs {
		cleanPath, err := filepath.Abs(tab.Path)
		if err != nil {
			changed = true
			continue
		}
		info, err := os.Stat(cleanPath)
		if err != nil || info.IsDir() {
			changed = true
			continue
		}
		key, err := canonicalizePath(cleanPath)
		if err != nil {
			changed = true
			continue
		}
		if _, ok := seen[key]; ok {
			changed = true
			continue
		}
		seen[key] = struct{}{}
		tabs = append(tabs, SessionTab{Path: cleanPath})
		if cleanPath == normalized.ActivePath {
			activeExists = true
		}
	}
	if len(tabs) != len(normalized.OpenTabs) {
		changed = true
	}
	normalized.OpenTabs = tabs

	if normalized.ActivePath != "" && !activeExists {
		if len(normalized.OpenTabs) > 0 {
			normalized.ActivePath = normalized.OpenTabs[0].Path
		} else {
			normalized.ActivePath = ""
		}
		changed = true
	}

	// 分屏布局以过滤后的 OpenTabs 为准：剔除失效 tab、回收空 pane、修正焦点。
	validPaths := make(map[string]struct{}, len(normalized.OpenTabs))
	for _, tab := range normalized.OpenTabs {
		validPaths[tab.Path] = struct{}{}
	}
	cleanLayout, layoutChanged := sanitizeSessionLayout(normalized.Layout, validPaths)
	if layoutChanged {
		changed = true
	}
	normalized.Layout = cleanLayout

	if normalized.Mode == "file" && len(normalized.OpenTabs) == 0 {
		return WindowSession{}, true
	}
	if normalized.Mode == "folder" && normalized.RootPath == "" && len(normalized.OpenTabs) == 0 {
		return WindowSession{}, true
	}
	return normalized, changed
}

const (
	sessionLayoutMaxPanes = 4
	sessionLayoutMaxDepth = 8
	sessionLayoutMinRatio = 0.15
	sessionLayoutMaxRatio = 0.85
)

// normalizeSessionLayoutPaths 对分屏布局中的路径做绝对化处理，与 OpenTabs 口径一致；
// 结构明显损坏（缺少 root / 子节点不足 / 层级过深）时返回 nil，前端会退化为单 pane。
func normalizeSessionLayoutPaths(layout *SessionPreviewLayout) *SessionPreviewLayout {
	if layout == nil {
		return nil
	}
	root := normalizeSessionSplitPaths(layout.Root, 0)
	if root == nil {
		return nil
	}
	return &SessionPreviewLayout{
		Root:          root,
		FocusedPaneID: strings.TrimSpace(layout.FocusedPaneID),
	}
}

func normalizeSessionSplitPaths(node *SessionSplitNode, depth int) *SessionSplitNode {
	if node == nil || depth > sessionLayoutMaxDepth {
		return nil
	}
	switch node.Type {
	case "split":
		if len(node.Children) < 2 {
			return nil
		}
		first := normalizeSessionSplitPaths(&node.Children[0], depth+1)
		second := normalizeSessionSplitPaths(&node.Children[1], depth+1)
		if first == nil {
			return second
		}
		if second == nil {
			return first
		}
		return &SessionSplitNode{
			Type:      "split",
			ID:        strings.TrimSpace(node.ID),
			Direction: node.Direction,
			Ratio:     node.Ratio,
			Children:  []SessionSplitNode{*first, *second},
		}
	case "pane":
		tabs := make([]string, 0, len(node.Tabs))
		for _, tab := range node.Tabs {
			path := strings.TrimSpace(tab)
			if path == "" {
				continue
			}
			if abs, err := filepath.Abs(path); err == nil {
				path = abs
			}
			tabs = append(tabs, path)
		}
		active := strings.TrimSpace(node.Active)
		if active != "" {
			if abs, err := filepath.Abs(active); err == nil {
				active = abs
			}
		}
		return &SessionSplitNode{
			Type:   "pane",
			ID:     strings.TrimSpace(node.ID),
			Tabs:   tabs,
			Active: active,
		}
	default:
		return nil
	}
}

// sanitizeSessionLayout 以 OpenTabs 为事实来源清理分屏布局：剔除已失效 / 重复的 tab、
// 回收空 pane、修正分割比例与焦点 pane。第二个返回值表示是否发生了修改。
func sanitizeSessionLayout(layout *SessionPreviewLayout, validPaths map[string]struct{}) (*SessionPreviewLayout, bool) {
	if layout == nil || layout.Root == nil {
		return nil, layout != nil
	}
	changed := false
	paneCount := 0

	var walk func(node *SessionSplitNode, depth int) *SessionSplitNode
	walk = func(node *SessionSplitNode, depth int) *SessionSplitNode {
		if node == nil || depth > sessionLayoutMaxDepth {
			changed = true
			return nil
		}
		if node.Type == "split" {
			if len(node.Children) < 2 {
				changed = true
				return nil
			}
			first := walk(&node.Children[0], depth+1)
			second := walk(&node.Children[1], depth+1)
			if first == nil {
				changed = true
				return second
			}
			if second == nil {
				changed = true
				return first
			}
			firstEmpty := first.Type == "pane" && len(first.Tabs) == 0
			secondEmpty := second.Type == "pane" && len(second.Tabs) == 0
			if firstEmpty && secondEmpty {
				changed = true
				return first
			}
			if firstEmpty {
				changed = true
				return second
			}
			if secondEmpty {
				changed = true
				return first
			}
			direction := node.Direction
			if direction != "column" {
				direction = "row"
			}
			if direction != node.Direction {
				changed = true
			}
			ratio := node.Ratio
			if ratio < sessionLayoutMinRatio || ratio > sessionLayoutMaxRatio {
				ratio = 0.5
				changed = true
			}
			return &SessionSplitNode{
				Type:      "split",
				ID:        node.ID,
				Direction: direction,
				Ratio:     ratio,
				Children:  []SessionSplitNode{*first, *second},
			}
		}
		if node.Type != "pane" {
			changed = true
			return nil
		}

		paneCount++
		if paneCount > sessionLayoutMaxPanes {
			changed = true
			return nil
		}

		tabs := make([]string, 0, len(node.Tabs))
		seen := make(map[string]struct{}, len(node.Tabs))
		for _, path := range node.Tabs {
			if _, ok := validPaths[path]; !ok {
				changed = true
				continue
			}
			if _, dup := seen[path]; dup {
				changed = true
				continue
			}
			seen[path] = struct{}{}
			tabs = append(tabs, path)
		}
		active := node.Active
		if active != "" {
			if _, ok := seen[active]; !ok {
				active = ""
				changed = true
			}
		}
		if active == "" && len(tabs) > 0 {
			active = tabs[len(tabs)-1]
		}
		return &SessionSplitNode{Type: "pane", ID: node.ID, Tabs: tabs, Active: active}
	}

	root := walk(layout.Root, 0)
	if root == nil {
		return nil, true
	}

	paneIDs := make(map[string]struct{})
	firstPaneID := ""
	var collect func(node *SessionSplitNode)
	collect = func(node *SessionSplitNode) {
		if node == nil {
			return
		}
		if node.Type == "pane" {
			if firstPaneID == "" {
				firstPaneID = node.ID
			}
			paneIDs[node.ID] = struct{}{}
			return
		}
		for i := range node.Children {
			collect(&node.Children[i])
		}
	}
	collect(root)

	focused := layout.FocusedPaneID
	if _, ok := paneIDs[focused]; !ok {
		focused = firstPaneID
		changed = true
	}

	return &SessionPreviewLayout{Root: root, FocusedPaneID: focused}, changed
}

func normalizeWindowSession(session WindowSession) WindowSession {
	normalized := session
	normalized.HasSession = false
	normalized.Mode = strings.ToLower(strings.TrimSpace(normalized.Mode))
	if normalized.Mode != "folder" && normalized.Mode != "file" {
		if strings.TrimSpace(normalized.RootPath) != "" {
			normalized.Mode = "folder"
		} else {
			normalized.Mode = "file"
		}
	}
	if normalized.LeftPaneWidth <= 0 {
		normalized.LeftPaneWidth = 320
	}
	if root := strings.TrimSpace(normalized.RootPath); root != "" {
		if cleanRoot, err := filepath.Abs(root); err == nil {
			normalized.RootPath = cleanRoot
		}
	}
	if active := strings.TrimSpace(normalized.ActivePath); active != "" {
		if cleanActive, err := filepath.Abs(active); err == nil {
			normalized.ActivePath = cleanActive
		}
	}

	tabs := make([]SessionTab, 0, len(normalized.OpenTabs))
	for _, tab := range normalized.OpenTabs {
		if path := strings.TrimSpace(tab.Path); path != "" {
			if cleanPath, err := filepath.Abs(path); err == nil {
				tabs = append(tabs, SessionTab{Path: cleanPath})
			}
		}
	}
	normalized.OpenTabs = tabs
	normalized.Layout = normalizeSessionLayoutPaths(normalized.Layout)
	return normalized
}

func sessionHasContent(session WindowSession) bool {
	return (session.Mode == "folder" && session.RootPath != "") || len(session.OpenTabs) > 0
}

func windowSessionsEqual(left, right WindowSession) bool {
	if left.Mode != right.Mode ||
		left.RootPath != right.RootPath ||
		left.ActivePath != right.ActivePath ||
		left.SidebarOpen != right.SidebarOpen ||
		left.LeftPaneWidth != right.LeftPaneWidth ||
		len(left.OpenTabs) != len(right.OpenTabs) {
		return false
	}
	for i := range left.OpenTabs {
		if left.OpenTabs[i].Path != right.OpenTabs[i].Path {
			return false
		}
	}
	return reflect.DeepEqual(left.Layout, right.Layout)
}

func (a *App) snapshotAppSessionLocked() AppSession {
	ids := make([]int, 0, len(a.windowSessions))
	for id := range a.windowSessions {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)

	windows := make([]WindowSession, 0, len(ids))
	for _, id := range ids {
		session := normalizeWindowSession(a.windowSessions[uint(id)])
		if sessionHasContent(session) {
			windows = append(windows, session)
		}
	}

	return AppSession{
		Version:   1,
		Windows:   windows,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
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
	if cleanPath, absErr := filepath.Abs(path); absErr == nil {
		if info, statErr := os.Stat(cleanPath); statErr == nil && !info.IsDir() {
			allowFile(state, cleanPath)
		}
	}
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
	var watcherToClose *fsnotify.Watcher
	if value, ok := a.states.Load(windowID); ok {
		state := value.(*windowState)
		state.mu.Lock()
		for path := range state.openPaths {
			if owner, ok := a.pathOwners.Load(path); ok && owner.(uint) == windowID {
				a.pathOwners.Delete(path)
			}
		}
		// Close watcher 前先把引用拿出来，再清空字段；Close 后消费 goroutine 会从 Events channel 关闭退出
		watcherToClose = state.watcher
		state.watcher = nil
		state.watchedDirs = nil
		state.mu.Unlock()
	}
	a.states.Delete(windowID)
	// Close 必须在 states.Delete 之后、goroutine 检查 state 之前；幂等，重复调用安全
	if watcherToClose != nil {
		_ = watcherToClose.Close()
	}
	// 收回该窗口名下的所有 media token，避免窗口关闭后旧 token 仍能访问文件
	if a.mediaServer != nil {
		a.mediaServer.RevokeWindow(windowID)
	}
}

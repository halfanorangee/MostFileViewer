package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

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
	currentRoot  string
	allowedFiles map[string]struct{}
	openPaths    map[string]struct{} // 当前窗口已打开的文件/文件夹路径集合
	mu           sync.Mutex
}

type App struct {
	states     sync.Map // windowID(uint) → *windowState
	pathOwners sync.Map // canonicalPath(string) → windowID(uint)
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

var revealPathInFileManager = openInFileManager

func NewApp() *App {
	return &App{}
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
		allowedFiles: make(map[string]struct{}),
		openPaths:    make(map[string]struct{}),
	})
	return actual.(*windowState)
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

	nodes, err := scanFolderLevel(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("扫描文件夹失败: %w", err)
	}

	state := a.getOrCreateState(ctx)
	state.mu.Lock()
	state.currentRoot = cleanRoot
	state.allowedFiles = make(map[string]struct{})
	state.mu.Unlock()
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
	return nodes, nil
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

	if isOfficePreviewExtension(extension) || isImagePreviewExtension(extension) || isPdfPreviewExtension(extension) {
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

	if err := writeFileAtomically(cleanPath, encodedContent, info.Mode().Perm()); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	return nil
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
	if runtime.GOOS == "windows" {
		if err := replaceFileOnWindows(path, tmpName); err != nil {
			return err
		}
	} else if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	removeTemp = false
	return nil
}

func replaceFileOnWindows(path string, tmpName string) error {
	backup, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".bak-*")
	if err != nil {
		return err
	}
	backupName := backup.Name()
	if err := backup.Close(); err != nil {
		_ = os.Remove(backupName)
		return err
	}
	if err := os.Remove(backupName); err != nil {
		return err
	}

	backupMoved := false
	defer func() {
		if backupMoved {
			_ = os.Remove(backupName)
		}
	}()

	if err := os.Rename(path, backupName); err != nil {
		return err
	}
	backupMoved = true

	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Rename(backupName, path)
		return err
	}

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

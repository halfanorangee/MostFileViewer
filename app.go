package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
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

type App struct {
	currentRoot  string
	allowedFiles map[string]struct{}
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
	return &App{
		allowedFiles: make(map[string]struct{}),
	}
}

func (a *App) SelectFile() (string, error) {
	selectedPath, err := application.Get().Dialog.OpenFile().
		CanChooseDirectories(false).
		CanChooseFiles(true).
		SetTitle("选择需要预览的文件").
		PromptForSingleSelection()
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

	a.allowFile(cleanPath)
	return cleanPath, nil
}

func (a *App) SelectFolder() (string, error) {
	selectedPath, err := application.Get().Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		SetTitle("选择需要预览的文件夹").
		PromptForSingleSelection()
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

func (a *App) LoadFolderTree(root string) ([]FileTreeNode, error) {
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

	a.currentRoot = cleanRoot
	a.resetAllowedFiles()
	return nodes, nil
}

func (a *App) LoadFolderChildren(path string) ([]FileTreeNode, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析文件夹路径失败: %w", err)
	}
	if strings.TrimSpace(a.currentRoot) == "" {
		a.currentRoot = cleanPath
	}
	if !isPathWithinRoot(a.currentRoot, cleanPath) {
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

func (a *App) ReadFile(path string) (*FileContent, error) {
	return a.readFile(path, "")
}

func (a *App) ReadFileWithEncoding(path string, encoding string) (*FileContent, error) {
	return a.readFile(path, encoding)
}

func (a *App) readFile(path string, requestedEncoding string) (*FileContent, error) {
	cleanPath, info, err := a.validateFilePath(path)
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

func (a *App) ReadFileChunk(path string, offset int64, size int) (*FileChunk, error) {
	cleanPath, info, err := a.validateFilePath(path)
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

func (a *App) SaveFile(path string, content string, encoding string) error {
	cleanPath, info, err := a.validateFilePath(path)
	if err != nil {
		return err
	}

	existingData, err := os.ReadFile(cleanPath)
	if err != nil {
		return fmt.Errorf("读取原文件失败: %w", err)
	}

	encoding, err = normalizeTextEncoding(encoding)
	if err != nil {
		return err
	}

	encodedContent, err := encodeText(content, encoding, existingData)
	if err != nil {
		return fmt.Errorf("编码文件内容失败: %w", err)
	}

	if err := writeFileAtomically(cleanPath, encodedContent, info.Mode().Perm()); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
	}

	return nil
}

func (a *App) ShowInFileManager(path string) error {
	cleanPath, info, err := a.validateFileManagerPath(path)
	if err != nil {
		return err
	}

	if err := revealPathInFileManager(cleanPath, info.IsDir()); err != nil {
		return fmt.Errorf("在文件管理器中显示失败: %w", err)
	}
	return nil
}

func (a *App) validateFilePath(path string) (string, os.FileInfo, error) {
	if strings.TrimSpace(path) == "" {
		return "", nil, errors.New("文件路径不能为空")
	}

	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, fmt.Errorf("解析文件路径失败: %w", err)
	}
	if strings.TrimSpace(a.currentRoot) == "" {
		a.currentRoot = filepath.Dir(cleanPath)
	}
	if !isPathWithinRoot(a.currentRoot, cleanPath) && !a.isAllowedFile(cleanPath) {
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

func (a *App) validateFileManagerPath(path string) (string, os.FileInfo, error) {
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

	if strings.TrimSpace(a.currentRoot) == "" {
		if info.IsDir() {
			a.currentRoot = cleanPath
		} else {
			a.currentRoot = filepath.Dir(cleanPath)
		}
	}
	if !isPathWithinRoot(a.currentRoot, cleanPath) && !a.isAllowedFile(cleanPath) {
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

func (a *App) allowFile(path string) {
	if a.allowedFiles == nil {
		a.allowedFiles = make(map[string]struct{})
	}
	a.allowedFiles[path] = struct{}{}
}

func (a *App) resetAllowedFiles() {
	if a.allowedFiles == nil {
		a.allowedFiles = make(map[string]struct{})
		return
	}
	for path := range a.allowedFiles {
		delete(a.allowedFiles, path)
	}
}

func (a *App) isAllowedFile(path string) bool {
	_, ok := a.allowedFiles[path]
	return ok
}

func isPathWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

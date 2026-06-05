package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/text/encoding/simplifiedchinese"
	textunicode "golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

type App struct {
	currentRoot string
}

const (
	maxTextPreviewBytes   int64 = 20 * 1024 * 1024
	maxOfficePreviewBytes int64 = 50 * 1024 * 1024
)

type FileTreeNode struct {
	Name      string         `json:"name"`
	Path      string         `json:"path"`
	Type      string         `json:"type"`
	Extension string         `json:"extension"`
	Children  []FileTreeNode `json:"children,omitempty"`
}

type FileContent struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Extension string `json:"extension"`
	Base64    string `json:"base64"`
	Encoding  string `json:"encoding"`
}

func NewApp() *App {
	return &App{}
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

	a.currentRoot = filepath.Dir(cleanPath)
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

	a.currentRoot = cleanPath
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

	nodes, err := scanFolder(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("扫描文件夹失败: %w", err)
	}

	a.currentRoot = cleanRoot
	return nodes, nil
}

func (a *App) ReadFile(path string) (*FileContent, error) {
	cleanPath, info, err := a.validateFilePath(path)
	if err != nil {
		return nil, err
	}

	limit := maxPreviewSize(info.Name())
	if info.Size() > limit {
		return nil, fmt.Errorf("文件过大，无法预览（当前 %.1f MB，限制 %.0f MB）", float64(info.Size())/1024/1024, float64(limit)/1024/1024)
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	return &FileContent{
		Name:      info.Name(),
		Path:      cleanPath,
		Extension: strings.ToLower(filepath.Ext(info.Name())),
		Base64:    base64.StdEncoding.EncodeToString(data),
		Encoding:  detectTextEncoding(data),
	}, nil
}

func (a *App) SaveFile(path string, content string) error {
	cleanPath, info, err := a.validateFilePath(path)
	if err != nil {
		return err
	}

	existingData, err := os.ReadFile(cleanPath)
	if err != nil {
		return fmt.Errorf("读取原文件失败: %w", err)
	}

	encodedContent, err := encodeText(content, detectTextEncoding(existingData), existingData)
	if err != nil {
		return fmt.Errorf("编码文件内容失败: %w", err)
	}

	if int64(len(encodedContent)) > maxTextPreviewBytes {
		return fmt.Errorf("文件过大，无法保存（当前 %.1f MB，限制 %.0f MB）", float64(len(encodedContent))/1024/1024, float64(maxTextPreviewBytes)/1024/1024)
	}

	if err := writeFileAtomically(cleanPath, encodedContent, info.Mode().Perm()); err != nil {
		return fmt.Errorf("保存文件失败: %w", err)
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
	if !isPathWithinRoot(a.currentRoot, cleanPath) {
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

func maxPreviewSize(name string) int64 {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".docx", ".xlsx", ".xls", ".xlsm", ".xltx", ".xltm":
		return maxOfficePreviewBytes
	default:
		return maxTextPreviewBytes
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
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}

	removeTemp = false
	return nil
}

func scanFolder(root string) ([]FileTreeNode, error) {
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
			children, childErr := scanFolder(fullPath)
			if childErr != nil {
				return nil, childErr
			}
			node.Children = children
		} else {
			node.Type = "file"
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
	if utf8.Valid(data) {
		return "utf-8"
	}
	return "gbk"
}

func encodeText(content string, encoding string, originalData []byte) ([]byte, error) {
	var transformer transform.Transformer
	var bom []byte

	switch strings.ToLower(encoding) {
	case "utf-16le":
		transformer = textunicode.UTF16(textunicode.LittleEndian, textunicode.IgnoreBOM).NewEncoder()
		if bytes.HasPrefix(originalData, []byte{0xFF, 0xFE}) {
			bom = []byte{0xFF, 0xFE}
		}
	case "utf-16be":
		transformer = textunicode.UTF16(textunicode.BigEndian, textunicode.IgnoreBOM).NewEncoder()
		if bytes.HasPrefix(originalData, []byte{0xFE, 0xFF}) {
			bom = []byte{0xFE, 0xFF}
		}
	case "gbk", "gb18030":
		transformer = simplifiedchinese.GB18030.NewEncoder()
	default:
		encoded := []byte(content)
		if bytes.HasPrefix(originalData, []byte{0xEF, 0xBB, 0xBF}) {
			return append([]byte{0xEF, 0xBB, 0xBF}, encoded...), nil
		}
		return encoded, nil
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

func isPathWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

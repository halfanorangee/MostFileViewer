package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// mediaServer 把磁盘上的媒体文件通过 token 暴露给 WebView。
// 视频/音频组件拿到 token URL 后，<video>/<audio> 会自动用 HTTP Range
// 拉取片段，无需把整文件读入 ArrayBuffer，从而把内存占用从「整文件」
// 降到「当前读取的几 MB 缓冲」。
type mediaServer struct {
	mu         sync.Mutex
	entries    map[string]mediaEntry
	baseURL    string
	httpServer *http.Server
	cancel     context.CancelFunc
}

type mediaEntry struct {
	path string
	size int64
	// modifiedUnix 记录注册时的文件修改时间（UnixNano）。源文件被同大小替换时，
	// serveMedia 依据它拒绝旧 token 继续响应。
	modifiedUnix int64
	windowID     uint
	expiresAt    time.Time
}

const (
	// mediaTokenTTL 单个 token 的有效期。覆盖一次完整视频播放所需的时长，
	// 同时也避免泄漏过老 token；窗口关闭时立即回收。
	mediaTokenTTL = 2 * time.Hour

	// mediaTokenCleanup 后台清理过期 token 的扫描间隔。
	mediaTokenCleanup = 5 * time.Minute

	// mediaPathPrefix 媒体路由前缀。匹配 /media/<token>。
	mediaPathPrefix = "/media/"
)

func newMediaServer() *mediaServer {
	return &mediaServer{entries: make(map[string]mediaEntry)}
}

// Start 在随机 loopback 端口启动真正的 HTTP 服务。媒体不能经过 Wails 的
// Asset Middleware：Windows 版 Wails 会先把整个响应聚合进 bytes.Buffer，
// 大视频会因此耗尽内存。标准 net/http 会直接把 Range 数据写入 socket。
func (s *mediaServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.httpServer != nil {
		return nil
	}

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("media: 启动本地监听失败: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}
	s.baseURL = "http://" + listener.Addr().String()
	s.httpServer = server
	s.cancel = cancel

	go s.runCleanup(ctx)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Printf("media: 本地 HTTP 服务异常退出: %v", err)
		}
	}()
	return nil
}

func (s *mediaServer) Close() error {
	s.mu.Lock()
	server := s.httpServer
	cancel := s.cancel
	s.httpServer = nil
	s.cancel = nil
	s.baseURL = ""
	s.entries = make(map[string]mediaEntry)
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if server == nil {
		return nil
	}
	ctx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(ctx); err != nil {
		_ = server.Close()
		return fmt.Errorf("media: 关闭本地服务失败: %w", err)
	}
	return nil
}

func (s *mediaServer) URL(token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.baseURL == "" {
		return "", errors.New("media: 本地服务未启动")
	}
	return s.baseURL + mediaPathPrefix + token, nil
}

func (s *mediaServer) runCleanup(ctx context.Context) {
	t := time.NewTicker(mediaTokenCleanup)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := time.Now()
			s.mu.Lock()
			for k, v := range s.entries {
				if v.expiresAt.Before(now) {
					delete(s.entries, k)
				}
			}
			s.mu.Unlock()
		}
	}
}

// Register 生成 128bit 随机 token 并绑定到指定路径。返回的 token 用作
// URL 的一部分。相同 path 在同一窗口内可重复注册拿到不同 token（不影响）。
func (s *mediaServer) Register(path string, size int64, modifiedUnix int64, windowID uint) (string, error) {
	if path == "" {
		return "", fmt.Errorf("media: path 不能为空")
	}
	if size <= 0 {
		return "", fmt.Errorf("media: size 必须 > 0")
	}

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("media: 生成 token 失败: %w", err)
	}
	token := hex.EncodeToString(buf)

	s.mu.Lock()
	s.entries[token] = mediaEntry{
		path:         path,
		size:         size,
		modifiedUnix: modifiedUnix,
		windowID:     windowID,
		expiresAt:    time.Now().Add(mediaTokenTTL),
	}
	s.mu.Unlock()
	return token, nil
}

// Revoke 回收一个 token：仅当 token 存在且属于 windowID 时删除。
// revoked 表示本次是否真正删除；foreign 表示 token 存在但属于其他窗口。
// token 不存在时两者均为 false，调用方据此实现幂等回收。
func (s *mediaServer) Revoke(token string, windowID uint) (revoked bool, foreign bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[token]
	if !ok {
		return false, false
	}
	if entry.windowID != windowID {
		return false, true
	}
	delete(s.entries, token)
	return true, false
}

// RevokeWindow 关闭/卸载窗口时回收该窗口名下的全部 token。
func (s *mediaServer) RevokeWindow(windowID uint) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.entries {
		if v.windowID == windowID {
			delete(s.entries, k)
		}
	}
}

// ServeHTTP 只服务带随机 token 的媒体路由。该 handler 运行在独立的
// 127.0.0.1 HTTP 服务上，不经过 Wails 自定义协议。
func (s *mediaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, mediaPathPrefix) {
		http.NotFound(w, r)
		return
	}
	s.serveMedia(w, r)
}

func (s *mediaServer) serveMedia(w http.ResponseWriter, r *http.Request) {
	// Wails 页面和随机 loopback 端口不是同源。媒体元素通常不要求 CORS，
	// 但显式允许跨源资源可避免 WebView2/PNA 策略升级后拦截本地媒体请求。
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Private-Network", "true")
	w.Header().Set("Cross-Origin-Resource-Policy", "cross-origin")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD, OPTIONS")
		http.Error(w, "media: 请求方法不受支持", http.StatusMethodNotAllowed)
		return
	}

	token := strings.TrimPrefix(r.URL.Path, mediaPathPrefix)
	if token == "" || strings.ContainsRune(token, '/') {
		http.Error(w, "media: 无效的 token", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	entry, ok := s.entries[token]
	if ok && time.Now().After(entry.expiresAt) {
		delete(s.entries, token)
		ok = false
	} else if ok {
		entry.expiresAt = time.Now().Add(mediaTokenTTL)
		s.entries[token] = entry
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "media: token 不存在或已过期", http.StatusForbidden)
		return
	}

	info, err := os.Stat(entry.path)
	if err != nil {
		http.Error(w, "media: 源文件不可访问", http.StatusNotFound)
		return
	}
	if info.IsDir() || info.Size() != entry.size || info.ModTime().UnixNano() != entry.modifiedUnix {
		// 源文件被替换（包括同大小替换）；拒绝响应避免把错误片段抛给播放器。
		// 前端资源错误无法读取响应头，此处同时给出机器可读标记便于测试与调试。
		w.Header().Set("X-Most-Media-Error", "source-changed")
		http.Error(w, "media: 源文件状态变更", http.StatusConflict)
		return
	}

	file, err := os.Open(entry.path)
	if err != nil {
		http.Error(w, "media: 打开文件失败", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(entry.path))
	contentType := mime.TypeByExtension(ext)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	// 强制 WebView 每次拉取都校验（避免复用旧缓存导致看不到外部对文件的修改）
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")

	total := info.Size()

	rangeHeader := r.Header.Get("Range")
	if rangeHeader == "" {
		// 无 Range：整文件流式响应。io.Copy 按 32KB 块读取，内存占用固定。
		w.Header().Set("Content-Length", strconv.FormatInt(total, 10))
		w.WriteHeader(http.StatusOK)
		if r.Method == http.MethodHead {
			return
		}
		if _, err := io.Copy(w, file); err != nil {
			log.Printf("media: 整文件流式写出失败 (path=%s): %v", entry.path, err)
		}
		return
	}

	start, end, err := parseRange(rangeHeader, total)
	if err != nil {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", total))
		http.Error(w, "media: Range 不可满足", http.StatusRequestedRangeNotSatisfiable)
		return
	}

	if _, err := file.Seek(start, io.SeekStart); err != nil {
		http.Error(w, "media: 定位失败", http.StatusInternalServerError)
		return
	}
	length := end - start + 1
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, total))
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.WriteHeader(http.StatusPartialContent)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.CopyN(w, file, length); err != nil {
		log.Printf("media: Range 写出失败 (path=%s, start=%d, length=%d): %v",
			entry.path, start, length, err)
	}
}

// parseRange 解析单段 Range 头，返回闭区间 [start, end]。支持 "bytes=N-" 和
// "bytes=N-M"。不处理多段（视频 <video> 也不会发出多段请求）。
func parseRange(header string, total int64) (int64, int64, error) {
	const prefix = "bytes="
	if !strings.HasPrefix(header, prefix) {
		return 0, 0, fmt.Errorf("media: Range 头格式错误")
	}
	spec := strings.TrimPrefix(header, prefix)
	if i := strings.IndexByte(spec, ','); i >= 0 {
		spec = spec[:i] // 多段仅取首段
	}
	dash := strings.IndexByte(spec, '-')
	if dash < 0 {
		return 0, 0, fmt.Errorf("media: Range 头格式错误")
	}
	startStr := strings.TrimSpace(spec[:dash])
	endStr := strings.TrimSpace(spec[dash+1:])

	var start, end int64
	var err error
	if startStr == "" {
		// 后缀范围：bytes=-N → 最后 N 字节
		suffix, e := strconv.ParseInt(endStr, 10, 64)
		if e != nil || suffix <= 0 {
			return 0, 0, fmt.Errorf("media: Range 后缀解析失败")
		}
		if suffix > total {
			suffix = total
		}
		start = total - suffix
		end = total - 1
	} else {
		start, err = strconv.ParseInt(startStr, 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("media: Range 起点解析失败")
		}
		if endStr == "" {
			end = total - 1
		} else {
			end, err = strconv.ParseInt(endStr, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("media: Range 终点解析失败")
			}
		}
	}
	if start < 0 || start >= total || end < start {
		return 0, 0, fmt.Errorf("media: Range 越界")
	}
	if end >= total {
		end = total - 1
	}
	return start, end, nil
}

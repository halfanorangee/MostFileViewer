package main

import (
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// dropItem 描述一个被拖入的文件/文件夹
type dropItem struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

func main() {
	appInstance := NewApp()

	app := application.New(application.Options{
		Name:        "MostFileViewer",
		Description: "文件夹预览器 - 浏览目录并预览 Word、Excel 文件",
		Services: []application.Service{
			application.NewService(appInstance),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false, // 多窗口时不应在关闭一个窗口后退出
		},
	})

	win := createMainWindow(app, "light")
	registerWindowHandlers(win, appInstance)

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}

// createMainWindow 创建主窗口（复用逻辑），backgroundTheme 指定初始背景色主题
// 由于后端无法读取前端 localStorage，新窗口初始以明色背景创建。
// 前端 index.html 内联脚本会在 webview 渲染前设置正确主题，
// SetWindowBackground 在前端初始化后调用以校正窗口原生背景色。
func createMainWindow(app *application.App, backgroundTheme string) *application.WebviewWindow {
	rgb := themeBackgrounds[backgroundTheme]
	if rgb == ([3]uint8{}) {
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

// registerWindowHandlers 注册窗口的事件处理器（文件拖放按窗口发送 + 窗口关闭清理状态）
func registerWindowHandlers(win *application.WebviewWindow, appInstance *App) {
	// 监听原生文件拖放事件，判断文件/文件夹后转发给前端（按窗口发送）
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
			win.EmitEvent("files-dropped", items) // 按窗口发送，仅当前窗口响应
		}
	})

	// 窗口关闭时清理状态，避免内存泄漏和去重误判
	win.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		appInstance.cleanupWindowState(win.ID())
	})
}

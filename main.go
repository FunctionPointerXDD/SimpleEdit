package main

import (
	"embed"
	"net/http"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// 최대화로 시작, 주소 칸 오른쪽 클릭 붙여넣기 허용
func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:            "운전 영상 편집기",
		Width:            1280,
		Height:           800,
		WindowStartState: options.Maximised,
		MinWidth:         1024,
		MinHeight:        600,
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: http.HandlerFunc(app.serveVideo),
		},
		BackgroundColour:         &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:                app.startup,
		OnBeforeClose:            app.beforeClose,
		OnShutdown:               app.shutdown,
		Bind:                     []interface{}{app},
		EnableDefaultContextMenu: true,
		Windows: &windows.Options{
			ZoomFactor:           1.0,
			IsZoomControlEnabled: false,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

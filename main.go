package main

import (
	"embed"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"log"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := &App{}
	if err := wails.Run(&options.App{
		Title: "青帆 · 素材生产", Width: 1200, Height: 800,
		AssetServer: &assetserver.Options{Assets: assets},
		OnStartup:   app.startup, OnShutdown: app.shutdown, Bind: []interface{}{app},
	}); err != nil {
		log.Fatal(err)
	}
}

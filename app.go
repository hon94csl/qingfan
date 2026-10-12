package main

import (
	"context"
	"github.com/hon94csl/qingfan/internal/media"
)

type App struct{ ctx context.Context }

func (a *App) startup(ctx context.Context)         { a.ctx = ctx }
func (a *App) CheckMediaTools() []media.ToolStatus { return media.CheckTools(a.ctx) }

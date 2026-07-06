package main

import (
	"context"
)

// App holds the application state and backend services.
type App struct {
	ctx context.Context
}

// newApp creates a new App instance.
func newApp() *App {
	return &App{}
}

// startup is called by Wails when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

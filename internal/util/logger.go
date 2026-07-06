package util

import (
	"log/slog"
	"os"
	"path/filepath"
)

// InitLogger configures slog to write JSON to ~/Library/Logs/AndroidFileTransfer/app.log.
// Falls back to stderr on any error.
func InitLogger() {
	logDir := filepath.Join(os.Getenv("HOME"), "Library", "Logs", "AndroidFileTransfer")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		return
	}
	f, err := os.OpenFile(
		filepath.Join(logDir, "app.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644,
	)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})))
}

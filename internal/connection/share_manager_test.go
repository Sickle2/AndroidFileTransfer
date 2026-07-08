package connection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"AndroidFileTransfer/internal/model"
)

func TestShareManagerDefaultConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")

	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	cfg := mgr.Config()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir() error = %v", err)
	}
	downloads := filepath.Join(homeDir, "Downloads")

	if cfg.Mode != model.ShareModeSelected {
		t.Errorf("Mode = %q, want %q", cfg.Mode, model.ShareModeSelected)
	}
	if cfg.RootDir != downloads {
		t.Errorf("RootDir = %q, want %q", cfg.RootDir, downloads)
	}
	if cfg.UploadDir != filepath.Join(downloads, "AndroidFileTransfer") {
		t.Errorf("UploadDir = %q, want %q", cfg.UploadDir, filepath.Join(downloads, "AndroidFileTransfer"))
	}
	if cfg.SharedItems == nil {
		t.Fatal("SharedItems is nil, want empty slice")
	}
	if len(cfg.SharedItems) != 0 {
		t.Errorf("len(SharedItems) = %d, want 0", len(cfg.SharedItems))
	}
	if _, err := os.Stat(configPath); err != nil {
		t.Errorf("default config was not saved: %v", err)
	}
}

func TestShareManagerSaveReload(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nested", "config.json")
	rootDir := filepath.Join(t.TempDir(), "root")
	uploadDir := filepath.Join(t.TempDir(), "upload")
	itemPath := filepath.Join(rootDir, "photo.jpg")

	mgr := &ShareManager{
		configPath: configPath,
		config: model.ShareConfig{
			Mode:      model.ShareModeDirectory,
			RootDir:   rootDir,
			UploadDir: uploadDir,
			SharedItems: []model.SharedItem{
				{ID: "item-1", Name: "photo.jpg", Path: itemPath, IsDir: false},
			},
		},
	}
	if err := mgr.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reloaded, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("reload error = %v", err)
	}
	cfg := reloaded.Config()
	if cfg.Mode != model.ShareModeDirectory {
		t.Errorf("Mode = %q, want %q", cfg.Mode, model.ShareModeDirectory)
	}
	if cfg.RootDir != rootDir {
		t.Errorf("RootDir = %q, want %q", cfg.RootDir, rootDir)
	}
	if cfg.UploadDir != uploadDir {
		t.Errorf("UploadDir = %q, want %q", cfg.UploadDir, uploadDir)
	}
	if len(cfg.SharedItems) != 1 {
		t.Fatalf("len(SharedItems) = %d, want 1", len(cfg.SharedItems))
	}
	if cfg.SharedItems[0].Path != "" {
		t.Errorf("SharedItems[0].Path = %q, want empty after JSON reload", cfg.SharedItems[0].Path)
	}
	if cfg.SharedItems[0].ID != "item-1" || cfg.SharedItems[0].Name != "photo.jpg" || cfg.SharedItems[0].IsDir {
		t.Errorf("SharedItems[0] = %+v, want saved public fields", cfg.SharedItems[0])
	}
}

func TestSharedItemPathNotExposedInJSON(t *testing.T) {
	item := model.SharedItem{
		ID:    "item-1",
		Name:  "secret.txt",
		Path:  "/Users/example/secret.txt",
		IsDir: false,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	jsonText := string(data)
	if strings.Contains(jsonText, "Path") || strings.Contains(jsonText, "path") {
		t.Fatalf("JSON exposes Path field: %s", jsonText)
	}
	if strings.Contains(jsonText, item.Path) || strings.Contains(jsonText, "/Users/example") {
		t.Fatalf("JSON exposes real path: %s", jsonText)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := decoded["path"]; ok {
		t.Fatalf("decoded JSON contains path key: %#v", decoded)
	}
}

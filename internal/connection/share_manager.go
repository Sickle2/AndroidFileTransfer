package connection

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"AndroidFileTransfer/internal/model"
)

const (
	shareConfigDirName  = "AndroidFileTransfer"
	shareConfigFileName = "config.json"
)

// ShareManager owns WiFi sharing configuration persistence.
// It is safe for concurrent use.
type ShareManager struct {
	mu         sync.RWMutex
	config     model.ShareConfig
	configPath string
}

// NewShareManager loads the WiFi sharing configuration from the default macOS
// Application Support location. If no configuration exists yet, it creates and
// saves the default configuration.
func NewShareManager() (*ShareManager, error) {
	configPath, err := defaultShareConfigPath()
	if err != nil {
		return nil, err
	}
	return newShareManagerWithConfigPath(configPath)
}

func newShareManagerWithConfigPath(configPath string) (*ShareManager, error) {
	mgr := &ShareManager{configPath: configPath}
	if err := mgr.load(); err != nil {
		return nil, err
	}
	return mgr, nil
}

// Config returns a copy of the current share configuration.
func (m *ShareManager) Config() model.ShareConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneShareConfig(m.config)
}

// Save persists the current share configuration to disk.
func (m *ShareManager) Save() error {
	m.mu.RLock()
	cfg := cloneShareConfig(m.config)
	configPath := m.configPath
	m.mu.RUnlock()

	return saveShareConfig(configPath, cfg)
}

func (m *ShareManager) load() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg, err := defaultShareConfig()
			if err != nil {
				return err
			}
			m.mu.Lock()
			m.config = cfg
			m.mu.Unlock()
			return m.Save()
		}
		return fmt.Errorf("读取共享配置失败: %w", err)
	}

	var cfg model.ShareConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析共享配置失败: %w", err)
	}
	m.mu.Lock()
	m.config = normalizeShareConfig(cfg)
	m.mu.Unlock()
	return nil
}

func saveShareConfig(configPath string, cfg model.ShareConfig) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return fmt.Errorf("创建共享配置目录失败: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化共享配置失败: %w", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		return fmt.Errorf("保存共享配置失败: %w", err)
	}
	return nil
}

func defaultShareConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败: %w", err)
	}
	return filepath.Join(homeDir, "Library", "Application Support", shareConfigDirName, shareConfigFileName), nil
}

func defaultShareConfig() (model.ShareConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return model.ShareConfig{}, fmt.Errorf("获取用户主目录失败: %w", err)
	}
	downloads := filepath.Join(homeDir, "Downloads")
	return model.ShareConfig{
		Mode:        model.ShareModeSelected,
		RootDir:     downloads,
		UploadDir:   filepath.Join(downloads, "AndroidFileTransfer"),
		SharedItems: []model.SharedItem{},
	}, nil
}

func normalizeShareConfig(cfg model.ShareConfig) model.ShareConfig {
	defaults, err := defaultShareConfig()
	if err != nil {
		defaults = model.ShareConfig{
			Mode:        model.ShareModeSelected,
			SharedItems: []model.SharedItem{},
		}
	}
	if cfg.Mode == "" {
		cfg.Mode = defaults.Mode
	}
	if cfg.RootDir == "" {
		cfg.RootDir = defaults.RootDir
	}
	if cfg.UploadDir == "" {
		cfg.UploadDir = defaults.UploadDir
	}
	if cfg.SharedItems == nil {
		cfg.SharedItems = []model.SharedItem{}
	}
	return cfg
}

func cloneShareConfig(cfg model.ShareConfig) model.ShareConfig {
	clone := cfg
	if cfg.SharedItems == nil {
		clone.SharedItems = []model.SharedItem{}
		return clone
	}
	clone.SharedItems = make([]model.SharedItem, len(cfg.SharedItems))
	copy(clone.SharedItems, cfg.SharedItems)
	return clone
}

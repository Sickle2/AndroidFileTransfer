package connection

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"AndroidFileTransfer/internal/model"
)

const (
	shareConfigDirName  = "AndroidFileTransfer"
	shareConfigFileName = "config.json"
)

// ShareManager owns WiFi sharing configuration persistence and virtual-path
// resolution. It is safe for concurrent use.
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

// UploadDir returns the configured upload destination directory.
func (m *ShareManager) UploadDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.UploadDir
}

// Save persists the current share configuration to disk.
func (m *ShareManager) Save() error {
	m.mu.RLock()
	cfg := cloneShareConfig(m.config)
	configPath := m.configPath
	m.mu.RUnlock()

	return saveShareConfig(configPath, cfg)
}

// SetMode updates the sharing mode and saves the configuration.
func (m *ShareManager) SetMode(mode model.ShareMode) error {
	if mode != model.ShareModeSelected && mode != model.ShareModeDirectory {
		return fmt.Errorf("未知共享模式: %q", mode)
	}
	m.mu.Lock()
	m.config.Mode = mode
	m.mu.Unlock()
	return m.Save()
}

// SetRootDir updates the root directory (used in directory mode) and saves the
// configuration. The directory itself must not be a hidden name.
func (m *ShareManager) SetRootDir(dir string) error {
	abs, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return fmt.Errorf("无效路径: %w", err)
	}
	if isHiddenName(filepath.Base(abs)) {
		return fmt.Errorf("不允许设置隐藏目录为根目录: %q", dir)
	}
	m.mu.Lock()
	m.config.RootDir = abs
	m.mu.Unlock()
	return m.Save()
}

// SetUploadDir updates the upload destination directory and saves the
// configuration.
func (m *ShareManager) SetUploadDir(dir string) error {
	abs, err := filepath.Abs(filepath.Clean(dir))
	if err != nil {
		return fmt.Errorf("无效路径: %w", err)
	}
	m.mu.Lock()
	m.config.UploadDir = abs
	m.mu.Unlock()
	return m.Save()
}

// AddSharedPaths adds one or more real Mac paths to the shared-items list.
// Hidden files or folders (name starts with '.') are rejected.
// Duplicate real paths are silently skipped.
// The configuration is saved automatically on success.
func (m *ShareManager) AddSharedPaths(paths []string) error {
	now := time.Now().UnixNano()

	m.mu.Lock()
	added := 0
	for i, p := range paths {
		abs, err := filepath.Abs(filepath.Clean(p))
		if err != nil {
			m.mu.Unlock()
			return fmt.Errorf("无效路径 %q: %w", p, err)
		}
		if isHiddenName(filepath.Base(abs)) {
			m.mu.Unlock()
			return fmt.Errorf("不允许共享隐藏文件或文件夹: %q", filepath.Base(abs))
		}
		// Skip duplicates by real path.
		dup := false
		for _, existing := range m.config.SharedItems {
			if existing.Path == abs {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil {
			m.mu.Unlock()
			return fmt.Errorf("无法访问路径 %q: %w", p, err)
		}
		id := fmt.Sprintf("item-%d-%d", now, i)
		m.config.SharedItems = append(m.config.SharedItems, model.SharedItem{
			ID:    id,
			Name:  filepath.Base(abs),
			Path:  abs,
			IsDir: info.IsDir(),
		})
		added++
	}
	m.mu.Unlock()

	if added == 0 {
		return nil // all duplicates — nothing to persist
	}
	return m.Save()
}

// RemoveSharedItem removes the shared item with the given ID and saves the
// configuration. Returns an error if the ID is not found.
func (m *ShareManager) RemoveSharedItem(id string) error {
	m.mu.Lock()
	items := m.config.SharedItems
	found := false
	newItems := make([]model.SharedItem, 0, len(items))
	for _, item := range items {
		if item.ID == id {
			found = true
			continue
		}
		newItems = append(newItems, item)
	}
	if !found {
		m.mu.Unlock()
		return fmt.Errorf("共享项不存在: %q", id)
	}
	m.config.SharedItems = newItems
	m.mu.Unlock()
	return m.Save()
}

// ClearSharedItems removes all shared items and saves the configuration.
func (m *ShareManager) ClearSharedItems() error {
	m.mu.Lock()
	m.config.SharedItems = []model.SharedItem{}
	m.mu.Unlock()
	return m.Save()
}

// errVirtualDir is returned by ResolveVirtualPath when the path identifies a
// virtual directory root. Callers should use ListVirtualDir instead.
var errVirtualDir = errors.New("虚拟根目录：请使用 ListVirtualDir 列出目录")

// ResolveVirtualPath maps a virtual path (as seen by the Android browser) to a
// real Mac file path and returns its os.FileInfo.
//
// selected mode:
//
//	"/" or ""          → errVirtualDir
//	"/shared/<id>"     → shared item root (file or directory)
//	"/shared/<id>/…"   → subpath inside a directory shared item
//
// directory mode:
//
//	"/" or "/browse"   → errVirtualDir
//	"/browse/<rel>"    → filepath.Join(rootDir, rel)
//
// Hidden path segments and directory-traversal attempts are rejected.
func (m *ShareManager) ResolveVirtualPath(vpath string) (string, os.FileInfo, error) {
	m.mu.RLock()
	cfg := cloneShareConfig(m.config)
	m.mu.RUnlock()

	vpath = cleanVPath(vpath)
	if err := rejectHiddenVPath(vpath); err != nil {
		return "", nil, err
	}

	switch cfg.Mode {
	case model.ShareModeSelected:
		return resolveSelectedPath(vpath, cfg.SharedItems)
	case model.ShareModeDirectory:
		return resolveDirectoryPath(vpath, cfg.RootDir)
	default:
		return "", nil, fmt.Errorf("未知共享模式: %q", cfg.Mode)
	}
}

// ListVirtualDir returns virtual directory entries for the given virtual path.
// Every FileInfo.Path in the result is a virtual path; no real Mac paths are
// exposed. Hidden entries (name starts with '.') are silently filtered out.
func (m *ShareManager) ListVirtualDir(vpath string) ([]model.FileInfo, error) {
	m.mu.RLock()
	cfg := cloneShareConfig(m.config)
	m.mu.RUnlock()

	vpath = cleanVPath(vpath)
	if err := rejectHiddenVPath(vpath); err != nil {
		return nil, err
	}

	switch cfg.Mode {
	case model.ShareModeSelected:
		return listSelectedDir(vpath, cfg.SharedItems)
	case model.ShareModeDirectory:
		return listDirectoryDir(vpath, cfg.RootDir)
	default:
		return nil, fmt.Errorf("未知共享模式: %q", cfg.Mode)
	}
}

// ValidateUploadName checks that name is safe to use as an upload filename.
// Rejects: empty names, names containing '/' or '\', names starting with '.',
// and names that change under filepath.Clean (e.g. "a/../b").
func ValidateUploadName(name string) error {
	if name == "" {
		return fmt.Errorf("上传文件名不能为空")
	}
	if strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("上传文件名不能包含路径分隔符: %q", name)
	}
	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("不允许上传隐藏文件: %q", name)
	}
	if filepath.Clean(name) != name {
		return fmt.Errorf("上传文件名不合法: %q", name)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal path helpers
// ---------------------------------------------------------------------------

// isHiddenName reports whether a single filename segment starts with a dot.
func isHiddenName(name string) bool {
	return strings.HasPrefix(name, ".")
}

// cleanVPath normalises a virtual path: ensures a leading '/', removes
// redundant slashes/dots, and uses forward slashes only.
func cleanVPath(vpath string) string {
	v := "/" + strings.TrimPrefix(vpath, "/")
	// filepath.Clean handles ".." etc.; replace OS separator with "/" afterwards.
	v = strings.ReplaceAll(filepath.Clean(v), string(os.PathSeparator), "/")
	if v == "" {
		return "/"
	}
	return v
}

// rejectHiddenVPath returns an error if any segment of the virtual path starts
// with a dot.
func rejectHiddenVPath(vpath string) error {
	for _, seg := range strings.Split(strings.TrimPrefix(vpath, "/"), "/") {
		if seg != "" && strings.HasPrefix(seg, ".") {
			return fmt.Errorf("禁止访问隐藏路径: %q", seg)
		}
	}
	return nil
}

// safejoin joins base with a (possibly slash-separated) relative path and
// verifies the result stays inside base, preventing directory traversal.
// filepath.Join cleans the result, so ".." components are collapsed before
// the bounds check.
func safejoin(base, rel string) (string, error) {
	candidate := filepath.Join(base, rel)
	if candidate != base && !strings.HasPrefix(candidate, base+string(os.PathSeparator)) {
		return "", fmt.Errorf("禁止访问根目录外的路径")
	}
	return candidate, nil
}

// resolveSelectedPath resolves a selected-mode virtual path to a real path.
func resolveSelectedPath(vpath string, items []model.SharedItem) (string, os.FileInfo, error) {
	if vpath == "/" {
		return "", nil, errVirtualDir
	}
	if !strings.HasPrefix(vpath, "/shared/") {
		return "", nil, fmt.Errorf("selected 模式下虚拟路径必须以 /shared/ 开头，got %q", vpath)
	}

	rest := strings.TrimPrefix(vpath, "/shared/")
	id, subRel, hasSub := strings.Cut(rest, "/")

	item := findItem(items, id)
	if item == nil {
		return "", nil, fmt.Errorf("共享项不存在: %q", id)
	}

	if !hasSub || subRel == "" {
		info, err := os.Stat(item.Path)
		if err != nil {
			return "", nil, fmt.Errorf("访问共享项失败: %w", err)
		}
		return item.Path, info, nil
	}

	if !item.IsDir {
		return "", nil, fmt.Errorf("共享项 %q 不是目录，无法访问子路径", item.Name)
	}
	resolved, err := safejoin(item.Path, subRel)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("访问共享子路径失败: %w", err)
	}
	return resolved, info, nil
}

// resolveDirectoryPath resolves a directory-mode virtual path to a real path.
func resolveDirectoryPath(vpath string, rootDir string) (string, os.FileInfo, error) {
	if vpath == "/" || vpath == "/browse" {
		return "", nil, errVirtualDir
	}
	if !strings.HasPrefix(vpath, "/browse/") {
		return "", nil, fmt.Errorf("directory 模式下虚拟路径必须以 /browse/ 开头，got %q", vpath)
	}
	rel := strings.TrimPrefix(vpath, "/browse/")
	resolved, err := safejoin(rootDir, rel)
	if err != nil {
		return "", nil, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", nil, fmt.Errorf("访问路径失败: %w", err)
	}
	return resolved, info, nil
}

// listSelectedDir returns virtual file entries for selected mode.
func listSelectedDir(vpath string, items []model.SharedItem) ([]model.FileInfo, error) {
	if vpath == "/" {
		result := make([]model.FileInfo, 0, len(items))
		for _, item := range items {
			if isHiddenName(item.Name) {
				continue
			}
			result = append(result, model.FileInfo{
				Name:  item.Name,
				Path:  "/shared/" + item.ID,
				IsDir: item.IsDir,
			})
		}
		return result, nil
	}

	if !strings.HasPrefix(vpath, "/shared/") {
		return nil, fmt.Errorf("selected 模式下虚拟路径必须以 /shared/ 开头，got %q", vpath)
	}

	rest := strings.TrimPrefix(vpath, "/shared/")
	id, subRel, hasSub := strings.Cut(rest, "/")

	item := findItem(items, id)
	if item == nil {
		return nil, fmt.Errorf("共享项不存在: %q", id)
	}
	if !item.IsDir {
		return nil, fmt.Errorf("共享项 %q 不是目录", item.Name)
	}

	realDir := item.Path
	if hasSub && subRel != "" {
		var err error
		realDir, err = safejoin(item.Path, subRel)
		if err != nil {
			return nil, err
		}
	}

	entries, err := os.ReadDir(realDir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	result := make([]model.FileInfo, 0, len(entries))
	for _, e := range entries {
		if isHiddenName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		var entryVPath string
		if hasSub && subRel != "" {
			entryVPath = "/shared/" + id + "/" + subRel + "/" + e.Name()
		} else {
			entryVPath = "/shared/" + id + "/" + e.Name()
		}
		result = append(result, model.FileInfo{
			Name:    e.Name(),
			Path:    entryVPath,
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			ModTime: info.ModTime(),
		})
	}
	return result, nil
}

// listDirectoryDir returns virtual file entries for directory mode.
func listDirectoryDir(vpath string, rootDir string) ([]model.FileInfo, error) {
	var realDir string
	var vpathPrefix string

	switch {
	case vpath == "/" || vpath == "/browse":
		realDir = rootDir
		vpathPrefix = "/browse/"
	case strings.HasPrefix(vpath, "/browse/"):
		rel := strings.TrimPrefix(vpath, "/browse/")
		var err error
		realDir, err = safejoin(rootDir, rel)
		if err != nil {
			return nil, err
		}
		vpathPrefix = vpath + "/"
	default:
		return nil, fmt.Errorf("directory 模式下虚拟路径必须以 /browse/ 开头，got %q", vpath)
	}

	entries, err := os.ReadDir(realDir)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	result := make([]model.FileInfo, 0, len(entries))
	for _, e := range entries {
		if isHiddenName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, model.FileInfo{
			Name:    e.Name(),
			Path:    vpathPrefix + e.Name(),
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			ModTime: info.ModTime(),
		})
	}
	return result, nil
}

// findItem returns a pointer to the SharedItem with the given ID, or nil.
func findItem(items []model.SharedItem, id string) *model.SharedItem {
	for i := range items {
		if items[i].ID == id {
			return &items[i]
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Persistence helpers
// ---------------------------------------------------------------------------

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

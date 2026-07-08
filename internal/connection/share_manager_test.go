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

// ---------------------------------------------------------------------------
// Task 2 tests: virtual path resolution and hidden file security
// ---------------------------------------------------------------------------

func TestIsHiddenName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{".env", true},
		{".git", true},
		{".DS_Store", true},
		{"file.txt", false},
		{"normal", false},
		{"dir", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isHiddenName(tt.name)
		if got != tt.want {
			t.Errorf("isHiddenName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestAddSharedPathsRejectsHidden(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	tmpDir := t.TempDir()
	hidden := filepath.Join(tmpDir, ".hidden.txt")
	if err := os.WriteFile(hidden, []byte("secret"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	err = mgr.AddSharedPaths([]string{hidden})
	if err == nil {
		t.Fatal("AddSharedPaths(.hidden.txt) succeeded, want error")
	}
	if !strings.Contains(err.Error(), "隐藏") {
		t.Errorf("error = %v, want mention of hidden", err)
	}
}

func TestAddSharedPathsDeduplicates(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := mgr.AddSharedPaths([]string{file}); err != nil {
		t.Fatalf("AddSharedPaths() error = %v", err)
	}
	if err := mgr.AddSharedPaths([]string{file}); err != nil {
		t.Fatalf("AddSharedPaths() second time error = %v", err)
	}

	cfg := mgr.Config()
	if len(cfg.SharedItems) != 1 {
		t.Errorf("len(SharedItems) = %d, want 1 (deduplication)", len(cfg.SharedItems))
	}
}

func TestRemoveSharedItem(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file.txt")
	if err := os.WriteFile(file, []byte("data"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := mgr.AddSharedPaths([]string{file}); err != nil {
		t.Fatalf("AddSharedPaths() error = %v", err)
	}

	cfg := mgr.Config()
	if len(cfg.SharedItems) != 1 {
		t.Fatalf("len(SharedItems) = %d, want 1", len(cfg.SharedItems))
	}
	id := cfg.SharedItems[0].ID

	if err := mgr.RemoveSharedItem(id); err != nil {
		t.Fatalf("RemoveSharedItem(%q) error = %v", id, err)
	}

	cfg = mgr.Config()
	if len(cfg.SharedItems) != 0 {
		t.Errorf("len(SharedItems) = %d, want 0 after remove", len(cfg.SharedItems))
	}

	// Not found case.
	err = mgr.RemoveSharedItem("nonexistent")
	if err == nil {
		t.Fatal("RemoveSharedItem(nonexistent) succeeded, want error")
	}
	if !strings.Contains(err.Error(), "不存在") {
		t.Errorf("error = %v, want mention of not found", err)
	}
}

func TestClearSharedItems(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, []byte("1"), 0o600)
	os.WriteFile(file2, []byte("2"), 0o600)

	if err := mgr.AddSharedPaths([]string{file1, file2}); err != nil {
		t.Fatalf("AddSharedPaths() error = %v", err)
	}
	if len(mgr.Config().SharedItems) != 2 {
		t.Fatalf("len(SharedItems) = %d, want 2", len(mgr.Config().SharedItems))
	}

	if err := mgr.ClearSharedItems(); err != nil {
		t.Fatalf("ClearSharedItems() error = %v", err)
	}
	if len(mgr.Config().SharedItems) != 0 {
		t.Errorf("len(SharedItems) = %d, want 0 after clear", len(mgr.Config().SharedItems))
	}
}

func TestResolveVirtualPathSelectedMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file.txt")
	dir := filepath.Join(tmpDir, "folder")
	subfile := filepath.Join(dir, "sub.txt")
	os.WriteFile(file, []byte("data"), 0o600)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(subfile, []byte("sub"), 0o600)

	mgr.AddSharedPaths([]string{file, dir})
	cfg := mgr.Config()
	if len(cfg.SharedItems) != 2 {
		t.Fatalf("len(SharedItems) = %d, want 2", len(cfg.SharedItems))
	}

	var fileItem, dirItem model.SharedItem
	for _, item := range cfg.SharedItems {
		if item.IsDir {
			dirItem = item
		} else {
			fileItem = item
		}
	}

	// Root returns errVirtualDir.
	_, _, err = mgr.ResolveVirtualPath("/")
	if err != errVirtualDir {
		t.Errorf("ResolveVirtualPath(\"/\") error = %v, want errVirtualDir", err)
	}

	// Shared file.
	realPath, info, err := mgr.ResolveVirtualPath("/shared/" + fileItem.ID)
	if err != nil {
		t.Fatalf("ResolveVirtualPath(/shared/%s) error = %v", fileItem.ID, err)
	}
	if realPath != file {
		t.Errorf("realPath = %q, want %q", realPath, file)
	}
	if info.IsDir() {
		t.Errorf("info.IsDir() = true, want false")
	}

	// Shared directory.
	realPath, info, err = mgr.ResolveVirtualPath("/shared/" + dirItem.ID)
	if err != nil {
		t.Fatalf("ResolveVirtualPath(/shared/%s) error = %v", dirItem.ID, err)
	}
	if realPath != dir {
		t.Errorf("realPath = %q, want %q", realPath, dir)
	}
	if !info.IsDir() {
		t.Errorf("info.IsDir() = false, want true")
	}

	// Subfile within shared directory.
	realPath, info, err = mgr.ResolveVirtualPath("/shared/" + dirItem.ID + "/sub.txt")
	if err != nil {
		t.Fatalf("ResolveVirtualPath(/shared/%s/sub.txt) error = %v", dirItem.ID, err)
	}
	if realPath != subfile {
		t.Errorf("realPath = %q, want %q", realPath, subfile)
	}
	if info.IsDir() {
		t.Errorf("info.IsDir() = true, want false")
	}

	// Nonexistent item ID.
	_, _, err = mgr.ResolveVirtualPath("/shared/nonexistent")
	if err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Errorf("ResolveVirtualPath(/shared/nonexistent) error = %v, want not found", err)
	}

	// Subpath on a file item (not a directory).
	_, _, err = mgr.ResolveVirtualPath("/shared/" + fileItem.ID + "/extra")
	if err == nil || !strings.Contains(err.Error(), "不是目录") {
		t.Errorf("ResolveVirtualPath(/shared/<file>/extra) error = %v, want not a directory", err)
	}
}

func TestResolveVirtualPathDirectoryMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	rootDir := t.TempDir()
	file := filepath.Join(rootDir, "file.txt")
	subdir := filepath.Join(rootDir, "subdir")
	subfile := filepath.Join(subdir, "sub.txt")
	os.WriteFile(file, []byte("data"), 0o600)
	os.MkdirAll(subdir, 0o755)
	os.WriteFile(subfile, []byte("sub"), 0o600)

	mgr.SetMode(model.ShareModeDirectory)
	mgr.SetRootDir(rootDir)

	// Root returns errVirtualDir.
	_, _, err = mgr.ResolveVirtualPath("/")
	if err != errVirtualDir {
		t.Errorf("ResolveVirtualPath(\"/\") error = %v, want errVirtualDir", err)
	}
	_, _, err = mgr.ResolveVirtualPath("/browse")
	if err != errVirtualDir {
		t.Errorf("ResolveVirtualPath(\"/browse\") error = %v, want errVirtualDir", err)
	}

	// Top-level file.
	realPath, info, err := mgr.ResolveVirtualPath("/browse/file.txt")
	if err != nil {
		t.Fatalf("ResolveVirtualPath(/browse/file.txt) error = %v", err)
	}
	if realPath != file {
		t.Errorf("realPath = %q, want %q", realPath, file)
	}
	if info.IsDir() {
		t.Errorf("info.IsDir() = true, want false")
	}

	// Subdir.
	realPath, _, err = mgr.ResolveVirtualPath("/browse/subdir")
	if err != nil {
		t.Fatalf("ResolveVirtualPath(/browse/subdir) error = %v", err)
	}
	if realPath != subdir {
		t.Errorf("realPath = %q, want %q", realPath, subdir)
	}

	// File in subdir.
	realPath, _, err = mgr.ResolveVirtualPath("/browse/subdir/sub.txt")
	if err != nil {
		t.Fatalf("ResolveVirtualPath(/browse/subdir/sub.txt) error = %v", err)
	}
	if realPath != subfile {
		t.Errorf("realPath = %q, want %q", realPath, subfile)
	}
}

func TestResolveVirtualPathRejectsHidden(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	rootDir := t.TempDir()
	hidden := filepath.Join(rootDir, ".hidden")
	os.WriteFile(hidden, []byte("secret"), 0o600)

	mgr.SetMode(model.ShareModeDirectory)
	mgr.SetRootDir(rootDir)

	_, _, err = mgr.ResolveVirtualPath("/browse/.hidden")
	if err == nil {
		t.Fatal("ResolveVirtualPath(/browse/.hidden) succeeded, want error")
	}
	if !strings.Contains(err.Error(), "隐藏") {
		t.Errorf("error = %v, want mention of hidden", err)
	}
}

func TestResolveVirtualPathRejectsTraversal(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	rootDir := t.TempDir()
	mgr.SetMode(model.ShareModeDirectory)
	mgr.SetRootDir(rootDir)

	// cleanVPath collapses ".." early, so deep traversal attempts end up with
	// a path outside the /browse/ prefix and are rejected at the prefix check.
	// Either rejection reason is correct from a security standpoint.
	traversalPaths := []string{
		"/browse/../../../etc/passwd",
		"/browse/sub/../../..",
	}
	for _, vpath := range traversalPaths {
		_, _, err = mgr.ResolveVirtualPath(vpath)
		if err == nil {
			t.Errorf("ResolveVirtualPath(%q) succeeded, want error", vpath)
		}
	}

	// safejoin is the backstop for relative escapes within a real subpath.
	// cleanVPath normalises ".."-heavy virtual paths early, but safejoin must
	// still block raw relative traversal (e.g. from internal callers or if a
	// future code path bypasses cleanVPath).
	outer := t.TempDir()
	_, safeErr := safejoin(rootDir, "../../"+filepath.Base(outer))
	if safeErr == nil {
		t.Error("safejoin(../../escape) succeeded, want error")
	} else if !strings.Contains(safeErr.Error(), "根目录外") {
		t.Errorf("safejoin error = %v, want mention of 根目录外", safeErr)
	}
}

func TestListVirtualDirSelectedMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "visible.txt")
	dir := filepath.Join(tmpDir, "folder")
	hidden := filepath.Join(dir, ".hidden")
	visible := filepath.Join(dir, "visible")
	os.WriteFile(file, []byte("data"), 0o600)
	os.MkdirAll(dir, 0o755)
	os.WriteFile(hidden, []byte("secret"), 0o600)
	os.WriteFile(visible, []byte("ok"), 0o600)

	mgr.AddSharedPaths([]string{file, dir})
	cfg := mgr.Config()

	// List root: should return shared items.
	entries, err := mgr.ListVirtualDir("/")
	if err != nil {
		t.Fatalf("ListVirtualDir(\"/\") error = %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("len(entries) = %d, want 2", len(entries))
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Path, "/shared/") {
			t.Errorf("entry.Path = %q, want prefix /shared/", e.Path)
		}
		if strings.Contains(e.Path, tmpDir) {
			t.Errorf("entry.Path = %q contains real dir %q", e.Path, tmpDir)
		}
	}

	// Find the directory item ID.
	var dirItem model.SharedItem
	for _, item := range cfg.SharedItems {
		if item.IsDir {
			dirItem = item
			break
		}
	}

	// List inside shared directory: hidden file should be filtered.
	entries, err = mgr.ListVirtualDir("/shared/" + dirItem.ID)
	if err != nil {
		t.Fatalf("ListVirtualDir(/shared/%s) error = %v", dirItem.ID, err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (hidden filtered)", len(entries))
	}
	if entries[0].Name != "visible" {
		t.Errorf("entries[0].Name = %q, want visible", entries[0].Name)
	}
	if !strings.HasPrefix(entries[0].Path, "/shared/"+dirItem.ID+"/") {
		t.Errorf("entries[0].Path = %q, want /shared/%s/...", entries[0].Path, dirItem.ID)
	}
	if strings.Contains(entries[0].Path, tmpDir) {
		t.Errorf("entry.Path = %q contains real dir %q", entries[0].Path, tmpDir)
	}
}

func TestListVirtualDirDirectoryMode(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	rootDir := t.TempDir()
	visible := filepath.Join(rootDir, "visible.txt")
	hidden := filepath.Join(rootDir, ".hidden.txt")
	os.WriteFile(visible, []byte("ok"), 0o600)
	os.WriteFile(hidden, []byte("secret"), 0o600)

	mgr.SetMode(model.ShareModeDirectory)
	mgr.SetRootDir(rootDir)

	// List root.
	entries, err := mgr.ListVirtualDir("/")
	if err != nil {
		t.Fatalf("ListVirtualDir(\"/\") error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("len(entries) = %d, want 1 (hidden filtered)", len(entries))
	}
	if entries[0].Name != "visible.txt" {
		t.Errorf("entries[0].Name = %q, want visible.txt", entries[0].Name)
	}
	if !strings.HasPrefix(entries[0].Path, "/browse/") {
		t.Errorf("entries[0].Path = %q, want prefix /browse/", entries[0].Path)
	}
	if strings.Contains(entries[0].Path, rootDir) {
		t.Errorf("entry.Path = %q contains real rootDir %q", entries[0].Path, rootDir)
	}
}

func TestListVirtualDirNoRealPathLeak(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath() error = %v", err)
	}

	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "file.txt")
	os.WriteFile(file, []byte("data"), 0o600)
	mgr.AddSharedPaths([]string{file})

	entries, err := mgr.ListVirtualDir("/")
	if err != nil {
		t.Fatalf("ListVirtualDir(\"/\") error = %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.Path, tmpDir) {
			t.Errorf("entry.Path = %q contains real tmpDir %q", e.Path, tmpDir)
		}
		if strings.Contains(e.Path, "/Users/") || strings.Contains(e.Path, "/private/") {
			t.Errorf("entry.Path = %q leaks real Mac path", e.Path)
		}
	}
}

func TestValidateUploadName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid.txt", false},
		{"file-name_123.doc", false},
		{"", true},                // empty
		{".hidden", true},         // starts with dot
		{"path/to/file", true},    // contains /
		{"path\\to\\file", true},  // contains \
		{"file/../escape", true},  // changes under Clean
		{"./file", true},          // changes under Clean
	}
	for _, tt := range tests {
		err := ValidateUploadName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateUploadName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

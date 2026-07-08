package connection

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"AndroidFileTransfer/internal/model"
)

// newTestServer creates a WiFiServer backed by a fresh ShareManager in a temp
// directory. The ShareManager is returned so tests can add shared items.
func newTestServer(t *testing.T) (*WiFiServer, *ShareManager) {
	t.Helper()
	configPath := filepath.Join(t.TempDir(), "config.json")
	mgr, err := newShareManagerWithConfigPath(configPath)
	if err != nil {
		t.Fatalf("newShareManagerWithConfigPath: %v", err)
	}
	srv := NewWiFiServer()
	srv.SetShareManager(mgr)
	return srv, mgr
}

func TestWiFiServerFilesAPI_SelectedMode(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	// Before adding any shared item the root listing is empty.
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var files []model.FileInfo
	json.NewDecoder(resp.Body).Decode(&files)
	if len(files) != 0 {
		t.Fatalf("expected empty list before sharing, got %v", files)
	}

	// Add a real file; it should appear with a virtual path.
	dir := t.TempDir()
	hello := filepath.Join(dir, "hello.txt")
	os.WriteFile(hello, []byte("hi"), 0o644)

	if err := shareMgr.AddSharedPaths([]string{hello}); err != nil {
		t.Fatalf("AddSharedPaths: %v", err)
	}

	resp2, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	var files2 []model.FileInfo
	json.NewDecoder(resp2.Body).Decode(&files2)
	if len(files2) != 1 {
		t.Fatalf("expected 1 entry, got %v", files2)
	}
	if files2[0].Name != "hello.txt" {
		t.Errorf("Name = %q, want hello.txt", files2[0].Name)
	}
	// Virtual path must not contain the real dir.
	if strings.Contains(files2[0].Path, dir) {
		t.Errorf("Path = %q leaks real directory %q", files2[0].Path, dir)
	}
	if !strings.HasPrefix(files2[0].Path, "/shared/") {
		t.Errorf("Path = %q, want prefix /shared/", files2[0].Path)
	}
}

func TestWiFiServerFilesAPI_HiddenFiltered(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	dir := t.TempDir()
	visible := filepath.Join(dir, "visible.txt")
	hidden := filepath.Join(dir, ".secret")
	os.WriteFile(visible, []byte("ok"), 0o644)
	os.WriteFile(hidden, []byte("nope"), 0o644)

	// Switch to directory mode so the directory itself is browsed.
	shareMgr.SetMode(model.ShareModeDirectory)
	shareMgr.SetRootDir(dir)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var files []model.FileInfo
	json.NewDecoder(resp.Body).Decode(&files)

	for _, f := range files {
		if strings.HasPrefix(f.Name, ".") {
			t.Errorf("hidden file %q appears in response", f.Name)
		}
		if strings.Contains(f.Path, dir) {
			t.Errorf("Path %q leaks real dir", f.Path)
		}
	}
	found := false
	for _, f := range files {
		if f.Name == "visible.txt" {
			found = true
		}
	}
	if !found {
		t.Error("visible.txt not in response")
	}
}

func TestWiFiServerDownloadAPI(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("file content"), 0o644)

	if err := shareMgr.AddSharedPaths([]string{filePath}); err != nil {
		t.Fatalf("AddSharedPaths: %v", err)
	}
	cfg := shareMgr.Config()
	id := cfg.SharedItems[0].ID
	vpath := "/shared/" + id

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/download?path=" + vpath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "file content" {
		t.Errorf("body = %q, want file content", string(body))
	}
}

func TestWiFiServerDownloadDirectory403(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	dir := t.TempDir()
	if err := shareMgr.AddSharedPaths([]string{dir}); err != nil {
		t.Fatalf("AddSharedPaths: %v", err)
	}
	cfg := shareMgr.Config()
	id := cfg.SharedItems[0].ID

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/download?path=/shared/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for directory download, got %d", resp.StatusCode)
	}
}

func TestWiFiServerDownloadHidden403(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	dir := t.TempDir()
	shareMgr.SetMode(model.ShareModeDirectory)
	shareMgr.SetRootDir(dir)

	hidden := filepath.Join(dir, ".hidden.txt")
	os.WriteFile(hidden, []byte("secret"), 0o600)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	// Try to download a hidden file using the directory mode virtual path.
	resp, err := http.Get(ts.URL + "/api/download?path=/browse/.hidden.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for hidden file, got %d", resp.StatusCode)
	}
}

func TestWiFiServerUploadAPI(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	uploadDir := t.TempDir()
	shareMgr.SetUploadDir(uploadDir)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	// ?path is ignored; upload always lands in upload dir.
	body := strings.NewReader("--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"up.txt\"\r\n\r\nupload content\r\n--boundary--\r\n")
	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?path=/ignored/path", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	content, _ := os.ReadFile(filepath.Join(uploadDir, "up.txt"))
	if string(content) != "upload content" {
		t.Errorf("content = %q, want upload content", string(content))
	}
}

func TestWiFiServerUploadHiddenName400(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	uploadDir := t.TempDir()
	shareMgr.SetUploadDir(uploadDir)

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	body := strings.NewReader("--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\".hidden\"\r\n\r\ndata\r\n--boundary--\r\n")
	req, _ := http.NewRequest("POST", ts.URL+"/api/upload", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for hidden upload name, got %d", resp.StatusCode)
	}
}

func TestWiFiServerJSONNoRealPathLeak(t *testing.T) {
	srv, shareMgr := newTestServer(t)

	dir := t.TempDir()
	file := filepath.Join(dir, "visible.txt")
	os.WriteFile(file, []byte("data"), 0o644)
	shareMgr.AddSharedPaths([]string{file})

	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	jsonStr := string(raw)

	if strings.Contains(jsonStr, dir) {
		t.Errorf("JSON response leaks real dir %q:\n%s", dir, jsonStr)
	}
	// The macOS temp path begins with /private or /tmp.
	if strings.Contains(jsonStr, "/private/") || strings.Contains(jsonStr, "/Users/") {
		t.Errorf("JSON response leaks real Mac path:\n%s", jsonStr)
	}
}

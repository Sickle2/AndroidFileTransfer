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

func TestWiFiServerFilesAPI(t *testing.T) {
	// create temp dir with one file
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0o644)

	srv := &WiFiServer{rootDir: dir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/files?path=" + dir)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var files []model.FileInfo
	json.NewDecoder(resp.Body).Decode(&files)
	if len(files) == 0 {
		t.Fatal("expected at least one file")
	}
	found := false
	for _, f := range files {
		if f.Name == "hello.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("hello.txt not in response: %+v", files)
	}
}

func TestWiFiServerDownloadAPI(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	os.WriteFile(filePath, []byte("file content"), 0o644)

	srv := &WiFiServer{rootDir: dir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/download?path=" + filePath)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "file content" {
		t.Errorf("expected 'file content', got %q", body)
	}
}

func TestWiFiServerUploadAPI(t *testing.T) {
	dir := t.TempDir()
	srv := &WiFiServer{rootDir: dir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	body := strings.NewReader("--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"up.txt\"\r\n\r\nupload content\r\n--boundary--\r\n")
	req, _ := http.NewRequest("POST", ts.URL+"/api/upload?path="+dir, body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=boundary")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "up.txt"))
	if string(content) != "upload content" {
		t.Errorf("expected 'upload content', got %q", content)
	}
}

func TestResolvePathRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	srv := &WiFiServer{rootDir: dir}

	_, err := srv.resolvePath("../../../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
}

func TestResolvePathRejectsAbsoluteOutside(t *testing.T) {
	dir := t.TempDir()
	srv := &WiFiServer{rootDir: dir}

	_, err := srv.resolvePath("/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path outside root, got nil")
	}
}

func TestResolvePathAllowsInside(t *testing.T) {
	dir := t.TempDir()
	srv := &WiFiServer{rootDir: dir}

	os.MkdirAll(filepath.Join(dir, "sub"), 0o755)
	resolved, err := srv.resolvePath(filepath.Join(dir, "sub"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.HasPrefix(resolved, dir) {
		t.Errorf("expected resolved path to be within %q, got %q", dir, resolved)
	}
}

func TestDownloadRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	srv := &WiFiServer{rootDir: dir}
	ts := httptest.NewServer(srv.handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/download?path=../../../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

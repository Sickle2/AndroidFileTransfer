package connection

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"AndroidFileTransfer/internal/model"
	"AndroidFileTransfer/internal/util"
)

// WiFiServer serves files over HTTP for Android browser access.
type WiFiServer struct {
	rootDir string
	server  *http.Server
	address string // http://<ip>:<port>
	qrCode  string // data URI
	uiFS    fs.FS  // embedded android-ui filesystem, injected by main package
}

// NewWiFiServer creates a server restricted to rootDir. If rootDir is empty,
// the user's home directory is used as the root.
func NewWiFiServer(rootDir string) *WiFiServer {
	if rootDir == "" {
		home, _ := os.UserHomeDir()
		rootDir = home
	}
	abs, err := filepath.Abs(filepath.Clean(rootDir))
	if err != nil {
		abs = filepath.Clean(rootDir)
	}
	return &WiFiServer{rootDir: abs, uiFS: nil}
}

// resolvePath resolves userPath against rootDir and ensures the result stays
// within rootDir. userPath may be relative (interpreted relative to rootDir)
// or absolute. Returns an error if the resolved path escapes rootDir.
func (s *WiFiServer) resolvePath(userPath string) (string, error) {
	var candidate string
	if filepath.IsAbs(userPath) {
		candidate = userPath
	} else {
		candidate = filepath.Join(s.rootDir, userPath)
	}
	cleaned, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return "", err
	}
	if cleaned != s.rootDir && !strings.HasPrefix(cleaned, s.rootDir+string(os.PathSeparator)) {
		return "", errors.New("禁止访问根目录外的路径")
	}
	return cleaned, nil
}

// Start finds a free port (8080-8084) and begins serving.
func (s *WiFiServer) Start() error {
	mux := s.handler()
	var ln net.Listener
	var port int
	for _, p := range []int{8080, 8081, 8082, 8083, 8084} {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", p))
		if err == nil {
			ln = l
			port = p
			break
		}
	}
	if ln == nil {
		return fmt.Errorf("启动 WiFi 服务失败：端口 8080-8084 均被占用")
	}

	ip, err := util.GetLocalIP()
	if err != nil {
		ip = "localhost"
	}
	s.address = fmt.Sprintf("http://%s:%d", ip, port)
	s.qrCode, _ = util.GenerateQRCodeBase64(s.address)

	s.server = &http.Server{Handler: mux}
	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("WiFiServer error", "err", err)
		}
	}()
	slog.Info("WiFiServer started", "address", s.address)
	return nil
}

// Stop shuts down the HTTP server.
func (s *WiFiServer) Stop() {
	if s.server != nil {
		s.server.Close()
	}
}

// Address returns the server URL (http://<ip>:<port>).
func (s *WiFiServer) Address() string { return s.address }

// QRCode returns a PNG data URI for the server URL.
func (s *WiFiServer) QRCode() string { return s.qrCode }

// SetUIFS injects the embedded android-ui filesystem so it is served at "/".
// Must be called before Start().
func (s *WiFiServer) SetUIFS(uiFS fs.FS) { s.uiFS = uiFS }

// handler builds the HTTP mux (exported for testing with httptest).
func (s *WiFiServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/upload", s.handleUpload)
	// serve UI static files at root, if one has been injected
	if s.uiFS != nil {
		mux.Handle("/", http.FileServer(http.FS(s.uiFS)))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			fmt.Fprint(w, "Android File Transfer server running")
		})
	}
	return mux
}

func (s *WiFiServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		path = s.rootDir
	}
	path, err := s.resolvePath(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var files []model.FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, model.FileInfo{
			Name:    e.Name(),
			Path:    filepath.Join(path, e.Name()),
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			ModTime: info.ModTime(),
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func (s *WiFiServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	path, err := s.resolvePath(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	io.Copy(w, f)
}

func (s *WiFiServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	destDir := r.URL.Query().Get("path")
	if destDir == "" {
		destDir = s.rootDir
	}
	destDir, err := s.resolvePath(destDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, fh := range r.MultipartForm.File["file"] {
		src, err := fh.Open()
		if err != nil {
			continue
		}
		defer src.Close()
		dst, err := os.Create(filepath.Join(destDir, filepath.Base(fh.Filename)))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer dst.Close()
		io.Copy(dst, src)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true}`))
}

package connection

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

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

// NewWiFiServer creates a server with the user's home dir as root.
func NewWiFiServer() *WiFiServer {
	home, _ := os.UserHomeDir()
	return &WiFiServer{rootDir: home, uiFS: nil}
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
		return fmt.Errorf("WiFiServer: no free port in range 8080-8084")
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
	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var files []model.FileInfo
	for _, e := range entries {
		info, _ := e.Info()
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
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()
	info, _ := f.Stat()
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(path))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	io.Copy(w, f)
}

func (s *WiFiServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	destDir := r.URL.Query().Get("path")
	if destDir == "" {
		destDir = s.rootDir
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
		dst, err := os.Create(filepath.Join(destDir, fh.Filename))
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

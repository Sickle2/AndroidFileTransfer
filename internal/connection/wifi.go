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
// Wire a ShareManager via SetShareManager before calling Start.
type WiFiServer struct {
	shareMgr *ShareManager
	server   *http.Server
	address  string // http://<ip>:<port>
	qrCode   string // data URI
	uiFS     fs.FS  // embedded android-ui filesystem, injected by main package
}

// NewWiFiServer creates a WiFi HTTP server. Call SetShareManager to configure
// the sharing scope, then Start to begin serving.
func NewWiFiServer() *WiFiServer {
	return &WiFiServer{}
}

// SetShareManager wires the share configuration into the server.
// Must be called before Start.
func (s *WiFiServer) SetShareManager(mgr *ShareManager) { s.shareMgr = mgr }

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
// Must be called before Start.
func (s *WiFiServer) SetUIFS(uiFS fs.FS) { s.uiFS = uiFS }

// handler builds the HTTP mux. Used directly in tests via httptest.
func (s *WiFiServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/files", s.handleFiles)
	mux.HandleFunc("/api/download", s.handleDownload)
	mux.HandleFunc("/api/upload", s.handleUpload)
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

// handleFiles lists virtual directory contents via the ShareManager.
// The ?path parameter is a virtual path (e.g. "/" or "/shared/<id>").
// All returned FileInfo.Path values are virtual — no real Mac paths leak.
func (s *WiFiServer) handleFiles(w http.ResponseWriter, r *http.Request) {
	if s.shareMgr == nil {
		http.Error(w, "共享管理器未初始化", http.StatusInternalServerError)
		return
	}
	vpath := r.URL.Query().Get("path")
	if vpath == "" {
		vpath = "/"
	}
	files, err := s.shareMgr.ListVirtualDir(vpath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if files == nil {
		files = []model.FileInfo{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

// handleDownload resolves a virtual path and streams the file to the client.
// Downloading a directory returns 400; hidden or out-of-bounds paths return 403.
func (s *WiFiServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	if s.shareMgr == nil {
		http.Error(w, "共享管理器未初始化", http.StatusInternalServerError)
		return
	}
	vpath := r.URL.Query().Get("path")
	realPath, info, err := s.shareMgr.ResolveVirtualPath(vpath)
	if err != nil {
		if err == errVirtualDir {
			http.Error(w, "无法下载目录", http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusForbidden)
		}
		return
	}
	if info.IsDir() {
		http.Error(w, "无法下载目录", http.StatusBadRequest)
		return
	}
	f, err := os.Open(realPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(realPath))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	io.Copy(w, f)
}

// handleUpload accepts multipart file uploads and saves them to the upload
// directory from ShareManager. The ?path query parameter is intentionally
// ignored — uploads always land in the configured upload directory.
func (s *WiFiServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if s.shareMgr == nil {
		http.Error(w, "共享管理器未初始化", http.StatusInternalServerError)
		return
	}
	uploadDir := s.shareMgr.UploadDir()
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		http.Error(w, "创建接收目录失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	for _, fh := range r.MultipartForm.File["file"] {
		name := filepath.Base(fh.Filename)
		if err := ValidateUploadName(name); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		src, err := fh.Open()
		if err != nil {
			continue
		}
		func() {
			defer src.Close()
			dst, err := os.Create(filepath.Join(uploadDir, name))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer dst.Close()
			io.Copy(dst, src)
		}()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"success":true}`))
}

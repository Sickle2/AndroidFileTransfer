package connection

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"AndroidFileTransfer/internal/model"
)

// Manager aggregates WiFiServer, ShareManager, and ADBManager, routing file
// operations by deviceID prefix ("wifi:" or "adb:").
type Manager struct {
	wifiSrv    *WiFiServer
	shareMgr   *ShareManager
	adbMgr     *ADBManager
	broadcaster Broadcaster

	mu         sync.RWMutex
	adbDevices []model.Device

	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewManager creates a Manager with the given collaborators.
// adbMgr and shareMgr may be nil; the Manager logs a warning and continues
// in a degraded mode (WiFi-only or no share config, respectively).
func NewManager(wifiSrv *WiFiServer, adbMgr *ADBManager, shareMgr *ShareManager) *Manager {
	return &Manager{
		wifiSrv:  wifiSrv,
		adbMgr:   adbMgr,
		shareMgr: shareMgr,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the WiFiServer and begins ADB device polling (every 2 s).
func (m *Manager) Start() error {
	if err := m.wifiSrv.Start(); err != nil {
		return fmt.Errorf("WiFi 服务启动失败: %w", err)
	}

	if m.adbMgr == nil {
		slog.Warn("manager: ADB unavailable, running in WiFi-only mode")
	} else {
		// Initial detection so ListDevices is populated before first call.
		m.refreshADB()
		m.wg.Add(1)
		go m.pollADB()
	}
	return nil
}

// Stop shuts down the WiFiServer, stops ADB polling, and closes the broadcaster.
// Safe to call multiple times; subsequent calls are no-ops.
func (m *Manager) Stop() {
	m.stopOnce.Do(func() { close(m.stopCh) })
	m.wg.Wait()
	m.wifiSrv.Stop()
	m.broadcaster.Close()
}

// pollADB runs in a goroutine and refreshes ADB devices every 2 seconds.
func (m *Manager) pollADB() {
	defer m.wg.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.refreshADB()
		}
	}
}

// refreshADB calls DetectDevices and stores the result under the mutex.
func (m *Manager) refreshADB() {
	if m.adbMgr == nil {
		return
	}
	devices := m.adbMgr.DetectDevices()
	m.mu.Lock()
	m.adbDevices = devices
	m.mu.Unlock()
}

// RefreshDevices triggers an immediate ADB device refresh (blocking).
func (m *Manager) RefreshDevices() {
	m.refreshADB()
}

// ListDevices returns the combined list of WiFi (fixed single entry) and ADB
// devices currently known to the Manager.
func (m *Manager) ListDevices() []model.Device {
	addr := m.wifiSrv.Address()
	wifiID := "wifi:" + strings.TrimPrefix(addr, "http://")
	devices := []model.Device{
		{
			ID:     wifiID,
			Name:   addr,
			Type:   "wifi",
			Status: "connected",
		},
	}

	m.mu.RLock()
	devices = append(devices, m.adbDevices...)
	m.mu.RUnlock()

	return devices
}

// GetFileList returns file entries for the given deviceID and path.
// For WiFi devices the path is a virtual path resolved via ShareManager.
// For ADB devices it delegates to ADBManager.ListFiles.
func (m *Manager) GetFileList(deviceID, path string) ([]model.FileInfo, error) {
	switch {
	case strings.HasPrefix(deviceID, "wifi:"):
		return m.wifiFileList(path)
	case strings.HasPrefix(deviceID, "adb:"):
		if m.adbMgr == nil {
			return nil, fmt.Errorf("ADB 不可用")
		}
		serial := strings.TrimPrefix(deviceID, "adb:")
		return m.adbMgr.ListFiles(serial, path)
	default:
		return nil, fmt.Errorf("未知的设备 ID 前缀: %q", deviceID)
	}
}

// wifiFileList lists the virtual directory via ShareManager.
// path is a virtual path (e.g. "/" or "/shared/<id>"); no real Mac paths
// are returned to callers.
func (m *Manager) wifiFileList(path string) ([]model.FileInfo, error) {
	if m.shareMgr == nil {
		return nil, fmt.Errorf("共享管理器未初始化")
	}
	if path == "" {
		path = "/"
	}
	return m.shareMgr.ListVirtualDir(path)
}

// Download transfers a file from the device to localPath.
// WiFi devices are not supported for download via the Manager (files are served
// over HTTP directly to the Android browser). ADB devices use adb pull.
func (m *Manager) Download(deviceID, remotePath, localPath string) error {
	switch {
	case strings.HasPrefix(deviceID, "wifi:"):
		return fmt.Errorf("WiFi 下载由 HTTP 服务器处理")
	case strings.HasPrefix(deviceID, "adb:"):
		if m.adbMgr == nil {
			return fmt.Errorf("ADB 不可用")
		}
		serial := strings.TrimPrefix(deviceID, "adb:")
		m.broadcaster.Publish(model.TransferProgress{
			DeviceID:  deviceID,
			FileName:  remotePath,
			BytesDone: 0,
		})
		err := m.adbMgr.Pull(serial, remotePath, localPath)
		if err != nil {
			m.broadcaster.Publish(model.TransferProgress{
				DeviceID: deviceID,
				FileName: remotePath,
				Error:    err.Error(),
			})
			return err
		}
		m.broadcaster.Publish(model.TransferProgress{
			DeviceID:  deviceID,
			FileName:  remotePath,
			BytesDone: -1, // sentinel: completed
		})
		return nil
	default:
		return fmt.Errorf("未知的设备 ID 前缀: %q", deviceID)
	}
}

// Upload transfers a file from localPath to the device.
// WiFi devices are not supported for upload via the Manager (uploads happen
// over HTTP from the Android browser). ADB devices use adb push.
func (m *Manager) Upload(deviceID, localPath, remotePath string) error {
	switch {
	case strings.HasPrefix(deviceID, "wifi:"):
		return fmt.Errorf("WiFi 上传由 HTTP 服务器处理")
	case strings.HasPrefix(deviceID, "adb:"):
		if m.adbMgr == nil {
			return fmt.Errorf("ADB 不可用")
		}
		serial := strings.TrimPrefix(deviceID, "adb:")
		m.broadcaster.Publish(model.TransferProgress{
			DeviceID:  deviceID,
			FileName:  localPath,
			BytesDone: 0,
		})
		err := m.adbMgr.Push(serial, localPath, remotePath)
		if err != nil {
			m.broadcaster.Publish(model.TransferProgress{
				DeviceID: deviceID,
				FileName: localPath,
				Error:    err.Error(),
			})
			return err
		}
		m.broadcaster.Publish(model.TransferProgress{
			DeviceID:  deviceID,
			FileName:  localPath,
			BytesDone: -1, // sentinel: completed
		})
		return nil
	default:
		return fmt.Errorf("未知的设备 ID 前缀: %q", deviceID)
	}
}

// SubscribeProgress returns a channel that receives TransferProgress events.
func (m *Manager) SubscribeProgress() <-chan model.TransferProgress {
	return m.broadcaster.Subscribe()
}

// WiFiAddress delegates to the underlying WiFiServer.Address().
func (m *Manager) WiFiAddress() string {
	return m.wifiSrv.Address()
}

// WiFiQRCode delegates to the underlying WiFiServer.QRCode().
func (m *Manager) WiFiQRCode() string {
	return m.wifiSrv.QRCode()
}

// ShareConfig returns a copy of the current WiFi share configuration.
func (m *Manager) ShareConfig() model.ShareConfig {
	if m.shareMgr == nil {
		return model.ShareConfig{}
	}
	return m.shareMgr.Config()
}

// WiFiServer returns the underlying WiFiServer so callers (in package main) can
// call SetUIFS before Start().
func (m *Manager) WiFiServer() *WiFiServer {
	return m.wifiSrv
}

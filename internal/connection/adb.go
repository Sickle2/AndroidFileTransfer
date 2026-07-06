package connection

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"AndroidFileTransfer/internal/model"
)

// ADBManager communicates with Android devices via the adb command-line tool.
type ADBManager struct {
	adbPath string
}

// NewADBManager locates the adb binary; returns an error if not found.
func NewADBManager() (*ADBManager, error) {
	path, err := exec.LookPath("adb")
	if err != nil {
		return nil, fmt.Errorf("adb 未安装，请运行：brew install android-platform-tools")
	}
	return &ADBManager{adbPath: path}, nil
}

// runCmd runs `adb <args>` and returns combined output.
func (m *ADBManager) runCmd(args ...string) (string, error) {
	out, err := exec.Command(m.adbPath, args...).CombinedOutput()
	return string(out), err
}

// DetectDevices returns all currently connected ADB devices.
func (m *ADBManager) DetectDevices() []model.Device {
	out, err := m.runCmd("devices")
	if err != nil {
		slog.Error("adb devices failed", "err", err)
		return nil
	}
	return parseADBDevices(out)
}

// parseADBDevices parses the output of `adb devices`.
func parseADBDevices(output string) []model.Device {
	var devices []model.Device
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] == "List" {
			continue
		}
		serial := fields[0]
		state := strings.Join(fields[1:], " ")
		status := "connected"
		if state != "device" {
			status = "disconnected"
		}
		devices = append(devices, model.Device{
			ID:     "adb:" + serial,
			Name:   serial,
			Type:   "adb",
			Status: status,
		})
	}
	return devices
}

// ListFiles lists files in path on the given device (serial without "adb:" prefix).
func (m *ADBManager) ListFiles(serial, path string) ([]model.FileInfo, error) {
	out, err := m.runCmd("-s", serial, "shell", "ls", "-la", path)
	if err != nil {
		return nil, fmt.Errorf("adb ls: %w", err)
	}
	return parseADBFiles(out, path), nil
}

// parseADBFiles parses `adb shell ls -la` output.
func parseADBFiles(output, dir string) []model.FileInfo {
	var files []model.FileInfo
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}
		perms := fields[0]

		// The filename is everything after the "YYYY-MM-DD HH:MM" timestamp,
		// which sits at fixed columns (fields[5], fields[6]). Locating that
		// anchor in the raw line (rather than just taking the last field)
		// preserves filenames that contain spaces.
		dateStr, timeStr := fields[5], fields[6]
		anchor := dateStr + " " + timeStr
		name := fields[len(fields)-1]
		if idx := strings.Index(line, anchor); idx >= 0 {
			name = strings.TrimSpace(line[idx+len(anchor):])
		}
		// Symlink lines look like "... 10:30 link -> target"; keep only the link name.
		if arrow := strings.Index(name, " -> "); arrow >= 0 {
			name = name[:arrow]
		}
		if name == "" || name == "." || name == ".." {
			continue
		}
		isDir := strings.HasPrefix(perms, "d")
		size, _ := strconv.ParseInt(fields[4], 10, 64)
		modTime := time.Time{}
		if t, err := time.Parse("2006-01-02 15:04", anchor); err == nil {
			modTime = t
		}
		files = append(files, model.FileInfo{
			Name:    name,
			Path:    filepath.Join(dir, name),
			Size:    size,
			IsDir:   isDir,
			ModTime: modTime,
		})
	}
	return files
}

// Pull downloads a file from the device to localPath.
func (m *ADBManager) Pull(serial, remotePath, localPath string) error {
	slog.Info("adb pull", "serial", serial, "remote", remotePath, "local", localPath)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	_, err := m.runCmd("-s", serial, "pull", remotePath, localPath)
	return err
}

// Push uploads a file from localPath to the device.
func (m *ADBManager) Push(serial, localPath, remotePath string) error {
	slog.Info("adb push", "serial", serial, "local", localPath, "remote", remotePath)
	_, err := m.runCmd("-s", serial, "push", localPath, remotePath)
	return err
}

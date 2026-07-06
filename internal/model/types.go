package model

import "time"

// Device represents a connected Android device (WiFi, ADB, or MTP).
type Device struct {
	ID     string `json:"id"`     // "wifi:<ip>:<port>" | "adb:<serial>" | "mtp:<id>"
	Name   string `json:"name"`
	Type   string `json:"type"`   // "wifi" | "adb" | "mtp"
	Status string `json:"status"` // "connected" | "transferring" | "disconnected"
}

// FileInfo describes a file or directory entry.
type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"isDir"`
	ModTime time.Time `json:"modTime"`
}

// TransferProgress is emitted during file transfer operations.
type TransferProgress struct {
	DeviceID   string `json:"deviceId"`
	FileName   string `json:"fileName"`
	BytesDone  int64  `json:"bytesDone"`
	TotalBytes int64  `json:"totalBytes"`
	Error      string `json:"error,omitempty"`
}

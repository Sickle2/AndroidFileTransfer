package connection

import (
	"testing"
)

func TestADBManagerNew_ADBNotFound(t *testing.T) {
	// Override adb path to something that doesn't exist
	mgr := &ADBManager{adbPath: "/nonexistent/adb"}
	_, err := mgr.runCmd("version")
	if err == nil {
		t.Fatal("expected error when adb not found")
	}
}

func TestParseADBDevices(t *testing.T) {
	output := "List of devices attached\nabc123\tdevice\nxyz789\tunauthorized\n"
	devices := parseADBDevices(output)
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
	if devices[0].ID != "adb:abc123" {
		t.Errorf("expected adb:abc123, got %s", devices[0].ID)
	}
	if devices[0].Status != "connected" {
		t.Errorf("expected connected, got %s", devices[0].Status)
	}
	if devices[1].Status != "disconnected" {
		t.Errorf("unauthorized device should be disconnected, got %s", devices[1].Status)
	}
}

func TestParseADBFiles(t *testing.T) {
	output := `-rw-rw-rw- 1 root sdcard_rw  1024 2024-01-15 10:30 file.txt
drwxrwx--x 2 root sdcard_rw  4096 2024-01-15 10:00 folder
`
	files := parseADBFiles(output, "/sdcard")
	if len(files) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(files))
	}
	if files[0].Name != "file.txt" || files[0].Size != 1024 || files[0].IsDir {
		t.Errorf("unexpected file: %+v", files[0])
	}
	if files[1].Name != "folder" || !files[1].IsDir {
		t.Errorf("unexpected dir: %+v", files[1])
	}
}

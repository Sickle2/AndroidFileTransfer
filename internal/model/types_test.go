package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDeviceJSONRoundTrip(t *testing.T) {
	d := Device{ID: "adb:abc123", Name: "Pixel 6", Type: "adb", Status: "connected"}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	var d2 Device
	if err := json.Unmarshal(b, &d2); err != nil {
		t.Fatal(err)
	}
	if d2.ID != d.ID || d2.Type != d.Type {
		t.Errorf("round-trip mismatch: got %+v", d2)
	}
}

func TestFileInfoJSONRoundTrip(t *testing.T) {
	f := FileInfo{Name: "test.txt", Path: "/sdcard/test.txt", Size: 1024, IsDir: false, ModTime: time.Now().Truncate(time.Second)}
	b, _ := json.Marshal(f)
	var f2 FileInfo
	json.Unmarshal(b, &f2)
	if f2.Name != f.Name || f2.Size != f.Size {
		t.Errorf("round-trip mismatch: got %+v", f2)
	}
}

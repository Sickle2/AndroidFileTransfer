package util

import (
	"strings"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		t.Fatalf("GetLocalIP() error: %v", err)
	}
	if ip == "" || ip == "127.0.0.1" {
		t.Errorf("expected LAN IP, got %q", ip)
	}
	if len(strings.Split(ip, ".")) != 4 {
		t.Errorf("expected IPv4, got %q", ip)
	}
}

func TestGenerateQRCodeBase64(t *testing.T) {
	b64, err := GenerateQRCodeBase64("http://192.168.1.100:8080")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if !strings.HasPrefix(b64, "data:image/png;base64,") {
		t.Errorf("expected data URI prefix, got: %q", b64[:30])
	}
}

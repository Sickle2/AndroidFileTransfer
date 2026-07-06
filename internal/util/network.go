package util

import (
	"encoding/base64"
	"fmt"
	"net"

	qrcode "github.com/skip2/go-qrcode"
)

// GetLocalIP returns the machine's LAN IPv4 by dialing a UDP connection
// (no packet is actually sent).
func GetLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("GetLocalIP: %w", err)
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String(), nil
}

// GenerateQRCodeBase64 encodes url as a QR code and returns a PNG data URI.
func GenerateQRCodeBase64(url string) (string, error) {
	png, err := qrcode.Encode(url, qrcode.Medium, 256)
	if err != nil {
		return "", fmt.Errorf("GenerateQRCodeBase64: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

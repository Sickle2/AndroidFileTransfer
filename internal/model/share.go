package model

// ShareMode controls how files are exposed over WiFi sharing.
type ShareMode string

const (
	// ShareModeSelected exposes only explicitly selected shared items.
	ShareModeSelected ShareMode = "selected"
	// ShareModeDirectory exposes a configured root directory.
	ShareModeDirectory ShareMode = "directory"
)

// SharedItem describes one file or directory selected for WiFi sharing.
// Path is intentionally omitted from JSON responses so real Mac paths are not
// exposed to phones or other clients.
type SharedItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Path  string `json:"-"`
	IsDir bool   `json:"isDir"`
}

// ShareConfig stores WiFi sharing scope and upload destination settings.
type ShareConfig struct {
	Mode        ShareMode    `json:"mode"`
	RootDir     string       `json:"rootDir"`
	UploadDir   string       `json:"uploadDir"`
	SharedItems []SharedItem `json:"sharedItems"`
}

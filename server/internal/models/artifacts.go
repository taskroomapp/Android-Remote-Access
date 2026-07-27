package models

import (
	"time"

	"github.com/google/uuid"
)

// StoredLocation is a persisted GPS fix.
type StoredLocation struct {
	ID            uuid.UUID  `json:"id"`
	DeviceID      uuid.UUID  `json:"device_id"`
	Latitude      float64    `json:"latitude"`
	Longitude     float64    `json:"longitude"`
	Altitude      *float64   `json:"altitude,omitempty"`
	Accuracy      *float64   `json:"accuracy,omitempty"`
	Provider      string     `json:"provider"`
	Stale         bool       `json:"stale"`
	FixTime       *time.Time `json:"fix_time,omitempty"`
	DataEntryDate time.Time  `json:"data_entry_date"`
	EncBlob       string     `json:"-"`
}

// StoredFileEntry is a persisted file-tree metadata row (not file content).
type StoredFileEntry struct {
	ID            uuid.UUID  `json:"id"`
	DeviceID      uuid.UUID  `json:"device_id"`
	Path          string     `json:"path"`
	PathFP        string     `json:"-"`
	Name          string     `json:"name"`
	IsDirectory   bool       `json:"is_directory"`
	Size          int64      `json:"size"`
	Permissions   string     `json:"permissions"`
	ModifiedTime  *time.Time `json:"modified_time,omitempty"`
	DataEntryDate time.Time  `json:"data_entry_date"`
}

// ArtifactsSaveRequest persists location points, file inventory, and media blobs.
type ArtifactsSaveRequest struct {
	Locations []map[string]interface{} `json:"locations,omitempty"`
	Files     []map[string]interface{} `json:"files,omitempty"`
	Media     []MediaSaveItem          `json:"media,omitempty"`
}

// MediaSaveItem is a camera/mic (or other) blob uploaded from the panel.
type MediaSaveItem struct {
	FileName  string   `json:"file_name"`
	FileType  string   `json:"file_type"` // image, audio, video, document
	Source    string   `json:"source"`    // camera_snapshot, mic_stop, file_download, panel
	Camera    string   `json:"camera,omitempty"`
	MimeType  string   `json:"mime_type,omitempty"`
	Base64    string   `json:"base64,omitempty"`
	DataURL   string   `json:"data_url,omitempty"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

// ArtifactsSaveResult reports upsert counts.
type ArtifactsSaveResult struct {
	LocationsSaved int `json:"locations_saved"`
	FilesSaved     int `json:"files_saved"`
	MediaSaved     int `json:"media_saved"`
}

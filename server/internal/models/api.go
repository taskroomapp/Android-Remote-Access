package models

import "time"

// API Request/Response structures

// LoginRequest represents admin login credentials
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=8"`
}

// LoginResponse represents successful authentication response
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	Admin        AdminInfo `json:"admin"`
}

// AdminInfo represents basic admin information
type AdminInfo struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Role        string   `json:"role"`
	Permissions []string `json:"permissions"`
}

// RefreshTokenRequest represents token refresh request
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// CommandRequest represents a command to be sent to a device
type CommandRequest struct {
	DeviceID       string `json:"device_id" validate:"required,uuid"`
	CommandType    string `json:"command_type" validate:"required"`
	Payload        string `json:"payload"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Priority       int    `json:"priority"`
}

// CommandResponse represents command submission acknowledgment
type CommandResponse struct {
	TransactionID string    `json:"transaction_id"`
	Status        string    `json:"status"`
	Queued        bool      `json:"queued"`
	Message       string    `json:"message"`
	CreatedAt     time.Time `json:"created_at"`
}

// CommandStatusResponse represents the status of a command
type CommandStatusResponse struct {
	TransactionID string    `json:"transaction_id"`
	DeviceID      string    `json:"device_id"`
	CommandType   string    `json:"command_type"`
	Status        string    `json:"status"`
	Data          []byte    `json:"data,omitempty"`
	Error         string    `json:"error,omitempty"`
	IssuedBy      string    `json:"issued_by"`
	IssuedAt      time.Time `json:"issued_at"`
	CompletedAt   time.Time `json:"completed_at,omitempty"`
}

// DeviceListResponse represents the list of enrolled devices
type DeviceListResponse struct {
	Devices     []DeviceOverview `json:"devices"`
	Total       int             `json:"total"`
	OnlineCount int             `json:"online_count"`
	OfflineCount int            `json:"offline_count"`
}

// DeviceOverview represents summarized device info for lists
type DeviceOverview struct {
	ID           string    `json:"id"`
	FriendlyName string    `json:"friendly_name"`
	Owner        string    `json:"owner"`
	Status       string    `json:"status"`
	BatteryLevel int       `json:"battery_level"`
	LastCheckIn  time.Time `json:"last_check_in"`
}

// SearchRequest represents audit log search parameters
type SearchRequest struct {
	Query        string    `json:"query"`
	DeviceID     string    `json:"device_id"`
	AdminID      string    `json:"admin_id"`
	CommandType  string    `json:"command_type"`
	Status       string    `json:"status"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	Page         int       `json:"page"`
	PageSize     int       `json:"page_size"`
}

// SearchResponse represents paginated search results
type SearchResponse struct {
	Results    []AuditLogEntry `json:"results"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

// AuditLogEntry represents a single audit log entry for display
type AuditLogEntry struct {
	ID               string    `json:"id"`
	TransactionID   string    `json:"transaction_id"`
	AdministratorID  string    `json:"administrator_id"`
	AdministratorName string   `json:"administrator_name"`
	DeviceID         string    `json:"device_id"`
	DeviceName       string    `json:"device_name"`
	CommandType      string    `json:"command_type"`
	CommandPayload   string    `json:"command_payload"`
	Status           string    `json:"status"`
	IPAddress        string    `json:"ip_address"`
	Timestamp        time.Time `json:"timestamp"`
}

// EnrollDeviceRequest represents device enrollment request
type EnrollDeviceRequest struct {
	DeviceUUID       string `json:"device_uuid" validate:"required"`
	FriendlyName     string `json:"friendly_name" validate:"required"`
	Owner            string `json:"owner"`
	OSVersion        string `json:"os_version"`
	HardwareModel    string `json:"hardware_model"`
	CertificateHash  string `json:"certificate_hash"`
	X25519PublicKey  string `json:"x25519_public_key"`
	Ed25519PublicKey string `json:"ed25519_public_key"`
	KeyFingerprint   string `json:"key_fingerprint"`
}

// EnrollDeviceResponse represents device enrollment response
type EnrollDeviceResponse struct {
	DeviceID  string `json:"device_id"`
	ServerURL string `json:"server_url"`
	Message   string `json:"message"`
	Status    string `json:"status"`
}

// DashboardStats represents dashboard statistics
type DashboardStats struct {
	TotalDevices      int            `json:"total_devices"`
	OnlineDevices     int            `json:"online_devices"`
	OfflineDevices    int            `json:"offline_devices"`
	TotalCommands     int64          `json:"total_commands"`
	CommandsToday     int64          `json:"commands_today"`
	CommandsThisWeek  int64          `json:"commands_this_week"`
	SuccessRate       float64        `json:"success_rate"`
	TopCommands       map[string]int `json:"top_commands"`
	ActiveAdmins      int            `json:"active_admins"`
	RecentAlerts      []Alert        `json:"recent_alerts"`
}

// Alert represents a system alert
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	DeviceID  string    `json:"device_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Severity  string    `json:"severity"`
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// HealthResponse represents server health status
type HealthResponse struct {
	Status    string            `json:"status"`
	Version   string            `json:"version"`
	Uptime    time.Duration     `json:"uptime"`
	Checks    map[string]string `json:"checks"`
}

package models

import (
	"time"

	"github.com/google/uuid"
)

// Device represents an enrolled Android device in the system
type Device struct {
	ID              uuid.UUID `json:"id" db:"id"`
	FriendlyName    string    `json:"friendly_name" db:"friendly_name"`
	Owner           string    `json:"owner" db:"owner"`
	OSVersion       string    `json:"os_version" db:"os_version"`
	HardwareModel   string    `json:"hardware_model" db:"hardware_model"`
	DeviceUUID      string    `json:"device_uuid" db:"device_uuid"`
	Status          string    `json:"status" db:"status"` // online, offline
	BatteryLevel    int       `json:"battery_level" db:"battery_level"`
	LastCheckIn     time.Time `json:"last_check_in" db:"last_check_in"`
	CertificateHash  string    `json:"certificate_hash" db:"certificate_hash"`
	X25519PublicKey  string    `json:"x25519_public_key,omitempty" db:"x25519_public_key"`
	Ed25519PublicKey string    `json:"ed25519_public_key,omitempty" db:"ed25519_public_key"`
	KeyFingerprint   string    `json:"key_fingerprint,omitempty" db:"key_fingerprint"`
	KeyVersion       int       `json:"key_version,omitempty" db:"key_version"`
	KeyCreatedAt     time.Time `json:"key_created_at,omitempty" db:"key_created_at"`
	EnrolledAt       time.Time `json:"enrolled_at" db:"enrolled_at"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// Administrator represents a system admin user
type Administrator struct {
	ID           uuid.UUID `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Email        string    `json:"email" db:"email"`
	Role         string    `json:"role" db:"role"` // super_admin, admin, operator
	Permissions  []string  `json:"permissions" db:"permissions"`
	IsActive     bool      `json:"is_active" db:"is_active"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// AuditLog represents an immutable audit trail entry
type AuditLog struct {
	ID             uuid.UUID `json:"id" db:"id"`
	TransactionID   uuid.UUID `json:"transaction_id" db:"transaction_id"`
	AdministratorID uuid.UUID `json:"administrator_id" db:"administrator_id"`
	AdministratorName string `json:"administrator_name" db:"administrator_name"`
	DeviceID       uuid.UUID `json:"device_id" db:"device_id"`
	DeviceName     string    `json:"device_name" db:"device_name"`
	CommandType    string    `json:"command_type" db:"command_type"`
	CommandPayload string    `json:"command_payload" db:"command_payload"`
	ResponseStatus string    `json:"response_status" db:"response_status"` // pending, success, failed, timeout
	ResponseData   []byte    `json:"response_data,omitempty" db:"response_data"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
	Timestamp      time.Time `json:"timestamp" db:"timestamp"`
}

// PendingCommand represents a queued command for offline devices
type PendingCommand struct {
	ID          uuid.UUID `json:"id" db:"id"`
	DeviceID    uuid.UUID `json:"device_id" db:"device_id"`
	CommandType string    `json:"command_type" db:"command_type"`
	Payload     string    `json:"payload" db:"payload"`
	Priority    int       `json:"priority" db:"priority"`
	Status      string    `json:"status" db:"status"` // queued, dispatched, completed, failed
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	ExpiresAt   time.Time `json:"expires_at" db:"expires_at"`
}

// MediaFile represents stored media from devices
type MediaFile struct {
	ID            uuid.UUID `json:"id" db:"id"`
	AuditLogID    uuid.UUID `json:"audit_log_id" db:"audit_log_id"`
	DeviceID      uuid.UUID `json:"device_id" db:"device_id"`
	FileName      string    `json:"file_name" db:"file_name"`
	FileType      string    `json:"file_type" db:"file_type"` // image, audio, video, document
	FileSize      int64     `json:"file_size" db:"file_size"`
	EncryptedData []byte    `json:"-" db:"encrypted_data"`
	Checksum      string    `json:"checksum" db:"checksum"`
	Source        string    `json:"source,omitempty" db:"source"`
	Camera        string    `json:"camera,omitempty" db:"camera"`
	MimeType      string    `json:"mime_type,omitempty" db:"mime_type"`
	Latitude      *float64  `json:"latitude,omitempty" db:"latitude"`
	Longitude     *float64  `json:"longitude,omitempty" db:"longitude"`
	DataEntryDate time.Time `json:"data_entry_date" db:"data_entry_date"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
}

// Session represents an admin web session
type Session struct {
	ID             uuid.UUID `json:"id" db:"id"`
	AdministratorID uuid.UUID `json:"administrator_id" db:"administrator_id"`
	RefreshToken   string    `json:"refresh_token,omitempty" db:"refresh_token"`
	IPAddress      string    `json:"ip_address" db:"ip_address"`
	UserAgent      string    `json:"user_agent" db:"user_agent"`
	ExpiresAt      time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

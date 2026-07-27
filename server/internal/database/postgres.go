package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

// PostgresDB wraps the database connection
type PostgresDB struct {
	db *sql.DB
}

// NewPostgresDB creates a new PostgreSQL database connection
func NewPostgresDB(connString string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresDB{db: db}, nil
}

// Close closes the database connection
func (p *PostgresDB) Close() error {
	return p.db.Close()
}

// InitSchema creates the database tables.
// Statements run in separate Exec calls so an earlier migration (e.g. CKX1 key
// columns) still commits if a later step fails.
func (p *PostgresDB) InitSchema(ctx context.Context) error {
	steps := []string{
		`CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`,

		`CREATE TABLE IF NOT EXISTS administrators (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			username VARCHAR(50) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			role VARCHAR(20) NOT NULL DEFAULT 'admin',
			permissions TEXT[],
			is_active BOOLEAN DEFAULT true,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS devices (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			friendly_name VARCHAR(100) NOT NULL,
			owner VARCHAR(100),
			os_version VARCHAR(120),
			hardware_model VARCHAR(100),
			device_uuid VARCHAR(255) UNIQUE NOT NULL,
			status VARCHAR(20) DEFAULT 'offline',
			battery_level INTEGER DEFAULT 0,
			last_check_in TIMESTAMP WITH TIME ZONE,
			certificate_hash VARCHAR(512),
			x25519_public_key TEXT,
			ed25519_public_key TEXT,
			key_fingerprint TEXT,
			key_version INTEGER NOT NULL DEFAULT 1,
			key_created_at TIMESTAMP WITH TIME ZONE,
			enrolled_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,

		// CKX1 device-key columns (existing DBs created before CKX1)
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS x25519_public_key TEXT`,
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS ed25519_public_key TEXT`,
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS key_fingerprint TEXT`,
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS key_version INTEGER NOT NULL DEFAULT 1`,
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS key_created_at TIMESTAMP WITH TIME ZONE`,
		`ALTER TABLE devices ALTER COLUMN os_version TYPE VARCHAR(120)`,

		`CREATE TABLE IF NOT EXISTS sessions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			administrator_id UUID REFERENCES administrators(id) ON DELETE CASCADE,
			refresh_token VARCHAR(512),
			ip_address VARCHAR(45),
			user_agent TEXT,
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			transaction_id UUID NOT NULL,
			administrator_id UUID,
			administrator_name VARCHAR(100),
			device_id UUID,
			device_name VARCHAR(100),
			command_type VARCHAR(50) NOT NULL,
			command_payload TEXT,
			response_status VARCHAR(20) DEFAULT 'pending',
			response_data BYTEA,
			ip_address VARCHAR(45),
			user_agent TEXT,
			timestamp TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS pending_commands (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
			command_type VARCHAR(50) NOT NULL,
			payload TEXT,
			priority INTEGER DEFAULT 0,
			status VARCHAR(20) DEFAULT 'queued',
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE
		)`,

		`CREATE TABLE IF NOT EXISTS media_files (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			audit_log_id UUID REFERENCES audit_logs(id),
			device_id UUID REFERENCES devices(id) ON DELETE CASCADE,
			file_name VARCHAR(255) NOT NULL,
			file_type VARCHAR(50) NOT NULL,
			file_size BIGINT,
			encrypted_data BYTEA,
			checksum VARCHAR(64),
			created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS device_contacts (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			native_id VARCHAR(255) NOT NULL DEFAULT '',
			display_name TEXT NOT NULL DEFAULT '',
			number TEXT NOT NULL DEFAULT '',
			number_fp VARCHAR(64) NOT NULL DEFAULT '',
			data_entry_date TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE (device_id, native_id, number_fp)
		)`,

		`CREATE TABLE IF NOT EXISTS device_sms (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			native_id VARCHAR(255) NOT NULL,
			is_read BOOLEAN DEFAULT false,
			address TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			person TEXT NOT NULL DEFAULT '',
			message_date TIMESTAMP WITH TIME ZONE,
			message_type VARCHAR(50) NOT NULL DEFAULT '',
			data_entry_date TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE (device_id, native_id)
		)`,

		`CREATE TABLE IF NOT EXISTS device_call_logs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			call_id VARCHAR(255) NOT NULL,
			number TEXT NOT NULL DEFAULT '',
			name_call TEXT NOT NULL DEFAULT '',
			duration INTEGER NOT NULL DEFAULT 0,
			type_call VARCHAR(50) NOT NULL DEFAULT '',
			date_call TIMESTAMP WITH TIME ZONE,
			id_contacts VARCHAR(255) NOT NULL DEFAULT '',
			data_entry_date TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE (device_id, call_id)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status)`,

		`CREATE TABLE IF NOT EXISTS device_locations (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			latitude DOUBLE PRECISION NOT NULL,
			longitude DOUBLE PRECISION NOT NULL,
			altitude DOUBLE PRECISION,
			accuracy DOUBLE PRECISION,
			provider VARCHAR(50) NOT NULL DEFAULT '',
			stale BOOLEAN DEFAULT false,
			fix_time TIMESTAMP WITH TIME ZONE,
			data_entry_date TIMESTAMP WITH TIME ZONE DEFAULT NOW()
		)`,

		`CREATE TABLE IF NOT EXISTS device_file_entries (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			device_id UUID NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			path_fp VARCHAR(64) NOT NULL DEFAULT '',
			name TEXT NOT NULL DEFAULT '',
			is_directory BOOLEAN DEFAULT false,
			size BIGINT DEFAULT 0,
			permissions VARCHAR(32) NOT NULL DEFAULT '',
			modified_time TIMESTAMP WITH TIME ZONE,
			data_entry_date TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
			UNIQUE (device_id, path_fp)
		)`,

		`ALTER TABLE media_files ADD COLUMN IF NOT EXISTS source VARCHAR(50) DEFAULT ''`,
		`ALTER TABLE media_files ADD COLUMN IF NOT EXISTS camera VARCHAR(20) DEFAULT ''`,
		`ALTER TABLE media_files ADD COLUMN IF NOT EXISTS mime_type VARCHAR(100) DEFAULT ''`,
		`ALTER TABLE media_files ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION`,
		`ALTER TABLE media_files ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION`,
		`ALTER TABLE media_files ADD COLUMN IF NOT EXISTS data_entry_date TIMESTAMP WITH TIME ZONE DEFAULT NOW()`,
		`ALTER TABLE device_locations ADD COLUMN IF NOT EXISTS enc_blob TEXT`,
		`ALTER TABLE device_contacts ADD COLUMN IF NOT EXISTS number_fp VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE device_file_entries ADD COLUMN IF NOT EXISTS path_fp VARCHAR(64) NOT NULL DEFAULT ''`,
		`ALTER TABLE device_contacts ALTER COLUMN display_name TYPE TEXT`,
		`ALTER TABLE device_contacts ALTER COLUMN number TYPE TEXT`,
		`ALTER TABLE device_sms ALTER COLUMN address TYPE TEXT`,
		`ALTER TABLE device_sms ALTER COLUMN name TYPE TEXT`,
		`ALTER TABLE device_sms ALTER COLUMN person TYPE TEXT`,
		`ALTER TABLE device_call_logs ALTER COLUMN number TYPE TEXT`,
		`ALTER TABLE device_call_logs ALTER COLUMN name_call TYPE TEXT`,
		`ALTER TABLE device_file_entries ALTER COLUMN name TYPE TEXT`,

		// Legacy rows may share empty fingerprints after ADD COLUMN DEFAULT ''.
		// Empty path_fp collides per device; drop those (inventory re-syncs). Deduplicate the rest.
		`DELETE FROM device_file_entries WHERE COALESCE(path_fp, '') = ''`,
		`DELETE FROM device_file_entries a USING device_file_entries b
			WHERE a.ctid < b.ctid AND a.device_id = b.device_id AND a.path_fp = b.path_fp`,
		`DELETE FROM device_contacts a USING device_contacts b
			WHERE a.ctid < b.ctid
			  AND a.device_id = b.device_id
			  AND a.native_id = b.native_id
			  AND a.number_fp = b.number_fp`,

		`ALTER TABLE device_contacts DROP CONSTRAINT IF EXISTS device_contacts_device_id_native_id_number_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_device_contacts_identity
			ON device_contacts(device_id, native_id, number_fp)`,
		`ALTER TABLE device_file_entries DROP CONSTRAINT IF EXISTS device_file_entries_device_id_path_key`,
		`ALTER TABLE device_file_entries DROP CONSTRAINT IF EXISTS device_file_entries_device_id_path_fp_key`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_device_file_entries_path_fp
			ON device_file_entries(device_id, path_fp)`,

		`CREATE INDEX IF NOT EXISTS idx_device_contacts_device_id ON device_contacts(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_sms_device_id ON device_sms(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_sms_address ON device_sms(device_id, address)`,
		`CREATE INDEX IF NOT EXISTS idx_device_call_logs_device_id ON device_call_logs(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_locations_device_id ON device_locations(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_device_locations_fix_time ON device_locations(device_id, fix_time DESC NULLS LAST)`,
		`CREATE INDEX IF NOT EXISTS idx_device_file_entries_device_id ON device_file_entries(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_media_files_device_type ON media_files(device_id, file_type)`,
		`CREATE INDEX IF NOT EXISTS idx_devices_last_check_in ON devices(last_check_in)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_device_id ON audit_logs(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_administrator_id ON audit_logs(administrator_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_commands_device_id ON pending_commands(device_id)`,
		`CREATE INDEX IF NOT EXISTS idx_pending_commands_status ON pending_commands(status)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_administrator_id ON sessions(administrator_id)`,

		`CREATE OR REPLACE FUNCTION prevent_audit_modification()
		RETURNS TRIGGER AS $$
		BEGIN
			RAISE EXCEPTION 'Audit logs cannot be modified or deleted';
		END;
		$$ LANGUAGE plpgsql`,

		`DROP TRIGGER IF EXISTS prevent_audit_update ON audit_logs`,
		`CREATE TRIGGER prevent_audit_update
			BEFORE UPDATE ON audit_logs
			FOR EACH ROW
			EXECUTE FUNCTION prevent_audit_modification()`,

		`DROP TRIGGER IF EXISTS prevent_audit_delete ON audit_logs`,
		`CREATE TRIGGER prevent_audit_delete
			BEFORE DELETE ON audit_logs
			FOR EACH ROW
			EXECUTE FUNCTION prevent_audit_modification()`,
	}

	for i, step := range steps {
		if _, err := p.db.ExecContext(ctx, step); err != nil {
			return fmt.Errorf("schema step %d: %w", i+1, err)
		}
	}
	return nil
}

// Admin operations

// CreateAdministrator creates a new administrator
func (p *PostgresDB) CreateAdministrator(ctx context.Context, admin *models.Administrator) error {
	query := `
		INSERT INTO administrators (id, username, password_hash, email, role, permissions, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := p.db.ExecContext(ctx, query,
		admin.ID, admin.Username, admin.PasswordHash, admin.Email,
		admin.Role, pq.Array(admin.Permissions), admin.IsActive)
	return err
}

// GetAdministratorByUsername retrieves an administrator by username
func (p *PostgresDB) GetAdministratorByUsername(ctx context.Context, username string) (*models.Administrator, error) {
	query := `
		SELECT id, username, password_hash, email, role, permissions, is_active, created_at, updated_at
		FROM administrators WHERE username = $1 AND is_active = true
	`
	admin := &models.Administrator{}
	var permissions pq.StringArray
	err := p.db.QueryRowContext(ctx, query, username).Scan(
		&admin.ID, &admin.Username, &admin.PasswordHash, &admin.Email,
		&admin.Role, &permissions, &admin.IsActive, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		return nil, err
	}
	admin.Permissions = []string(permissions)
	return admin, nil
}

// GetAdministratorByID retrieves an administrator by ID
func (p *PostgresDB) GetAdministratorByID(ctx context.Context, id uuid.UUID) (*models.Administrator, error) {
	query := `
		SELECT id, username, password_hash, email, role, permissions, is_active, created_at, updated_at
		FROM administrators WHERE id = $1
	`
	admin := &models.Administrator{}
	var permissions pq.StringArray
	err := p.db.QueryRowContext(ctx, query, id).Scan(
		&admin.ID, &admin.Username, &admin.PasswordHash, &admin.Email,
		&admin.Role, &permissions, &admin.IsActive, &admin.CreatedAt, &admin.UpdatedAt)
	if err != nil {
		return nil, err
	}
	admin.Permissions = []string(permissions)
	return admin, nil
}

// Device operations

func scanDevice(scanner interface {
	Scan(dest ...interface{}) error
}) (*models.Device, error) {
	var d models.Device
	var owner, osVersion, hardwareModel, certHash sql.NullString
	var x25519, ed25519, keyFP sql.NullString
	var keyVersion sql.NullInt64
	var lastCheckIn, keyCreated sql.NullTime
	err := scanner.Scan(
		&d.ID, &d.FriendlyName, &owner, &osVersion, &hardwareModel,
		&d.DeviceUUID, &d.Status, &d.BatteryLevel, &lastCheckIn,
		&certHash, &x25519, &ed25519, &keyFP, &keyVersion, &keyCreated,
		&d.EnrolledAt, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if owner.Valid {
		d.Owner = owner.String
	}
	if osVersion.Valid {
		d.OSVersion = osVersion.String
	}
	if hardwareModel.Valid {
		d.HardwareModel = hardwareModel.String
	}
	if certHash.Valid {
		d.CertificateHash = certHash.String
	}
	if x25519.Valid {
		d.X25519PublicKey = x25519.String
	}
	if ed25519.Valid {
		d.Ed25519PublicKey = ed25519.String
	}
	if keyFP.Valid {
		d.KeyFingerprint = keyFP.String
	}
	if keyVersion.Valid {
		d.KeyVersion = int(keyVersion.Int64)
	}
	if keyCreated.Valid {
		d.KeyCreatedAt = keyCreated.Time
	}
	if lastCheckIn.Valid {
		d.LastCheckIn = lastCheckIn.Time
	}
	return &d, nil
}

const deviceSelectCols = `
		id, friendly_name, owner, os_version, hardware_model, device_uuid,
		status, battery_level, last_check_in, certificate_hash,
		x25519_public_key, ed25519_public_key, key_fingerprint, key_version, key_created_at,
		enrolled_at, created_at, updated_at`

// CreateDevice creates a new device enrollment
func (p *PostgresDB) CreateDevice(ctx context.Context, device *models.Device) error {
	if device.KeyVersion == 0 {
		device.KeyVersion = 1
	}
	query := `
		INSERT INTO devices (id, friendly_name, owner, os_version, hardware_model, device_uuid,
			status, battery_level, certificate_hash, x25519_public_key, ed25519_public_key,
			key_fingerprint, key_version, key_created_at, enrolled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	keyCreated := device.KeyCreatedAt
	if keyCreated.IsZero() {
		keyCreated = time.Now()
	}
	_, err := p.db.ExecContext(ctx, query,
		device.ID, device.FriendlyName, device.Owner, device.OSVersion, device.HardwareModel,
		device.DeviceUUID, device.Status, device.BatteryLevel, device.CertificateHash,
		device.X25519PublicKey, device.Ed25519PublicKey, device.KeyFingerprint,
		device.KeyVersion, keyCreated, device.EnrolledAt)
	return err
}

// GetDeviceByID retrieves a device by UUID
func (p *PostgresDB) GetDeviceByID(ctx context.Context, id uuid.UUID) (*models.Device, error) {
	query := `SELECT ` + deviceSelectCols + ` FROM devices WHERE id = $1`
	return scanDevice(p.db.QueryRowContext(ctx, query, id))
}

// GetDeviceByUUID retrieves a device by device UUID
func (p *PostgresDB) GetDeviceByUUID(ctx context.Context, deviceUUID string) (*models.Device, error) {
	query := `SELECT ` + deviceSelectCols + ` FROM devices WHERE device_uuid = $1`
	return scanDevice(p.db.QueryRowContext(ctx, query, deviceUUID))
}

// GetAllDevices retrieves all devices
func (p *PostgresDB) GetAllDevices(ctx context.Context) ([]models.Device, error) {
	query := `SELECT ` + deviceSelectCols + ` FROM devices ORDER BY last_check_in DESC NULLS LAST, created_at DESC
	`
	rows, err := p.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	for rows.Next() {
		d, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, *d)
	}
	return devices, rows.Err()
}

// UpdateDeviceStatus updates device status and battery level
func (p *PostgresDB) UpdateDeviceStatus(ctx context.Context, id uuid.UUID, status string, batteryLevel int) error {
	query := `
		UPDATE devices SET status = $2, battery_level = $3, last_check_in = NOW(), updated_at = NOW()
		WHERE id = $1
	`
	_, err := p.db.ExecContext(ctx, query, id, status, batteryLevel)
	return err
}

// UpdateDeviceCKX1Keys stores enrolled device public identity keys.
func (p *PostgresDB) UpdateDeviceCKX1Keys(ctx context.Context, id uuid.UUID, x25519, ed25519, fingerprint string) error {
	query := `
		UPDATE devices SET
			x25519_public_key = COALESCE(NULLIF($2, ''), x25519_public_key),
			ed25519_public_key = COALESCE(NULLIF($3, ''), ed25519_public_key),
			key_fingerprint = COALESCE(NULLIF($4, ''), key_fingerprint),
			certificate_hash = COALESCE(NULLIF($4, ''), certificate_hash),
			key_created_at = COALESCE(key_created_at, NOW()),
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := p.db.ExecContext(ctx, query, id, x25519, ed25519, fingerprint)
	return err
}

// UpdateDeviceMetadata refreshes operator-visible device fields on re-enroll / reconnect.
func (p *PostgresDB) UpdateDeviceMetadata(ctx context.Context, id uuid.UUID, friendlyName, osVersion, hardwareModel string) error {
	query := `
		UPDATE devices SET
			friendly_name = COALESCE(NULLIF($2, ''), friendly_name),
			os_version = COALESCE(NULLIF($3, ''), os_version),
			hardware_model = COALESCE(NULLIF($4, ''), hardware_model),
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := p.db.ExecContext(ctx, query, id, friendlyName, osVersion, hardwareModel)
	return err
}

// DeleteDevice removes a device from the registry
func (p *PostgresDB) DeleteDevice(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM devices WHERE id = $1`
	_, err := p.db.ExecContext(ctx, query, id)
	return err
}

// Session operations

// CreateSession creates a new admin session
func (p *PostgresDB) CreateSession(ctx context.Context, session *models.Session) error {
	query := `
		INSERT INTO sessions (id, administrator_id, refresh_token, ip_address, user_agent, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := p.db.ExecContext(ctx, query,
		session.ID, session.AdministratorID, session.RefreshToken,
		session.IPAddress, session.UserAgent, session.ExpiresAt)
	return err
}

// GetSessionByRefreshToken retrieves a session by refresh token
func (p *PostgresDB) GetSessionByRefreshToken(ctx context.Context, token string) (*models.Session, error) {
	query := `
		SELECT id, administrator_id, refresh_token, ip_address, user_agent, expires_at, created_at
		FROM sessions WHERE refresh_token = $1 AND expires_at > NOW()
	`
	session := &models.Session{}
	err := p.db.QueryRowContext(ctx, query, token).Scan(
		&session.ID, &session.AdministratorID, &session.RefreshToken,
		&session.IPAddress, &session.UserAgent, &session.ExpiresAt, &session.CreatedAt)
	return session, err
}

// DeleteSession removes a session
func (p *PostgresDB) DeleteSession(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := p.db.ExecContext(ctx, query, id)
	return err
}

// DeleteSessionByRefreshToken removes a session by refresh token
func (p *PostgresDB) DeleteSessionByRefreshToken(ctx context.Context, token string) error {
	query := `DELETE FROM sessions WHERE refresh_token = $1`
	_, err := p.db.ExecContext(ctx, query, token)
	return err
}

// Audit log operations

// CreateAuditLog creates a new audit log entry
func (p *PostgresDB) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (id, transaction_id, administrator_id, administrator_name,
			device_id, device_name, command_type, command_payload, response_status, 
			response_data, ip_address, user_agent, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := p.db.ExecContext(ctx, query,
		log.ID, log.TransactionID, log.AdministratorID, log.AdministratorName,
		log.DeviceID, log.DeviceName, log.CommandType, log.CommandPayload, log.ResponseStatus,
		log.ResponseData, log.IPAddress, log.UserAgent, log.Timestamp)
	return err
}

// UpdateAuditLogResponse updates the response data and status
func (p *PostgresDB) UpdateAuditLogResponse(ctx context.Context, transactionID uuid.UUID, status string, data []byte) error {
	// Note: This does NOT modify the original audit log entry, only appends response data
	// The trigger prevents UPDATE of core audit fields
	query := `
		UPDATE audit_logs SET response_status = $2, response_data = $3
		WHERE transaction_id = $1
	`
	_, err := p.db.ExecContext(ctx, query, transactionID, status, data)
	return err
}

// GetAuditLogByTransactionID returns the audit log row for a command transaction.
func (p *PostgresDB) GetAuditLogByTransactionID(ctx context.Context, transactionID uuid.UUID) (*models.AuditLog, error) {
	query := `
		SELECT id, transaction_id, administrator_id, administrator_name, device_id, device_name,
			command_type, command_payload, response_status, response_data, ip_address, user_agent, timestamp
		FROM audit_logs WHERE transaction_id = $1
	`
	logEntry := &models.AuditLog{}
	err := p.db.QueryRowContext(ctx, query, transactionID).Scan(
		&logEntry.ID, &logEntry.TransactionID, &logEntry.AdministratorID, &logEntry.AdministratorName,
		&logEntry.DeviceID, &logEntry.DeviceName, &logEntry.CommandType, &logEntry.CommandPayload,
		&logEntry.ResponseStatus, &logEntry.ResponseData, &logEntry.IPAddress, &logEntry.UserAgent, &logEntry.Timestamp)
	if err != nil {
		return nil, err
	}
	return logEntry, nil
}

// SearchAuditLogs searches audit logs with filters
func (p *PostgresDB) SearchAuditLogs(ctx context.Context, req models.SearchRequest) ([]models.AuditLog, int64, error) {
	baseQuery := `FROM audit_logs WHERE 1=1`
	args := []interface{}{}
	argCount := 0

	if req.Query != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND (command_payload ILIKE $%d OR device_name ILIKE $%d OR administrator_name ILIKE $%d)", argCount, argCount, argCount)
		args = append(args, "%"+req.Query+"%")
	}
	if req.DeviceID != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND device_id = $%d", argCount)
		args = append(args, req.DeviceID)
	}
	if req.AdminID != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND administrator_id = $%d", argCount)
		args = append(args, req.AdminID)
	}
	if req.CommandType != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND command_type = $%d", argCount)
		args = append(args, req.CommandType)
	}
	if req.Status != "" {
		argCount++
		baseQuery += fmt.Sprintf(" AND response_status = $%d", argCount)
		args = append(args, req.Status)
	}
	if !req.StartDate.IsZero() {
		argCount++
		baseQuery += fmt.Sprintf(" AND timestamp >= $%d", argCount)
		args = append(args, req.StartDate)
	}
	if !req.EndDate.IsZero() {
		argCount++
		baseQuery += fmt.Sprintf(" AND timestamp <= $%d", argCount)
		args = append(args, req.EndDate)
	}

	// Count total
	countQuery := "SELECT COUNT(*) " + baseQuery
	var total int64
	if err := p.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch page
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	offset := (req.Page - 1) * req.PageSize

	selectQuery := fmt.Sprintf(`
		SELECT id, transaction_id, administrator_id, administrator_name, device_id, device_name,
			command_type, command_payload, response_status, ip_address, user_agent, timestamp
		%s ORDER BY timestamp DESC LIMIT $%d OFFSET $%d
	`, baseQuery, argCount+1, argCount+2)
	args = append(args, req.PageSize, offset)

	rows, err := p.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []models.AuditLog
	for rows.Next() {
		var log models.AuditLog
		err := rows.Scan(
			&log.ID, &log.TransactionID, &log.AdministratorID, &log.AdministratorName,
			&log.DeviceID, &log.DeviceName, &log.CommandType, &log.CommandPayload,
			&log.ResponseStatus, &log.IPAddress, &log.UserAgent, &log.Timestamp)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, log)
	}
	return logs, total, rows.Err()
}

// Pending command operations

// CreatePendingCommand queues a command for an offline device
func (p *PostgresDB) CreatePendingCommand(ctx context.Context, cmd *models.PendingCommand) error {
	query := `
		INSERT INTO pending_commands (id, device_id, command_type, payload, priority, status, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := p.db.ExecContext(ctx, query,
		cmd.ID, cmd.DeviceID, cmd.CommandType, cmd.Payload, cmd.Priority, cmd.Status, cmd.ExpiresAt)
	return err
}

// GetPendingCommandsForDevice retrieves queued commands for a device
func (p *PostgresDB) GetPendingCommandsForDevice(ctx context.Context, deviceID uuid.UUID) ([]models.PendingCommand, error) {
	query := `
		SELECT id, device_id, command_type, payload, priority, status, created_at, expires_at
		FROM pending_commands WHERE device_id = $1 AND status = 'queued' AND expires_at > NOW()
		ORDER BY priority DESC, created_at ASC
	`
	rows, err := p.db.QueryContext(ctx, query, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commands []models.PendingCommand
	for rows.Next() {
		var cmd models.PendingCommand
		err := rows.Scan(&cmd.ID, &cmd.DeviceID, &cmd.CommandType, &cmd.Payload,
			&cmd.Priority, &cmd.Status, &cmd.CreatedAt, &cmd.ExpiresAt)
		if err != nil {
			return nil, err
		}
		commands = append(commands, cmd)
	}
	return commands, rows.Err()
}

// UpdatePendingCommandStatus updates the status of a pending command
func (p *PostgresDB) UpdatePendingCommandStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE pending_commands SET status = $2 WHERE id = $1`
	_, err := p.db.ExecContext(ctx, query, id, status)
	return err
}

// Media file operations

// CreateMediaFile stores a media file
func (p *PostgresDB) CreateMediaFile(ctx context.Context, file *models.MediaFile) error {
	if file.DataEntryDate.IsZero() {
		file.DataEntryDate = time.Now().UTC()
	}
	var auditID interface{}
	if file.AuditLogID != uuid.Nil {
		auditID = file.AuditLogID
	}
	query := `
		INSERT INTO media_files (
			id, audit_log_id, device_id, file_name, file_type, file_size, encrypted_data, checksum,
			source, camera, mime_type, latitude, longitude, data_entry_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err := p.db.ExecContext(ctx, query,
		file.ID, auditID, file.DeviceID, file.FileName, file.FileType,
		file.FileSize, file.EncryptedData, file.Checksum,
		file.Source, file.Camera, file.MimeType, file.Latitude, file.Longitude, file.DataEntryDate)
	return err
}

// GetMediaFileByID retrieves a media file
func (p *PostgresDB) GetMediaFileByID(ctx context.Context, id uuid.UUID) (*models.MediaFile, error) {
	query := `
		SELECT id, COALESCE(audit_log_id, '00000000-0000-0000-0000-000000000000'), device_id,
			file_name, file_type, file_size, encrypted_data, checksum,
			COALESCE(source, ''), COALESCE(camera, ''), COALESCE(mime_type, ''),
			latitude, longitude, COALESCE(data_entry_date, created_at), created_at
		FROM media_files WHERE id = $1
	`
	file := &models.MediaFile{}
	var lat, lng sql.NullFloat64
	err := p.db.QueryRowContext(ctx, query, id).Scan(
		&file.ID, &file.AuditLogID, &file.DeviceID, &file.FileName, &file.FileType,
		&file.FileSize, &file.EncryptedData, &file.Checksum,
		&file.Source, &file.Camera, &file.MimeType, &lat, &lng, &file.DataEntryDate, &file.CreatedAt)
	if err != nil {
		return nil, err
	}
	if lat.Valid {
		v := lat.Float64
		file.Latitude = &v
	}
	if lng.Valid {
		v := lng.Float64
		file.Longitude = &v
	}
	return file, nil
}

// GetMediaFilesByDevice retrieves media files for a device
func (p *PostgresDB) GetMediaFilesByDevice(ctx context.Context, deviceID uuid.UUID, limit int) ([]models.MediaFile, error) {
	return p.ListDeviceMedia(ctx, deviceID, "", limit)
}

// Statistics

// GetDashboardStats retrieves dashboard statistics
func (p *PostgresDB) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	stats := &models.DashboardStats{TopCommands: make(map[string]int)}

	// Device counts
	p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices`).Scan(&stats.TotalDevices)
	p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM devices WHERE status = 'online'`).Scan(&stats.OnlineDevices)
	stats.OfflineDevices = stats.TotalDevices - stats.OnlineDevices

	// Command counts
	p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&stats.TotalCommands)
	
	var todayStart time.Time = time.Now().Truncate(24 * time.Hour)
	p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE timestamp >= $1`, todayStart).Scan(&stats.CommandsToday)

	weekStart := todayStart.AddDate(0, 0, -7)
	p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE timestamp >= $1`, weekStart).Scan(&stats.CommandsThisWeek)

	// Success rate
	var successCount, totalCount int64
	p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs WHERE response_status = 'success'`).Scan(&successCount)
	p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&totalCount)
	if totalCount > 0 {
		stats.SuccessRate = float64(successCount) / float64(totalCount) * 100
	}

	// Top commands
	rows, err := p.db.QueryContext(ctx, `
		SELECT command_type, COUNT(*) as cnt FROM audit_logs 
		GROUP BY command_type ORDER BY cnt DESC LIMIT 10
	`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cmdType string
			var cnt int
			rows.Scan(&cmdType, &cnt)
			stats.TopCommands[cmdType] = cnt
		}
	}

	// Active admins
	p.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT administrator_id) FROM sessions WHERE expires_at > NOW()
	`).Scan(&stats.ActiveAdmins)

	return stats, nil
}

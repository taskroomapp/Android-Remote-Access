package database

import (
	"context"
	"database/sql"
	"path"
	"strconv"
	"strings"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
)

func asFloat64(v interface{}) (float64, bool) {
	if v == nil {
		return 0, false
	}
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func asFloatPtr(v interface{}) *float64 {
	f, ok := asFloat64(v)
	if !ok {
		return nil
	}
	return &f
}

// UpsertDeviceLocations stores GPS points.
func (p *PostgresDB) UpsertDeviceLocations(ctx context.Context, deviceID uuid.UUID, raw []map[string]interface{}) (int, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	query := `
		INSERT INTO device_locations (
			id, device_id, latitude, longitude, altitude, accuracy, provider, stale, fix_time, data_entry_date, enc_blob
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), $10)
	`
	saved := 0
	for _, item := range raw {
		encBlob := asString(item["enc_blob"])
		lat, okLat := asFloat64(item["latitude"])
		if !okLat {
			lat, okLat = asFloat64(item["lat"])
		}
		lng, okLng := asFloat64(item["longitude"])
		if !okLng {
			lng, okLng = asFloat64(item["lng"])
		}
		if encBlob == "" && (!okLat || !okLng) {
			continue
		}
		if encBlob != "" {
			// Coordinates live inside enc_blob; keep numeric columns as placeholders.
			lat, lng = 0, 0
		}
		fixTime := parseFlexibleTime(item["fix_time"])
		if fixTime == nil {
			fixTime = parseFlexibleTime(item["timestamp"])
		}
		locID := uuid.New()
		if s := asString(item["id"]); s != "" {
			if parsed, err := uuid.Parse(s); err == nil {
				locID = parsed
			}
		}
		_, err := p.db.ExecContext(ctx, query,
			locID, deviceID, lat, lng,
			asFloatPtr(item["altitude"]), asFloatPtr(item["accuracy"]),
			asString(item["provider"]), asBool(item["stale"]), fixTime, encBlob,
		)
		if err != nil {
			return saved, err
		}
		saved++
	}
	return saved, nil
}

// ListDeviceLocations returns GPS history newest-first.
func (p *PostgresDB) ListDeviceLocations(ctx context.Context, deviceID uuid.UUID, limit int) ([]models.StoredLocation, error) {
	if limit <= 0 {
		limit = 5000
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, device_id, latitude, longitude, altitude, accuracy, provider, stale, fix_time, data_entry_date, COALESCE(enc_blob, '')
		FROM device_locations WHERE device_id = $1
		ORDER BY COALESCE(fix_time, data_entry_date) DESC
		LIMIT $2
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.StoredLocation
	for rows.Next() {
		var loc models.StoredLocation
		var alt, acc sql.NullFloat64
		var fixTime sql.NullTime
		if err := rows.Scan(
			&loc.ID, &loc.DeviceID, &loc.Latitude, &loc.Longitude, &alt, &acc,
			&loc.Provider, &loc.Stale, &fixTime, &loc.DataEntryDate, &loc.EncBlob,
		); err != nil {
			return nil, err
		}
		if alt.Valid {
			v := alt.Float64
			loc.Altitude = &v
		}
		if acc.Valid {
			v := acc.Float64
			loc.Accuracy = &v
		}
		if fixTime.Valid {
			t := fixTime.Time
			loc.FixTime = &t
		}
		out = append(out, loc)
	}
	return out, rows.Err()
}

// UpsertDeviceFileEntries stores file-tree metadata.
func (p *PostgresDB) UpsertDeviceFileEntries(ctx context.Context, deviceID uuid.UUID, raw []map[string]interface{}) (int, error) {
	if len(raw) == 0 {
		return 0, nil
	}
	query := `
		INSERT INTO device_file_entries (
			id, device_id, path, path_fp, name, is_directory, size, permissions, modified_time, data_entry_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (device_id, path_fp) DO UPDATE SET
			path = EXCLUDED.path,
			name = EXCLUDED.name,
			is_directory = EXCLUDED.is_directory,
			size = EXCLUDED.size,
			permissions = EXCLUDED.permissions,
			modified_time = EXCLUDED.modified_time,
			data_entry_date = NOW()
	`
	saved := 0
	for _, item := range raw {
		filePath := asString(item["path"])
		pathFP := asString(item["path_fp"])
		if filePath == "" || pathFP == "" {
			continue
		}
		name := asString(item["name"])
		if name == "" {
			name = path.Base(filePath)
		}
		mod := parseFlexibleTime(item["modified_time"])
		if mod == nil {
			mod = parseFlexibleTime(item["modifiedTime"])
		}
		_, err := p.db.ExecContext(ctx, query,
			uuid.New(), deviceID, filePath, pathFP, name,
			asBool(item["is_directory"]) || asBool(item["isDirectory"]),
			int64(asInt(item["size"])),
			asString(item["permissions"]),
			mod,
		)
		if err != nil {
			return saved, err
		}
		saved++
	}
	return saved, nil
}

// ListDeviceFileEntries returns file inventory rows.
func (p *PostgresDB) ListDeviceFileEntries(ctx context.Context, deviceID uuid.UUID, limit int) ([]models.StoredFileEntry, error) {
	if limit <= 0 {
		limit = 50000
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, device_id, path, COALESCE(path_fp, ''), name, is_directory, size, permissions, modified_time, data_entry_date
		FROM device_file_entries WHERE device_id = $1
		ORDER BY path ASC
		LIMIT $2
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.StoredFileEntry
	for rows.Next() {
		var e models.StoredFileEntry
		var mod sql.NullTime
		if err := rows.Scan(
			&e.ID, &e.DeviceID, &e.Path, &e.PathFP, &e.Name, &e.IsDirectory, &e.Size, &e.Permissions, &mod, &e.DataEntryDate,
		); err != nil {
			return nil, err
		}
		if mod.Valid {
			t := mod.Time
			e.ModifiedTime = &t
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListDeviceMedia lists media metadata (no blob) optionally filtered by file_type.
func (p *PostgresDB) ListDeviceMedia(ctx context.Context, deviceID uuid.UUID, fileType string, limit int) ([]models.MediaFile, error) {
	if limit <= 0 {
		limit = 500
	}
	var (
		rows *sql.Rows
		err  error
	)
	if fileType == "" || fileType == "all" {
		rows, err = p.db.QueryContext(ctx, `
			SELECT id, COALESCE(audit_log_id, '00000000-0000-0000-0000-000000000000'), device_id,
				file_name, file_type, file_size, checksum,
				COALESCE(source, ''), COALESCE(camera, ''), COALESCE(mime_type, ''),
				latitude, longitude, COALESCE(data_entry_date, created_at), created_at
			FROM media_files WHERE device_id = $1
			ORDER BY COALESCE(data_entry_date, created_at) DESC
			LIMIT $2
		`, deviceID, limit)
	} else {
		rows, err = p.db.QueryContext(ctx, `
			SELECT id, COALESCE(audit_log_id, '00000000-0000-0000-0000-000000000000'), device_id,
				file_name, file_type, file_size, checksum,
				COALESCE(source, ''), COALESCE(camera, ''), COALESCE(mime_type, ''),
				latitude, longitude, COALESCE(data_entry_date, created_at), created_at
			FROM media_files WHERE device_id = $1 AND file_type = $2
			ORDER BY COALESCE(data_entry_date, created_at) DESC
			LIMIT $3
		`, deviceID, fileType, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []models.MediaFile
	for rows.Next() {
		var f models.MediaFile
		var lat, lng sql.NullFloat64
		if err := rows.Scan(
			&f.ID, &f.AuditLogID, &f.DeviceID, &f.FileName, &f.FileType, &f.FileSize, &f.Checksum,
			&f.Source, &f.Camera, &f.MimeType, &lat, &lng, &f.DataEntryDate, &f.CreatedAt,
		); err != nil {
			return nil, err
		}
		if lat.Valid {
			v := lat.Float64
			f.Latitude = &v
		}
		if lng.Valid {
			v := lng.Float64
			f.Longitude = &v
		}
		files = append(files, f)
	}
	return files, rows.Err()
}

// CountDeviceArtifacts returns row counts for status labels.
func (p *PostgresDB) CountDeviceArtifacts(ctx context.Context, deviceID uuid.UUID) (locations, files, media int, err error) {
	err = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_locations WHERE device_id = $1`, deviceID).Scan(&locations)
	if err != nil {
		return
	}
	err = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_file_entries WHERE device_id = $1`, deviceID).Scan(&files)
	if err != nil {
		return
	}
	err = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_files WHERE device_id = $1`, deviceID).Scan(&media)
	return
}

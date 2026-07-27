package api

import (
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"strings"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/enterprise/android-remote-access/server/internal/security"
	"github.com/google/uuid"
)

func (s *Server) openString(sealed, aad string) string {
	if s.encryptor == nil || sealed == "" {
		return sealed
	}
	plain, err := s.encryptor.OpenString(sealed, aad)
	if err != nil {
		log.Printf("security: openString failed: %v", err)
		return ""
	}
	return plain
}

func (s *Server) openRecordBound(sealed, recordAAD string) string {
	return s.openString(sealed, recordAAD)
}

func (s *Server) openContacts(items []models.StoredContact) {
	for i := range items {
		aad := security.AADContactRecord(items[i].DeviceID.String(), items[i].NativeID, items[i].NumberFP)
		items[i].DisplayName = s.openRecordBound(items[i].DisplayName, aad)
		items[i].Number = s.openRecordBound(items[i].Number, aad)
	}
}

func (s *Server) openSMS(items []models.StoredSMS) {
	for i := range items {
		aad := security.AADSMSRecord(items[i].DeviceID.String(), items[i].NativeID)
		open := func(v string) string {
			return s.openRecordBound(v, aad)
		}
		items[i].Address = open(items[i].Address)
		items[i].Message = open(items[i].Message)
		items[i].Name = open(items[i].Name)
		items[i].Person = open(items[i].Person)
	}
}

func (s *Server) openCalls(items []models.StoredCallLog) {
	for i := range items {
		aad := security.AADCallRecord(items[i].DeviceID.String(), items[i].CallID)
		open := func(v string) string {
			return s.openRecordBound(v, aad)
		}
		items[i].Number = open(items[i].Number)
		items[i].NameCall = open(items[i].NameCall)
	}
}

func (s *Server) openFileEntries(items []models.StoredFileEntry) {
	for i := range items {
		aad := security.AADFilePathRecord(items[i].DeviceID.String(), items[i].PathFP)
		open := func(v string) string {
			return s.openRecordBound(v, aad)
		}
		items[i].Path = open(items[i].Path)
		items[i].Name = open(items[i].Name)
	}
}

func (s *Server) sealContactMaps(deviceID uuid.UUID, items []map[string]interface{}) error {
	if s.encryptor == nil {
		return errors.New("encryptor required: refusing to store plaintext")
	}
	for _, m := range items {
		nativeID := asMapString(m, "id", "native_id")
		nameFields := []string{"name", "display_name", "displayName"}

		sealPhone := func(plain string, dest map[string]interface{}, numberKey string) error {
			if plain == "" || stringsHasAT1(plain) {
				return nil
			}
			fp := security.IdentityFingerprint(plain)
			dest["number_fp"] = fp
			aad := security.AADContactRecord(deviceID.String(), nativeID, fp)
			sealed, err := s.encryptor.SealString(plain, aad)
			if err != nil {
				return err
			}
			dest[numberKey] = sealed
			if err := security.SealMapStringFields(s.encryptor, m, nameFields, aad); err != nil {
				return err
			}
			return nil
		}

		if phones, ok := m["phones"].([]interface{}); ok {
			for i, p := range phones {
				switch t := p.(type) {
				case string:
					fp := security.IdentityFingerprint(t)
					aad := security.AADContactRecord(deviceID.String(), nativeID, fp)
					if !stringsHasAT1(t) && t != "" {
						sealed, err := s.encryptor.SealString(t, aad)
						if err != nil {
							return err
						}
						phones[i] = map[string]interface{}{"number": sealed, "number_fp": fp}
					}
					if err := security.SealMapStringFields(s.encryptor, m, nameFields, aad); err != nil {
						return err
					}
				case map[string]interface{}:
					num := asMapString(t, "number", "phone")
					key := "number"
					if asMapString(t, "number") == "" && asMapString(t, "phone") != "" {
						key = "phone"
					}
					if err := sealPhone(num, t, key); err != nil {
						return err
					}
				}
			}
		}

		number := asMapString(m, "number", "phone")
		if number != "" && !stringsHasAT1(number) {
			key := "number"
			if asMapString(m, "number") == "" {
				key = "phone"
			}
			if err := sealPhone(number, m, key); err != nil {
				return err
			}
		} else if fp := asMapString(m, "number_fp"); fp != "" {
			aad := security.AADContactRecord(deviceID.String(), nativeID, fp)
			if err := security.SealMapStringFields(s.encryptor, m, nameFields, aad); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) sealSMSMaps(deviceID uuid.UUID, items []map[string]interface{}) error {
	if s.encryptor == nil {
		return errors.New("encryptor required")
	}
	for _, m := range items {
		msgID := asMapString(m, "id", "native_id")
		aad := security.AADSMSRecord(deviceID.String(), msgID)
		if err := security.SealMapStringFields(s.encryptor, m, []string{"address", "message", "body", "name", "person"}, aad); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) sealCallMaps(deviceID uuid.UUID, items []map[string]interface{}) error {
	if s.encryptor == nil {
		return errors.New("encryptor required")
	}
	for _, m := range items {
		callID := asMapString(m, "call_id", "id")
		aad := security.AADCallRecord(deviceID.String(), callID)
		if err := security.SealMapStringFields(s.encryptor, m, []string{"number", "name", "name_call", "nameCall"}, aad); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) sealFileMaps(deviceID uuid.UUID, items []map[string]interface{}) error {
	if s.encryptor == nil {
		return errors.New("encryptor required")
	}
	for _, m := range items {
		pathPlain := asMapString(m, "path")
		if pathPlain == "" || stringsHasAT1(pathPlain) {
			continue
		}
		fp := security.IdentityFingerprint(pathPlain)
		m["path_fp"] = fp
		aad := security.AADFilePathRecord(deviceID.String(), fp)
		if err := security.SealMapStringFields(s.encryptor, m, []string{"path", "name", "file_name", "fileName"}, aad); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) sealLocationMaps(deviceID uuid.UUID, items []map[string]interface{}) error {
	if s.encryptor == nil {
		return errors.New("encryptor required: refusing to store plaintext")
	}
	for _, m := range items {
		locID := uuid.New()
		if existing := asMapString(m, "id"); existing != "" {
			if parsed, err := uuid.Parse(existing); err == nil {
				locID = parsed
			}
		}
		m["id"] = locID.String()
		aad := security.AADLocationRecord(deviceID.String(), locID.String())
		payload := map[string]interface{}{
			"latitude":  m["latitude"],
			"longitude": m["longitude"],
			"lat":       m["lat"],
			"lng":       m["lng"],
			"altitude":  m["altitude"],
			"accuracy":  m["accuracy"],
			"provider":  m["provider"],
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		sealed, err := s.encryptor.SealString(string(raw), aad)
		if err != nil {
			return err
		}
		m["enc_blob"] = sealed
		m["latitude"] = 0
		m["longitude"] = 0
		m["lat"] = 0
		m["lng"] = 0
		m["altitude"] = nil
		m["accuracy"] = nil
		m["provider"] = ""
	}
	return nil
}

func (s *Server) openLocations(items []models.StoredLocation) {
	for i := range items {
		if items[i].EncBlob == "" {
			continue
		}
		aad := security.AADLocationRecord(items[i].DeviceID.String(), items[i].ID.String())
		plain := s.openRecordBound(items[i].EncBlob, aad)
		if plain == "" {
			continue
		}
		var coords map[string]interface{}
		if err := json.Unmarshal([]byte(plain), &coords); err != nil {
			continue
		}
		if lat, ok := asAPIFloat(coords["latitude"]); ok {
			items[i].Latitude = lat
		} else if lat, ok := asAPIFloat(coords["lat"]); ok {
			items[i].Latitude = lat
		}
		if lng, ok := asAPIFloat(coords["longitude"]); ok {
			items[i].Longitude = lng
		} else if lng, ok := asAPIFloat(coords["lng"]); ok {
			items[i].Longitude = lng
		}
		if alt, ok := asAPIFloat(coords["altitude"]); ok {
			items[i].Altitude = &alt
		}
		if acc, ok := asAPIFloat(coords["accuracy"]); ok {
			items[i].Accuracy = &acc
		}
		if p, ok := coords["provider"].(string); ok {
			items[i].Provider = p
		}
		items[i].EncBlob = ""
	}
}

func stringsHasAT1(s string) bool {
	return strings.HasPrefix(s, security.EncStringPrefix)
}

func asMapString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
			if n, ok := v.(float64); ok {
				return strconv.FormatFloat(n, 'f', -1, 64)
			}
		}
	}
	return ""
}

func asAPIFloat(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(t, 64)
		return f, err == nil
	default:
		return 0, false
	}
}

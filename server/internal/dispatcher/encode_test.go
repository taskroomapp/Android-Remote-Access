package dispatcher

import (
	"encoding/base64"
	"testing"
)

func TestEncodeCommandData_JPEGBase64(t *testing.T) {
	jpeg := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	encoded := EncodeCommandData(jpeg)
	s, ok := encoded.(string)
	if !ok {
		t.Fatalf("expected string, got %T", encoded)
	}
	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("not valid base64: %v", err)
	}
	if len(decoded) < 2 || decoded[0] != 0xFF || decoded[1] != 0xD8 {
		t.Fatalf("decoded payload is not JPEG")
	}
}

func TestEncodeCommandData_JSONObject(t *testing.T) {
	raw := []byte(`{"recording":true}`)
	encoded := EncodeCommandData(raw)
	m, ok := encoded.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T (%v)", encoded, encoded)
	}
	if m["recording"] != true {
		t.Fatalf("unexpected map: %#v", m)
	}
}

func TestEncodeCommandData_PlainText(t *testing.T) {
	raw := []byte("hello world")
	encoded := EncodeCommandData(raw)
	if encoded != "hello world" {
		t.Fatalf("expected plain text, got %#v", encoded)
	}
}

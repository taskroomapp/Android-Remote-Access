package websocket

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
	"github.com/google/uuid"
)

func TestCKX1HandshakeAndFrame(t *testing.T) {
	serverX, err := cryptokit.GenerateX25519()
	if err != nil {
		t.Fatal(err)
	}
	serverEdPub, serverEdPriv, err := cryptokit.GenerateEd25519()
	if err != nil {
		t.Fatal(err)
	}
	deviceX, err := cryptokit.GenerateX25519()
	if err != nil {
		t.Fatal(err)
	}
	deviceEdPub, deviceEdPriv, err := cryptokit.GenerateEd25519()
	if err != nil {
		t.Fatal(err)
	}

	hub := NewHub(nil, nil)
	hub.SetCKX1Identity(NewServerIdentity(serverX, serverEdPub, serverEdPriv))

	client := &Client{
		ID:         uuid.New(),
		DeviceUUID: "device-agent-1",
		Hub:        hub,
		Send:       make(chan []byte, 8),
	}

	if err := client.sendKeyOffer(); err != nil {
		t.Fatal(err)
	}
	offerRaw := <-client.Send
	var offer map[string]interface{}
	if err := json.Unmarshal(offerRaw, &offer); err != nil {
		t.Fatal(err)
	}
	if offer["type"] != "key_offer" || offer["protocol"] != cryptokit.CKX1Protocol {
		t.Fatalf("bad key_offer: %v", offer)
	}

	sessionID, _ := offer["session_id"].(string)
	serverNonce, _ := offer["server_nonce"].(string)
	serverX25519B64, _ := offer["server_x25519_public_key"].(string)
	serverEd25519B64, _ := offer["server_ed25519_public_key"].(string)

	deviceX25519B64 := cryptokit.X25519PublicRawBase64(deviceX)
	deviceEd25519B64 := cryptokit.Ed25519PublicRawBase64(deviceEdPub)
	deviceNonce := base64.StdEncoding.EncodeToString(make([]byte, 16))

	transcript := cryptokit.HandshakeTranscript(
		sessionID, client.DeviceUUID,
		serverX25519B64, serverEd25519B64,
		deviceX25519B64, deviceEd25519B64,
		serverNonce, deviceNonce,
	)
	sig := cryptokit.SignEd25519(deviceEdPriv, transcript)

	msg := map[string]interface{}{
		"type":                     "key_exchange",
		"protocol":                 cryptokit.CKX1Protocol,
		"version":                  float64(cryptokit.CKX1Version),
		"session_id":               sessionID,
		"device_id":                 client.DeviceUUID,
		"device_x25519_public_key":  deviceX25519B64,
		"device_ed25519_public_key": deviceEd25519B64,
		"device_nonce":              deviceNonce,
		"signature":                base64.StdEncoding.EncodeToString(sig),
	}
	if err := client.handleKeyExchange(msg); err != nil {
		t.Fatalf("key_exchange: %v", err)
	}
	readyRaw := <-client.Send
	var ready map[string]interface{}
	_ = json.Unmarshal(readyRaw, &ready)
	if ready["type"] != "session_ready" {
		t.Fatalf("expected session_ready, got %v", ready)
	}
	if !client.hasSession() {
		t.Fatal("expected session ready")
	}

	wire, err := client.sealApplicationJSON([]byte(`{"transaction_id":"t1","command_type":"get_info"}`))
	if err != nil {
		t.Fatal(err)
	}
	var frame map[string]interface{}
	if err := json.Unmarshal(wire, &frame); err != nil {
		t.Fatal(err)
	}
	if frame["type"] != "enc" || frame["protocol"] != cryptokit.CKX1Protocol {
		t.Fatalf("bad enc frame: %v", frame)
	}

	// Device→server path: derive same keys and seal a heartbeat
	peerServer, _ := cryptokit.ParseX25519PublicRawBase64(serverX25519B64)
	shared, err := cryptokit.X25519SharedSecret(deviceX, peerServer)
	if err != nil {
		t.Fatal(err)
	}
	th := cryptokit.SHA256Sum(transcript)
	c2s, _, err := cryptokit.DeriveDirectionalKeys(shared, th)
	if err != nil {
		t.Fatal(err)
	}
	aad := cryptokit.FrameAAD(sessionID, client.DeviceUUID, cryptokit.DirDeviceToServer, 1, "-")
	nonce := make([]byte, cryptokit.CKX1NonceSize)
	ct, err := cryptokit.Ckx1AEADEncrypt(c2s, nonce, []byte(`{"type":"heartbeat"}`), aad)
	if err != nil {
		t.Fatal(err)
	}
	in := map[string]interface{}{
		"type":       "enc",
		"protocol":   cryptokit.CKX1Protocol,
		"version":    float64(cryptokit.CKX1Version),
		"session_id": sessionID,
		"seq":        float64(1),
		"dir":        cryptokit.DirDeviceToServer,
		"txn":        "-",
		"nonce":      base64.StdEncoding.EncodeToString(nonce),
		"ciphertext": base64.StdEncoding.EncodeToString(ct),
	}
	plain, err := client.openEncryptedEnvelope(in)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != `{"type":"heartbeat"}` {
		t.Fatalf("got %s", plain)
	}
	if _, err := client.openEncryptedEnvelope(in); err == nil {
		t.Fatal("duplicate seq must fail")
	}
}

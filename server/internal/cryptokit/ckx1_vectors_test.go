package cryptokit

import (
	"bytes"
	"crypto/ed25519"
	"testing"
)

func TestCKX1HandshakeRoundTrip(t *testing.T) {
	serverX, err := GenerateX25519()
	if err != nil {
		t.Fatal(err)
	}
	serverEdPub, serverEdPriv, err := GenerateEd25519()
	if err != nil {
		t.Fatal(err)
	}
	deviceX, err := GenerateX25519()
	if err != nil {
		t.Fatal(err)
	}
	deviceEdPub, deviceEdPriv, err := GenerateEd25519()
	if err != nil {
		t.Fatal(err)
	}

	sessionID := "11111111-1111-1111-1111-111111111111"
	deviceID := "22222222-2222-2222-2222-222222222222"
	serverX25519B64 := X25519PublicRawBase64(serverX)
	serverEd25519B64 := Ed25519PublicRawBase64(serverEdPub)
	deviceX25519B64 := X25519PublicRawBase64(deviceX)
	deviceEd25519B64 := Ed25519PublicRawBase64(deviceEdPub)
	serverNonce := "AAAAAAAAAAAAAAAAAAAAAA=="
	deviceNonce := "BBBBBBBBBBBBBBBBBBBBBB=="

	transcript := HandshakeTranscript(
		sessionID, deviceID,
		serverX25519B64, serverEd25519B64,
		deviceX25519B64, deviceEd25519B64,
		serverNonce, deviceNonce,
	)
	sig := SignEd25519(deviceEdPriv, transcript)
	if !VerifyEd25519(deviceEdPub, transcript, sig) {
		t.Fatal("signature verify failed")
	}
	_ = serverEdPriv

	peerPub, err := ParseX25519PublicRawBase64(deviceX25519B64)
	if err != nil {
		t.Fatal(err)
	}
	sharedServer, err := X25519SharedSecret(serverX, peerPub)
	if err != nil {
		t.Fatal(err)
	}
	peerServer, err := ParseX25519PublicRawBase64(serverX25519B64)
	if err != nil {
		t.Fatal(err)
	}
	sharedDevice, err := X25519SharedSecret(deviceX, peerServer)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sharedServer, sharedDevice) {
		t.Fatal("shared secrets differ")
	}

	th := SHA256Sum(transcript)
	c2sA, s2cA, err := DeriveDirectionalKeys(sharedServer, th)
	if err != nil {
		t.Fatal(err)
	}
	c2sB, s2cB, err := DeriveDirectionalKeys(sharedDevice, th)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(c2sA, c2sB) || !bytes.Equal(s2cA, s2cB) {
		t.Fatal("directional keys differ")
	}
	if bytes.Equal(c2sA, s2cA) {
		t.Fatal("directional keys must differ")
	}

	aad := FrameAAD(sessionID, deviceID, DirDeviceToServer, 1, "-")
	nonce := make([]byte, CKX1NonceSize)
	for i := range nonce {
		nonce[i] = byte(i)
	}
	ct, err := Ckx1AEADEncrypt(c2sA, nonce, []byte(`{"type":"heartbeat"}`), aad)
	if err != nil {
		t.Fatal(err)
	}
	pt, err := Ckx1AEADDecrypt(c2sB, nonce, ct, aad)
	if err != nil {
		t.Fatal(err)
	}
	if string(pt) != `{"type":"heartbeat"}` {
		t.Fatalf("plaintext mismatch: %s", pt)
	}
	if _, err := Ckx1AEADDecrypt(c2sB, nonce, ct, FrameAAD(sessionID, deviceID, DirServerToDevice, 1, "-")); err == nil {
		t.Fatal("wrong AAD must fail")
	}

	// Ed25519 type assert keep import used
	var _ ed25519.PublicKey = serverEdPub
}

func TestAT1RoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	aad := []byte("atrest:media:test")
	sealed, err := AT1Seal(key, []byte("secret"), aad)
	if err != nil {
		t.Fatal(err)
	}
	plain, err := AT1Open(key, sealed, aad)
	if err != nil {
		t.Fatal(err)
	}
	if string(plain) != "secret" {
		t.Fatal(plain)
	}
	if _, err := AT1Open(key, sealed, []byte("wrong")); err == nil {
		t.Fatal("wrong AAD must fail")
	}
}

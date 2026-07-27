package security

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestAtRestRoundTrip(t *testing.T) {
	e, err := NewDataEncryptor("unit-test-atrest-key-not-default")
	if err != nil {
		t.Fatal(err)
	}
	plain := []byte("secret-media-bytes")
	id := uuid.New()
	aad := []byte(AADMediaRecord(id))
	env, err := e.EncryptWithAAD(plain, aad)
	if err != nil {
		t.Fatal(err)
	}
	if len(env) < 12+16 {
		t.Fatalf("expected AT1 binary envelope, got len=%d", len(env))
	}
	out, err := e.DecryptWithAAD(env, aad)
	if err != nil || !bytes.Equal(out, plain) {
		t.Fatalf("decrypt mismatch: %v %q", err, out)
	}
}

func TestRefuseDefaultKey(t *testing.T) {
	if _, err := NewDataEncryptor("default-encryption-key-change"); err == nil {
		t.Fatal("expected refuse default key")
	}
}

func TestSealOpenStringAT1(t *testing.T) {
	e, err := NewDataEncryptor("unit-test-atrest-key-not-default")
	if err != nil {
		t.Fatal(err)
	}
	aad := AADSMSRecord(uuid.New().String(), "msg-1")
	sealed, err := e.SealString("hello-sms", aad)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sealed, EncStringPrefix) {
		t.Fatalf("expected AT1 prefix, got %q", sealed)
	}
	got, err := e.OpenString(sealed, aad)
	if err != nil || got != "hello-sms" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestRejectPlaintextSensitiveField(t *testing.T) {
	e, _ := NewDataEncryptor("unit-test-atrest-key-not-default")
	_, err := e.OpenString("legacy-plaintext", AADFieldSMSBody)
	if err != ErrPlaintextRejected {
		t.Fatalf("expected plaintext rejection, got %v", err)
	}
}

func TestWrongKey(t *testing.T) {
	e1, _ := NewDataEncryptor("unit-test-atrest-key-not-default-a")
	e2, _ := NewDataEncryptor("unit-test-atrest-key-not-default-b")
	aad := []byte(AADMediaRecord(uuid.New()))
	env, _ := e1.EncryptWithAAD([]byte("x"), aad)
	if _, err := e2.DecryptWithAAD(env, aad); err == nil {
		t.Fatal("wrong key must fail")
	}
}

func TestWrongAAD(t *testing.T) {
	e, _ := NewDataEncryptor("unit-test-atrest-key-not-default")
	env, _ := e.EncryptWithAAD([]byte("x"), []byte(AADAuditResponseRecord(uuid.New())))
	if _, err := e.DecryptWithAAD(env, []byte(AADAuditResponseRecord(uuid.New()))); err == nil {
		t.Fatal("wrong AAD must fail")
	}
}

func TestCrossRecordCiphertextSubstitution(t *testing.T) {
	e, _ := NewDataEncryptor("unit-test-atrest-key-not-default")
	idA, idB := uuid.New(), uuid.New()
	sealed, _ := e.SealString("secret", AADSMSRecord(idA.String(), "1"))
	if _, err := e.OpenString(sealed, AADSMSRecord(idB.String(), "1")); err == nil {
		t.Fatal("cross-record substitution must fail")
	}
}

func TestMustEncryptNil(t *testing.T) {
	if _, err := MustEncrypt(nil, []byte("x"), []byte(AADMedia)); err == nil {
		t.Fatal("expected error")
	}
}

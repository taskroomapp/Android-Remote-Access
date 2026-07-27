package security

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
	"github.com/google/uuid"
)

const (
	// EncStringPrefix marks at-rest string ciphertexts: AT1:<base64 nonce||ct>.
	EncStringPrefix = "AT1:"

	AADMedia          = "atrest:media"
	AADAuditPayload   = "atrest:audit-payload"
	AADAuditResponse  = "atrest:audit-response"
	AADPendingCommand = "atrest:pending-command"
	AADFieldSMSBody   = "atrest:field:sms-body"
	AADFieldContact   = "atrest:field:contact"
	AADFieldCall      = "atrest:field:call"
	AADFieldLocation  = "atrest:field:location"
	AADFieldFilePath  = "atrest:field:file-path"
	AADMirrorSnapshot = "atrest:mirror"
	AADDefault        = "atrest:data"
)

var (
	ErrPlaintextRejected   = errors.New("unexpected plaintext sensitive field: integrity error")
	ErrUnknownCipherFormat = errors.New("unknown ciphertext format")
)

// DataEncryptor encrypts persisted blobs with CKX1-ATREST ChaCha20-Poly1305 (AT1).
type DataEncryptor struct {
	key []byte
}

// NewDataEncryptor derives a 32-byte key under the CKX1-ATREST domain.
func NewDataEncryptor(masterKey string) (*DataEncryptor, error) {
	if strings.TrimSpace(masterKey) == "" {
		return nil, errors.New("encryption key is empty")
	}
	if masterKey == "default-encryption-key-change" ||
		masterKey == "your-32-byte-encryption-key-here" ||
		masterKey == "local-dev-atrest-key-change-me-before-any-shared-use" {
		return nil, errors.New("refusing insecure/default encryption_key; set a unique secret in config")
	}
	sum := sha256.Sum256([]byte(cryptokit.CKX1AtRestContext + "\x00" + masterKey))
	return &DataEncryptor{key: sum[:]}, nil
}

func AADMediaRecord(mediaID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", AADMedia, mediaID.String())
}

func AADAuditPayloadRecord(transactionID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", AADAuditPayload, transactionID.String())
}

func AADAuditResponseRecord(transactionID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", AADAuditResponse, transactionID.String())
}

func AADPendingRecord(pendingID uuid.UUID) string {
	return fmt.Sprintf("%s:%s", AADPendingCommand, pendingID.String())
}

func AADContactRecord(deviceID, nativeID, numberFP string) string {
	return fmt.Sprintf("%s:%s:%s:%s", AADFieldContact, deviceID, nativeID, numberFP)
}

func AADSMSRecord(deviceID, messageID string) string {
	return fmt.Sprintf("%s:%s:%s", AADFieldSMSBody, deviceID, messageID)
}

func AADCallRecord(deviceID, callID string) string {
	return fmt.Sprintf("%s:%s:%s", AADFieldCall, deviceID, callID)
}

func AADLocationRecord(deviceID, locationID string) string {
	return fmt.Sprintf("%s:%s:%s", AADFieldLocation, deviceID, locationID)
}

func AADFilePathRecord(deviceID, pathFP string) string {
	return fmt.Sprintf("%s:%s:%s", AADFieldFilePath, deviceID, pathFP)
}

func IdentityFingerprint(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("%x", sum[:])
}

func (e *DataEncryptor) Encrypt(plaintext []byte) ([]byte, error) {
	return e.EncryptWithAAD(plaintext, []byte(AADDefault))
}

func (e *DataEncryptor) EncryptWithAAD(plaintext, aad []byte) ([]byte, error) {
	if e == nil {
		return nil, errors.New("encryptor not configured")
	}
	return cryptokit.AT1SealBytes(e.key, plaintext, aad)
}

func (e *DataEncryptor) Decrypt(ciphertext []byte) ([]byte, error) {
	return e.DecryptWithAAD(ciphertext, []byte(AADDefault))
}

func (e *DataEncryptor) DecryptWithAAD(ciphertext, aad []byte) ([]byte, error) {
	if e == nil {
		return nil, errors.New("encryptor not configured")
	}
	plain, err := cryptokit.AT1OpenBytes(e.key, ciphertext, aad)
	if err != nil {
		return nil, ErrUnknownCipherFormat
	}
	return plain, nil
}

func (e *DataEncryptor) SealString(plain, aad string) (string, error) {
	return cryptokit.AT1Seal(e.key, []byte(plain), []byte(aad))
}

func (e *DataEncryptor) OpenString(sealed, aad string) (string, error) {
	if e == nil {
		return "", errors.New("encryptor not configured")
	}
	if sealed == "" {
		return "", nil
	}
	if !strings.HasPrefix(sealed, EncStringPrefix) {
		log.Printf("security: integrity error — unexpected plaintext sensitive field")
		return "", ErrPlaintextRejected
	}
	plain, err := cryptokit.AT1Open(e.key, sealed, []byte(aad))
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func MustEncrypt(e *DataEncryptor, plaintext, aad []byte) ([]byte, error) {
	if e == nil {
		return nil, errors.New("encryptor required: refusing to store plaintext")
	}
	return e.EncryptWithAAD(plaintext, aad)
}

func SealMapStringFields(e *DataEncryptor, m map[string]interface{}, fields []string, aad string) error {
	if e == nil {
		return errors.New("encryptor required")
	}
	for _, f := range fields {
		v, ok := m[f]
		if !ok || v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		if strings.HasPrefix(s, EncStringPrefix) {
			continue
		}
		sealed, err := e.SealString(s, aad)
		if err != nil {
			return err
		}
		m[f] = sealed
	}
	return nil
}

func OpenMapStringFields(e *DataEncryptor, m map[string]interface{}, fields []string, aad string) error {
	if e == nil {
		return errors.New("encryptor required")
	}
	for _, f := range fields {
		v, ok := m[f]
		if !ok || v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		plain, err := e.OpenString(s, aad)
		if err != nil {
			return err
		}
		m[f] = plain
	}
	return nil
}

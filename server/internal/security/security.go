package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	// JWT claims keys
	jwtAdminIDKey    = "admin_id"
	jwtUsernameKey   = "username"
	jwtRoleKey       = "role"
	jwtPermissionsKey = "permissions"

	// Token expiration times
	AccessTokenDuration  = 15 * time.Minute
	RefreshTokenDuration = 7 * 24 * time.Hour // 7 days
)

// JWTManager handles JWT token operations
type JWTManager struct {
	secretKey []byte
	issuer    string
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(secretKey, issuer string) *JWTManager {
	return &JWTManager{
		secretKey: []byte(secretKey),
		issuer:    issuer,
	}
}

// TokenPair represents an access/refresh token pair
type TokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// GenerateTokenPair generates access and refresh tokens for an admin
func (j *JWTManager) GenerateTokenPair(admin *models.Administrator) (*TokenPair, error) {
	now := time.Now()

	// Access token
	accessClaims := jwt.MapClaims{
		jwtAdminIDKey:     admin.ID.String(),
		jwtUsernameKey:   admin.Username,
		jwtRoleKey:       admin.Role,
		jwtPermissionsKey: admin.Permissions,
		"iat":            now.Unix(),
		"exp":            now.Add(AccessTokenDuration).Unix(),
		"iss":            j.issuer,
		"jti":            uuid.New().String(),
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString(j.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh token (simpler structure)
	refreshClaims := jwt.MapClaims{
		jwtAdminIDKey: admin.ID.String(),
		"iat":         now.Unix(),
		"exp":         now.Add(RefreshTokenDuration).Unix(),
		"iss":         j.issuer,
		"jti":         uuid.New().String(),
		"type":        "refresh",
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(j.secretKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenString,
		ExpiresAt:    now.Add(RefreshTokenDuration),
	}, nil
}

// ValidateAccessToken validates and parses an access token
func (j *JWTManager) ValidateAccessToken(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return nil, nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, nil, errors.New("invalid token claims")
	}

	// Check expiration
	if exp, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(exp), 0).Before(time.Now()) {
			return nil, nil, errors.New("token expired")
		}
	}

	return token, claims, nil
}

// ValidateRefreshToken validates a refresh token and returns the admin ID
func (j *JWTManager) ValidateRefreshToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.secretKey, nil
	})

	if err != nil {
		return uuid.Nil, err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, errors.New("invalid token claims")
	}

	// Check it's a refresh token
	if tokenType, ok := claims["type"].(string); !ok || tokenType != "refresh" {
		return uuid.Nil, errors.New("not a refresh token")
	}

	adminIDStr, ok := claims[jwtAdminIDKey].(string)
	if !ok {
		return uuid.Nil, errors.New("missing admin ID in token")
	}

	return uuid.Parse(adminIDStr)
}

// GetAdminIDFromClaims extracts admin ID from JWT claims
func (j *JWTManager) GetAdminIDFromClaims(claims jwt.MapClaims) (uuid.UUID, error) {
	adminIDStr, ok := claims[jwtAdminIDKey].(string)
	if !ok {
		return uuid.Nil, errors.New("missing admin ID in claims")
	}
	return uuid.Parse(adminIDStr)
}

// PasswordHasher handles password hashing
type PasswordHasher struct {
	cost int
}

// NewPasswordHasher creates a new password hasher
func NewPasswordHasher() *PasswordHasher {
	return &PasswordHasher{cost: bcrypt.DefaultCost}
}

// HashPassword hashes a password using bcrypt
func (p *PasswordHasher) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), p.cost)
	return string(bytes), err
}

// VerifyPassword verifies a password against a hash
func (p *PasswordHasher) VerifyPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateChecksum generates a SHA-256 checksum of data
func GenerateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// CertificateManager handles TLS certificate operations
type CertificateManager struct {
	caCert *x509.Certificate
	caKey  interface{}
}

// NewCertificateManager creates a certificate manager
func NewCertificateManager() *CertificateManager {
	return &CertificateManager{}
}

// GenerateCACertificate generates a self-signed CA certificate
func (c *CertificateManager) GenerateCACertificate(org string, yearsValid int) error {
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{org},
			CommonName:   "Android Remote Access CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(yearsValid, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	c.caCert = template
	return nil
}

// GenerateServerCertificate generates a certificate for the server
func (c *CertificateManager) GenerateServerCertificate(hosts []string, org string, validDays int) ([]byte, []byte, error) {
	if c.caCert == nil {
		return nil, nil, errors.New("CA not initialized")
	}

	// Generate key pair
	key, err := generateRSAKey(2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{org},
			CommonName:   hosts[0],
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(0, 0, validDays),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: parseIPs(hosts),
		DNSNames:    parseDNSNames(hosts),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, c.caCert, key.Public(), c.caKey)
	if err != nil {
		return nil, nil, err
	}

	keyPEM, err := encodePrivateKey(key)
	if err != nil {
		return nil, nil, err
	}

	return certDER, keyPEM, nil
}

// CreateServerTLSConfig creates a TLS configuration for the server
func CreateServerTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:  tls.VersionTLS12,
		ClientAuth:  tls.RequestClientCert,
	}, nil
}

// CreateClientTLSConfig creates a TLS configuration that verifies server cert
func CreateClientTLSConfig(caCertFile string) (*tls.Config, error) {
	caCert, err := osReadFile(caCertFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to parse CA certificate")
	}

	return &tls.Config{
		RootCAs:    caCertPool,
		MinVersion: tls.VersionTLS12,
	}, nil
}

// PermissionChecker checks admin permissions
type PermissionChecker struct {
	rolePermissions map[string][]string
}

// NewPermissionChecker creates a new permission checker
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{
		rolePermissions: map[string][]string{
			"super_admin": {
				"device:enroll", "device:revoke", "device:command",
				"file:*", "contacts:*", "calls:*", "sms:*",
				"camera:*", "mic:*", "location:*",
				"audit:read", "admin:*",
			},
			"admin": {
				"device:command",
				"file:read", "file:list",
				"contacts:read", "calls:read", "sms:read",
				"camera:snapshot", "mic:record",
				"location:read",
				"audit:read",
			},
			"operator": {
				"device:status",
				"file:list",
				"contacts:read",
				"calls:read",
				"sms:read",
			},
		},
	}
}

// HasPermission checks if an admin has a specific permission
func (p *PermissionChecker) HasPermission(admin *models.Administrator, permission string) bool {
	if !admin.IsActive {
		return false
	}

	perms, ok := p.rolePermissions[admin.Role]
	if !ok {
		return false
	}

	for _, perm := range perms {
		if perm == permission {
			return true
		}
		// Wildcard matching
		if strings.HasSuffix(perm, ":*") {
			prefix := strings.TrimSuffix(perm, ":*")
			if strings.HasPrefix(permission, prefix+":") {
				return true
			}
		}
	}
	return false
}

// HasAnyPermission checks if admin has any of the specified permissions
func (p *PermissionChecker) HasAnyPermission(admin *models.Administrator, permissions []string) bool {
	for _, perm := range permissions {
		if p.HasPermission(admin, perm) {
			return true
		}
	}
	return false
}

// Helper functions

func parseIPs(hosts []string) []net.IP {
	var ips []net.IP
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

func parseDNSNames(hosts []string) []string {
	var names []string
	for _, h := range hosts {
		if net.ParseIP(h) == nil {
			names = append(names, h)
		}
	}
	return names
}

// APIKeyAuth provides API key authentication
type APIKeyAuth struct {
	validKeys map[string]string // key -> device ID
}

// NewAPIKeyAuth creates a new API key authenticator
func NewAPIKeyAuth() *APIKeyAuth {
	return &APIKeyAuth{
		validKeys: make(map[string]string),
	}
}

// GenerateAPIKey generates a new API key for a device
func (a *APIKeyAuth) GenerateAPIKey(deviceID string) string {
	b := make([]byte, 32)
	rand.Read(b)
	key := base64.URLEncoding.EncodeToString(b)
	a.validKeys[key] = deviceID
	return key
}

// ValidateAPIKey validates an API key and returns the associated device ID
func (a *APIKeyAuth) ValidateAPIKey(key string) (string, bool) {
	deviceID, ok := a.validKeys[key]
	return deviceID, ok
}

// RevokeAPIKey removes an API key
func (a *APIKeyAuth) RevokeAPIKey(key string) {
	delete(a.validKeys, key)
}

// LoadAPIKeys loads API keys from JSON
func (a *APIKeyAuth) LoadAPIKeys(data []byte) error {
	return json.Unmarshal(data, &a.validKeys)
}

// ExportAPIKeys exports API keys to JSON
func (a *APIKeyAuth) ExportAPIKeys() ([]byte, error) {
	return json.Marshal(a.validKeys)
}

// InputValidator provides input validation utilities
type InputValidator struct{}

// NewInputValidator creates a new input validator
func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidateDeviceUUID validates a device UUID format
func (v *InputValidator) ValidateDeviceUUID(deviceID string) error {
	_, err := uuid.Parse(deviceID)
	return err
}

// SanitizeString removes potentially dangerous characters
func (v *InputValidator) SanitizeString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// ValidateCommandType validates a command type
func (v *InputValidator) ValidateCommandType(cmdType string) bool {
	validTypes := map[string]bool{
		"file_list": true, "file_read": true, "file_read_chunk": true, "file_write": true,
		"file_delete": true, "file_rename": true, "file_move": true,
		"file_download": true, "file_upload": true, "file_get_directory": true,
		"get_foreground_app": true, "get_browser_history": true,
		"get_installed_apps": true,
		"get_contacts": true, "get_call_logs": true, "get_sms_messages": true,
		"camera_snapshot": true, "camera_stream": true, "camera_stop": true,
		"mic_start": true, "mic_stop": true, "mic_stream": true,
		"get_device_info": true, "get_location": true,
		"heartbeat": true, "device_enroll": true, "device_disconnect": true,
	}
	return validTypes[cmdType]
}

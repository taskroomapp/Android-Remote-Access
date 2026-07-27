package main

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the server configuration
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	Redis     RedisConfig     `yaml:"redis"`
	Security  SecurityConfig  `yaml:"security"`
	TLS       TLSConfig       `yaml:"tls"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type RedisConfig struct {
	Addr     string `yaml:"addr"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type SecurityConfig struct {
	JWTSecret     string     `yaml:"jwt_secret"`
	EncryptionKey string     `yaml:"encryption_key"`
	Issuer        string     `yaml:"issuer"`
	CKX1          CKX1Config `yaml:"ckx1"`
}

type CKX1Config struct {
	Enabled                     bool     `yaml:"enabled"`
	ProtocolVersion             int      `yaml:"protocol_version"`
	ServerX25519PrivateKeyFile  string   `yaml:"server_x25519_private_key_file"`
	ServerEd25519PrivateKeyFile string   `yaml:"server_ed25519_private_key_file"`
	AllowedAlgorithms           []string `yaml:"allowed_algorithms"`
	SessionTimeoutSeconds       int      `yaml:"session_timeout_seconds"`
	HandshakeTimeoutSeconds     int      `yaml:"handshake_timeout_seconds"`
	MaxMessageBytes             int      `yaml:"max_message_bytes"`
}

type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == "" {
		cfg.Server.Port = "8443"
	}
	if cfg.Security.JWTSecret == "" {
		cfg.Security.JWTSecret = "default-jwt-secret-change-in-production"
	}
	if cfg.Security.EncryptionKey == "" {
		return nil, fmt.Errorf("security.encryption_key is required")
	}
	if cfg.Security.Issuer == "" {
		cfg.Security.Issuer = "android-remote-access"
	}
	if !cfg.Security.CKX1.Enabled {
		cfg.Security.CKX1.Enabled = true
	}
	if cfg.Security.CKX1.ProtocolVersion == 0 {
		cfg.Security.CKX1.ProtocolVersion = 1
	}
	if cfg.Security.CKX1.ServerX25519PrivateKeyFile == "" {
		cfg.Security.CKX1.ServerX25519PrivateKeyFile = "data/server-x25519.pkcs8"
	}
	if cfg.Security.CKX1.ServerEd25519PrivateKeyFile == "" {
		cfg.Security.CKX1.ServerEd25519PrivateKeyFile = "data/server-ed25519.pkcs8"
	}
	if len(cfg.Security.CKX1.AllowedAlgorithms) == 0 {
		cfg.Security.CKX1.AllowedAlgorithms = []string{"CKX1-X25519-HKDF-SHA256-CHACHA20-POLY1305"}
	}

	return &cfg, nil
}

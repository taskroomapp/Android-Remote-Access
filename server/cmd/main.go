package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/api"
	"github.com/enterprise/android-remote-access/server/internal/cryptokit"
	"github.com/enterprise/android-remote-access/server/internal/database"
	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/enterprise/android-remote-access/server/internal/security"
	ws "github.com/enterprise/android-remote-access/server/internal/websocket"
	"github.com/google/uuid"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	generateConfig := flag.Bool("generate-config", false, "Generate sample config file")
	flag.Parse()

	if *generateConfig {
		generateSampleConfig()
		return
	}

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize database
	var db *database.PostgresDB
	if cfg.Database.URL != "" {
		db, err = database.NewPostgresDB(cfg.Database.URL)
		if err != nil {
			log.Printf("Warning: Failed to connect to PostgreSQL: %v", err)
			log.Println("Server will run with in-memory storage only")
		} else {
			// Initialize schema
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := db.InitSchema(ctx); err != nil {
				log.Printf("Warning: Failed to initialize database schema: %v", err)
			}
			cancel()
		}
	}

	// Initialize Redis cache
	var cache *database.RedisCache
	if cfg.Redis.Addr != "" {
		cache, err = database.NewRedisCache(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
		if err != nil {
			log.Printf("Warning: Failed to connect to Redis: %v", err)
		}
	}

	// Initialize encryption
	encryptor, err := security.NewDataEncryptor(cfg.Security.EncryptionKey)
	if err != nil {
		log.Fatalf("Failed to create encryptor: %v", err)
	}

	x25519Priv, err := cryptokit.LoadOrGenerateX25519Key(cfg.Security.CKX1.ServerX25519PrivateKeyFile)
	if err != nil {
		log.Fatalf("Failed to load CKX1 X25519 key: %v", err)
	}
	edPub, edPriv, err := cryptokit.LoadOrGenerateEd25519Key(cfg.Security.CKX1.ServerEd25519PrivateKeyFile)
	if err != nil {
		log.Fatalf("Failed to load CKX1 Ed25519 key: %v", err)
	}
	identity := ws.NewServerIdentity(x25519Priv, edPub, edPriv)
	log.Printf("CKX1 server identity ready (fp=%s)", identity.Fingerprint)

	// Initialize WebSocket hub
	hub := ws.NewHub(db, cache)
	hub.SetCKX1Identity(identity)

	// Initialize command dispatcher
	dispatcher := dispatcher.NewCommandDispatcher(hub, db, cache, encryptor)

	// Start hub
	go hub.Run()

	// Initialize API server
	apiCfg := &api.Config{
		ServerHost:    cfg.Server.Host,
		ServerPort:    cfg.Server.Port,
		JWTSecret:     cfg.Security.JWTSecret,
		EncryptionKey: cfg.Security.EncryptionKey,
		Issuer:        cfg.Security.Issuer,
	}

	server := api.NewServer(db, cache, hub, dispatcher, apiCfg)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:         cfg.Server.Host + ":" + cfg.Server.Port,
		Handler:      server.Router(),
		ReadTimeout:  180 * time.Second,
		WriteTimeout: 180 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Configure TLS if enabled
	if cfg.TLS.Enabled {
		tlsConfig, err := security.CreateServerTLSConfig(cfg.TLS.CertFile, cfg.TLS.KeyFile)
		if err != nil {
			log.Fatalf("Failed to create TLS config: %v", err)
		}
		httpServer.TLSConfig = tlsConfig
	}

	// Create default admin if none exists
	if db != nil {
		createDefaultAdmin(db)
	}

	// Start server
	go func() {
		log.Printf("Starting server on %s:%s", cfg.Server.Host, cfg.Server.Port)
		if cfg.TLS.Enabled {
			log.Printf("HTTPS enabled with cert: %s", cfg.TLS.CertFile)
			if err := httpServer.ListenAndServeTLS(cfg.TLS.CertFile, cfg.TLS.KeyFile); err != nil {
				log.Fatalf("Server error: %v", err)
			}
		} else {
			if err := httpServer.ListenAndServe(); err != nil {
				log.Fatalf("Server error: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	if db != nil {
		db.Close()
	}
	if cache != nil {
		cache.Close()
	}

	log.Println("Server stopped")
}

func createDefaultAdmin(db *database.PostgresDB) {
	ctx := context.Background()

	if _, err := db.GetAdministratorByUsername(ctx, "admin"); err == nil {
		return
	}

	// Create default super admin
	passwordHash, err := security.NewPasswordHasher().HashPassword("admin123!")
	if err != nil {
		log.Printf("Failed to hash default admin password: %v", err)
		return
	}

	admin := &models.Administrator{
		ID:           uuid.New(),
		Username:     "admin",
		PasswordHash: passwordHash,
		Email:        "admin@remoteaccess.local",
		Role:         "super_admin",
		Permissions: []string{
			"device:enroll", "device:revoke", "device:command",
			"file:*", "contacts:*", "calls:*", "sms:*",
			"camera:*", "mic:*", "location:*",
			"audit:read", "admin:*",
		},
		IsActive: true,
	}

	if err := db.CreateAdministrator(ctx, admin); err != nil {
		log.Printf("Failed to create default admin (may already exist): %v", err)
	} else {
		log.Println("Default admin created: admin / admin123!")
		log.Println("WARNING: Change this password in production!")
	}
}

func generateSampleConfig() {
	config := `# Android Remote Access Server Configuration

# Server Configuration
server:
  host: "0.0.0.0"
  port: "8443"

# Database Configuration
database:
  url: "postgres://user:password@localhost:5432/android_remote_access?sslmode=require"

# Redis Configuration
redis:
  addr: "localhost:6379"
  password: ""
  db: 0

# Security Configuration
security:
  jwt_secret: "your-super-secret-jwt-key-change-in-production-minimum-32-bytes"
  encryption_key: "dev-only-egl-android-remote-2026-unique-secret"
  issuer: "android-remote-access"
  ckx1:
    enabled: true
    protocol_version: 1
    server_x25519_private_key_file: "data/server-x25519.pkcs8"
    server_ed25519_private_key_file: "data/server-ed25519.pkcs8"
    allowed_algorithms:
      - CKX1-X25519-HKDF-SHA256-CHACHA20-POLY1305
    session_timeout_seconds: 3600
    handshake_timeout_seconds: 15
    max_message_bytes: 524288

# TLS Configuration (optional, for production)
tls:
  enabled: false
  cert_file: "/path/to/server.crt"
  key_file: "/path/to/server.key"

# API Rate Limiting
rate_limit:
  enabled: true
  requests_per_minute: 100
`

	fmt.Println(config)
}

package api

import (
	"log"
	"net/http"
	"sync"

	"github.com/enterprise/android-remote-access/server/internal/database"
	"github.com/enterprise/android-remote-access/server/internal/dispatcher"
	"github.com/enterprise/android-remote-access/server/internal/security"
	"github.com/enterprise/android-remote-access/server/internal/websocket"
	"github.com/gorilla/mux"
)

// Server provides HTTP API functionality
type Server struct {
	router            *mux.Router
	db                *database.PostgresDB
	cache             *database.RedisCache
	hub               *websocket.Hub
	dispatcher        *dispatcher.CommandDispatcher
	jwtManager        *security.JWTManager
	passwordHasher    *security.PasswordHasher
	permissionChecker *security.PermissionChecker
	validator         *security.InputValidator
	encryptor         *security.DataEncryptor
	config            *Config
	mirrors           *mirrorStore
	adminCKX1         *security.AdminCKX1Store
	adminOfferMu      sync.Mutex
	adminOffers       map[string]pendingAdminOffer
}

// Config holds server configuration
type Config struct {
	ServerHost    string
	ServerPort    string
	JWTSecret     string
	EncryptionKey string
	Issuer        string
}

// NewServer creates a new API server
func NewServer(db *database.PostgresDB, cache *database.RedisCache, hub *websocket.Hub, disp *dispatcher.CommandDispatcher, cfg *Config) *Server {
	var encryptor *security.DataEncryptor
	if cfg != nil && cfg.EncryptionKey != "" {
		if e, err := security.NewDataEncryptor(cfg.EncryptionKey); err == nil {
			encryptor = e
		} else {
			log.Printf("Warning: failed to init media encryptor: %v", err)
		}
	}

	s := &Server{
		router:            mux.NewRouter(),
		db:                db,
		cache:             cache,
		hub:               hub,
		dispatcher:        disp,
		jwtManager:        security.NewJWTManager(cfg.JWTSecret, cfg.Issuer),
		passwordHasher:    security.NewPasswordHasher(),
		permissionChecker: security.NewPermissionChecker(),
		validator:         security.NewInputValidator(),
		encryptor:         encryptor,
		config:            cfg,
		mirrors:           newMirrorStore(),
		adminCKX1:         security.NewAdminCKX1Store(),
		adminOffers:       make(map[string]pendingAdminOffer),
	}

	s.setupRoutes()
	return s
}

// Router returns the HTTP handler for the API server.
func (s *Server) Router() http.Handler {
	return s.router
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")

	v1 := s.router.PathPrefix("/api/v1").Subrouter()

	auth := v1.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/login", s.handleLogin).Methods("POST")
	auth.HandleFunc("/refresh", s.handleRefresh).Methods("POST")
	auth.HandleFunc("/logout", s.handleLogout).Methods("POST")

	ckx1Auth := v1.PathPrefix("/auth/ckx1").Subrouter()
	ckx1Auth.Use(s.authMiddleware)
	ckx1Auth.HandleFunc("/offer", s.handleAdminCKX1Offer).Methods("POST")
	ckx1Auth.HandleFunc("/exchange", s.handleAdminCKX1Exchange).Methods("POST")

	useProtected := func(r *mux.Router) {
		r.Use(s.authMiddleware)
		r.Use(s.ckx1BodyMiddleware)
	}

	devices := v1.PathPrefix("/devices").Subrouter()
	useProtected(devices)
	devices.HandleFunc("", s.handleListDevices).Methods("GET")
	devices.HandleFunc("", s.handleEnrollDevice).Methods("POST")
	devices.HandleFunc("/{id}", s.handleGetDevice).Methods("GET")
	devices.HandleFunc("/{id}", s.handleDeleteDevice).Methods("DELETE")
	devices.HandleFunc("/{id}/status", s.handleGetDeviceStatus).Methods("GET")
	devices.HandleFunc("/{id}/comms", s.handleListDeviceComms).Methods("GET")
	devices.HandleFunc("/{id}/comms/save", s.handleSaveDeviceComms).Methods("POST")
	devices.HandleFunc("/{id}/comms/export", s.handleExportDeviceComms).Methods("GET")
	devices.HandleFunc("/{id}/artifacts", s.handleListDeviceArtifacts).Methods("GET")
	devices.HandleFunc("/{id}/artifacts/save", s.handleSaveDeviceArtifacts).Methods("POST")
	devices.HandleFunc("/{id}/artifacts/export", s.handleExportDeviceArtifacts).Methods("GET")

	commands := v1.PathPrefix("/commands").Subrouter()
	useProtected(commands)
	commands.HandleFunc("", s.handleExecuteCommand).Methods("POST")
	commands.HandleFunc("/{transaction_id}", s.handleGetCommandStatus).Methods("GET")
	commands.HandleFunc("/{transaction_id}/cancel", s.handleCancelCommand).Methods("POST")

	files := v1.PathPrefix("/files").Subrouter()
	useProtected(files)
	files.HandleFunc("/list", s.handleFileList).Methods("GET")
	files.HandleFunc("/read", s.handleFileRead).Methods("GET")
	files.HandleFunc("/stream", s.handleFileStream).Methods("GET")
	files.HandleFunc("/delete", s.handleFileDelete).Methods("DELETE")
	files.HandleFunc("/download/{file_id}", s.handleDownloadFile).Methods("GET")

	contacts := v1.PathPrefix("/contacts").Subrouter()
	useProtected(contacts)
	contacts.HandleFunc("/{device_id}", s.handleGetContacts).Methods("GET")

	callLogs := v1.PathPrefix("/calls").Subrouter()
	useProtected(callLogs)
	callLogs.HandleFunc("/{device_id}", s.handleGetCallLogs).Methods("GET")

	media := v1.PathPrefix("/media").Subrouter()
	useProtected(media)
	media.HandleFunc("/file/{file_id}", s.handleGetMediaFile).Methods("GET")
	media.HandleFunc("/{device_id}", s.handleGetMedia).Methods("GET")

	actions := v1.PathPrefix("/actions").Subrouter()
	useProtected(actions)
	actions.HandleFunc("/{device_id}/camera", s.handleCameraAction).Methods("POST")
	actions.HandleFunc("/{device_id}/microphone", s.handleMicAction).Methods("POST")
	actions.HandleFunc("/{device_id}/location", s.handleLocationAction).Methods("POST")
	actions.HandleFunc("/{device_id}/foreground-app", s.handleForegroundAppAction).Methods("GET")

	transfers := v1.PathPrefix("/transfers").Subrouter()
	useProtected(transfers)
	transfers.HandleFunc("", s.handleListTransfers).Methods("GET")
	transfers.HandleFunc("/completed", s.handleClearCompletedTransfers).Methods("DELETE")
	transfers.HandleFunc("/{transfer_id}/appeal", s.handleTransferAppeal).Methods("POST")

	mirrors := v1.PathPrefix("/mirrors").Subrouter()
	useProtected(mirrors)
	mirrors.HandleFunc("/{device_id}", s.handleGetMirror).Methods("GET")
	mirrors.HandleFunc("/{device_id}/update", s.handleMirrorUpdate).Methods("POST")

	audit := v1.PathPrefix("/audit").Subrouter()
	useProtected(audit)
	audit.HandleFunc("/logs", s.handleSearchAuditLogs).Methods("POST")
	audit.HandleFunc("/logs/{id}", s.handleGetAuditLog).Methods("GET")

	dashboard := v1.PathPrefix("/dashboard").Subrouter()
	useProtected(dashboard)
	dashboard.HandleFunc("/stats", s.handleDashboardStats).Methods("GET")

	admins := v1.PathPrefix("/admins").Subrouter()
	useProtected(admins)
	admins.HandleFunc("", s.handleListAdmins).Methods("GET")
	admins.HandleFunc("", s.handleCreateAdmin).Methods("POST")
	admins.HandleFunc("/{id}", s.handleGetAdmin).Methods("GET")
	admins.HandleFunc("/{id}", s.handleUpdateAdmin).Methods("PUT")
	admins.HandleFunc("/{id}", s.handleDeleteAdmin).Methods("DELETE")

	s.router.HandleFunc("/ws/devices/{device_id}", s.handleDeviceWebSocket)
	s.router.HandleFunc("/ws/admin", s.handleAdminWebSocket)
}

package api

import (
	"encoding/json"
	"net/http"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleListAdmins(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "admin:*") {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Insufficient permissions")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string][]models.AdminInfo{})
}

func (s *Server) handleCreateAdmin(w http.ResponseWriter, r *http.Request) {
	admin := getAdmin(r.Context())

	if !s.permissionChecker.HasPermission(admin, "admin:*") || admin.Role != "super_admin" {
		s.writeError(w, http.StatusForbidden, "FORBIDDEN", "Only super admins can create admins")
		return
	}

	var req struct {
		Username    string   `json:"username"`
		Password    string   `json:"password"`
		Email       string   `json:"email"`
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	passwordHash, err := s.passwordHasher.HashPassword(req.Password)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "HASH_ERROR", "Failed to hash password")
		return
	}

	newAdmin := &models.Administrator{
		ID:           uuid.New(),
		Username:     req.Username,
		PasswordHash: passwordHash,
		Email:        req.Email,
		Role:         req.Role,
		Permissions:  req.Permissions,
		IsActive:     true,
	}

	if err := s.db.CreateAdministrator(r.Context(), newAdmin); err != nil {
		s.writeError(w, http.StatusInternalServerError, "DB_ERROR", "Failed to create admin")
		return
	}

	s.writeJSON(w, http.StatusCreated, models.AdminInfo{
		ID:          newAdmin.ID.String(),
		Username:    newAdmin.Username,
		Email:       newAdmin.Email,
		Role:        newAdmin.Role,
		Permissions: newAdmin.Permissions,
	})
}

func (s *Server) handleGetAdmin(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Get admin"})
}

func (s *Server) handleUpdateAdmin(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Update admin"})
}

func (s *Server) handleDeleteAdmin(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Delete admin"})
}

package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
)

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	// Get admin from database
	admin, err := s.db.GetAdministratorByUsername(r.Context(), req.Username)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "AUTH_FAILED", "Invalid credentials")
		return
	}

	// Verify password
	if !s.passwordHasher.VerifyPassword(req.Password, admin.PasswordHash) {
		s.writeError(w, http.StatusUnauthorized, "AUTH_FAILED", "Invalid credentials")
		return
	}

	// Generate tokens
	tokens, err := s.jwtManager.GenerateTokenPair(admin)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate tokens")
		return
	}

	// Create session
	session := &models.Session{
		ID:              uuid.New(),
		AdministratorID: admin.ID,
		RefreshToken:    tokens.RefreshToken,
		IPAddress:       getClientIP(r),
		UserAgent:       r.UserAgent(),
		ExpiresAt:       tokens.ExpiresAt,
	}

	if err := s.db.CreateSession(r.Context(), session); err != nil {
		s.writeError(w, http.StatusInternalServerError, "SESSION_ERROR", "Failed to create session")
		return
	}

	// Cache session
	if s.cache != nil {
		s.cache.CacheSession(r.Context(), session.ID.String(), admin.ID, tokens.ExpiresAt.Sub(time.Now()))
	}

	s.writeJSON(w, http.StatusOK, models.LoginResponse{
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresAt:    tokens.ExpiresAt,
		Admin: models.AdminInfo{
			ID:          admin.ID.String(),
			Username:    admin.Username,
			Email:       admin.Email,
			Role:        admin.Role,
			Permissions: admin.Permissions,
		},
	})
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	var req models.RefreshTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}

	adminID, err := s.jwtManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid refresh token")
		return
	}

	// Verify session exists
	session, err := s.db.GetSessionByRefreshToken(r.Context(), req.RefreshToken)
	if err != nil || session.AdministratorID != adminID {
		s.writeError(w, http.StatusUnauthorized, "INVALID_SESSION", "Session not found")
		return
	}

	// Get admin
	admin, err := s.db.GetAdministratorByID(r.Context(), adminID)
	if err != nil || !admin.IsActive {
		s.writeError(w, http.StatusUnauthorized, "INVALID_ADMIN", "Admin not found or inactive")
		return
	}

	// Delete old session
	s.db.DeleteSessionByRefreshToken(r.Context(), req.RefreshToken)

	// Generate new tokens
	tokens, err := s.jwtManager.GenerateTokenPair(admin)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "TOKEN_ERROR", "Failed to generate tokens")
		return
	}

	// Create new session
	newSession := &models.Session{
		ID:              uuid.New(),
		AdministratorID: admin.ID,
		RefreshToken:    tokens.RefreshToken,
		IPAddress:       getClientIP(r),
		UserAgent:       r.UserAgent(),
		ExpiresAt:       tokens.ExpiresAt,
	}

	s.db.CreateSession(r.Context(), newSession)

	s.writeJSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_at":    tokens.ExpiresAt,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) == 2 {
			_, claims, _ := s.jwtManager.ValidateAccessToken(tokenParts[1])
			if claims != nil {
				if adminID, _ := s.jwtManager.GetAdminIDFromClaims(claims); adminID != uuid.Nil {
					// Invalidate session cache
					if s.cache != nil {
						s.cache.InvalidateSession(r.Context(), adminID.String())
					}
				}
			}
		}
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"message": "Logged out successfully"})
}

package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/enterprise/android-remote-access/server/internal/models"
)

// authMiddleware validates JWT tokens
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		admin, err := s.authenticateAdmin(r)
		if err != nil {
			s.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
			return
		}
		ctx := withAdmin(r.Context(), admin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authenticateAdmin resolves the administrator from a Bearer token or ?token= query (WebSocket).
func (s *Server) authenticateAdmin(r *http.Request) (*models.Administrator, error) {
	token := ""
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			return nil, fmt.Errorf("invalid authorization format")
		}
		token = tokenParts[1]
	}
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return nil, fmt.Errorf("missing authorization")
	}

	_, claims, err := s.jwtManager.ValidateAccessToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid or expired token")
	}

	adminID, err := s.jwtManager.GetAdminIDFromClaims(claims)
	if err != nil {
		return nil, fmt.Errorf("invalid token claims")
	}

	if s.db == nil {
		return nil, fmt.Errorf("database unavailable")
	}

	admin, err := s.db.GetAdministratorByID(r.Context(), adminID)
	if err != nil || admin == nil || !admin.IsActive {
		return nil, fmt.Errorf("admin not found or inactive")
	}
	return admin, nil
}

// Context keys
type contextKey string

const adminContextKey contextKey = "admin"

func withAdmin(ctx context.Context, admin *models.Administrator) context.Context {
	return context.WithValue(ctx, adminContextKey, admin)
}

func getAdmin(ctx context.Context) *models.Administrator {
	if admin, ok := ctx.Value(adminContextKey).(*models.Administrator); ok {
		return admin
	}
	return nil
}

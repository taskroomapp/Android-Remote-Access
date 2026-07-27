package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
)

var startTime = time.Now()

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	checks := map[string]string{
		"database": "ok",
		"redis":    "ok",
		"hub":      "ok",
	}

	if s.db == nil {
		checks["database"] = "error"
	}
	if s.cache == nil {
		checks["redis"] = "error"
	}
	if s.hub == nil {
		checks["hub"] = "error"
	}

	status := "healthy"
	for _, v := range checks {
		if v != "ok" {
			status = "degraded"
			break
		}
	}

	s.writeJSON(w, http.StatusOK, models.HealthResponse{
		Status:  status,
		Version: "1.0.0",
		Uptime:  time.Since(startTime),
		Checks:  checks,
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, code, message string) {
	s.writeJSON(w, status, models.ErrorResponse{
		Error: message,
		Code:  code,
	})
}

func getClientIP(r *http.Request) string {
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		return strings.Split(forwarded, ",")[0]
	}
	return r.RemoteAddr
}

func mediaContentType(fileType, fileName string) string {
	lower := strings.ToLower(fileName)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".ogg"):
		return "audio/ogg"
	case strings.HasSuffix(lower, ".mp4"), strings.HasSuffix(lower, ".m4a"):
		return "audio/mp4"
	case fileType == "image":
		return "image/jpeg"
	case fileType == "audio":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

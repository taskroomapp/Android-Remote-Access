package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/enterprise/android-remote-access/server/internal/security"
)

const adminCKX1Header = "X-CKX1-Session"

func (s *Server) ckx1BodyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionToken := strings.TrimSpace(r.Header.Get(adminCKX1Header))
		if sessionToken == "" {
			s.writeError(w, http.StatusUnauthorized, "CKX1_REQUIRED", "CKX1 session required")
			return
		}
		if s.adminCKX1 == nil {
			s.writeError(w, http.StatusServiceUnavailable, "CKX1_UNAVAILABLE", "CKX1 not configured")
			return
		}
		sess := s.adminCKX1.Get(sessionToken)
		if sess == nil || !sess.Ready() {
			s.writeError(w, http.StatusUnauthorized, "CKX1_NOT_READY", "CKX1 session not ready")
			return
		}
		admin := getAdmin(r.Context())
		if admin == nil || admin.ID != sess.AdminID {
			s.writeError(w, http.StatusForbidden, "CKX1_MISMATCH", "CKX1 session mismatch")
			return
		}

		if r.Body != nil && r.ContentLength != 0 {
			raw, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
			_ = r.Body.Close()
			if err != nil {
				s.writeError(w, http.StatusBadRequest, "BAD_BODY", "failed to read body")
				return
			}
			trimmed := bytes.TrimSpace(raw)
			if len(trimmed) > 0 && trimmed[0] == '{' {
				var frame map[string]interface{}
				if err := json.Unmarshal(trimmed, &frame); err == nil {
					if t, _ := frame["type"].(string); t == "enc" {
						plain, err := sess.OpenAdmin(frame)
						if err != nil {
							s.writeError(w, http.StatusBadRequest, "CKX1_DECRYPT", "CKX1 decrypt failed: "+err.Error())
							return
						}
						r.Body = io.NopCloser(bytes.NewReader(plain))
						r.ContentLength = int64(len(plain))
						r.Header.Set("Content-Type", "application/json")
					} else {
						r.Body = io.NopCloser(bytes.NewReader(raw))
						r.ContentLength = int64(len(raw))
					}
				} else {
					r.Body = io.NopCloser(bytes.NewReader(raw))
					r.ContentLength = int64(len(raw))
				}
			} else if len(raw) > 0 {
				r.Body = io.NopCloser(bytes.NewReader(raw))
				r.ContentLength = int64(len(raw))
			}
		}

		if skipCKX1ResponseWrap(r) {
			next.ServeHTTP(w, r)
			return
		}

		cw := &ckx1ResponseWriter{ResponseWriter: w, sess: sess, status: http.StatusOK}
		next.ServeHTTP(cw, r)
		_ = cw.flush()
	})
}

func skipCKX1ResponseWrap(r *http.Request) bool {
	p := r.URL.Path
	if strings.Contains(p, "/files/download/") || strings.Contains(p, "/files/stream") {
		return true
	}
	if strings.Contains(p, "/media/file/") {
		return true
	}
	if strings.HasSuffix(p, "/export") {
		return true
	}
	return false
}

type ckx1ResponseWriter struct {
	http.ResponseWriter
	sess   *security.AdminCKX1Session
	buf    bytes.Buffer
	status int
}

func (w *ckx1ResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *ckx1ResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *ckx1ResponseWriter) flush() error {
	body := w.buf.Bytes()
	ct := w.Header().Get("Content-Type")
	if len(bytes.TrimSpace(body)) == 0 || !strings.Contains(ct, "application/json") {
		w.ResponseWriter.WriteHeader(w.status)
		_, err := w.ResponseWriter.Write(body)
		return err
	}
	frame, err := w.sess.SealAdmin(body)
	if err != nil {
		http.Error(w.ResponseWriter, "CKX1 encrypt failed", http.StatusInternalServerError)
		return err
	}
	out, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-CKX1-Encrypted", "1")
	w.ResponseWriter.WriteHeader(w.status)
	_, err = w.ResponseWriter.Write(out)
	return err
}

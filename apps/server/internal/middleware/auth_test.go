package middleware

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func authRequest(t *testing.T, secret string, password string, token string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.GET("/ok", RequireAuth(secret, password), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder.Code
}

func TestRequireAuthBindsTokenToPassword(t *testing.T) {
	secret := "jwt-secret"
	password := "admin-password"

	token, err := NewToken(secret, password, time.Hour)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if code := authRequest(t, secret, password, token); code != http.StatusOK {
		t.Fatalf("valid token status = %d, want %d", code, http.StatusOK)
	}
	if code := authRequest(t, secret, "changed-password", token); code != http.StatusUnauthorized {
		t.Fatalf("token after password change status = %d, want %d", code, http.StatusUnauthorized)
	}
	if code := authRequest(t, "other-secret", password, token); code != http.StatusUnauthorized {
		t.Fatalf("token after secret change status = %d, want %d", code, http.StatusUnauthorized)
	}
}

func TestRequireAuthRejectsExpiredAndLegacyTokens(t *testing.T) {
	secret := "jwt-secret"
	password := "admin-password"

	expired, err := NewToken(secret, password, -time.Hour)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}
	if code := authRequest(t, secret, password, expired); code != http.StatusUnauthorized {
		t.Fatalf("expired token status = %d, want %d", code, http.StatusUnauthorized)
	}

	payload, err := json.Marshal(tokenPayload{Exp: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatalf("marshal legacy payload: %v", err)
	}
	part := base64.RawURLEncoding.EncodeToString(payload)
	legacy := part + "." + sign(part, secret)
	if code := authRequest(t, secret, password, legacy); code != http.StatusUnauthorized {
		t.Fatalf("legacy token status = %d, want %d", code, http.StatusUnauthorized)
	}
	if code := authRequest(t, secret, password, "not-a-token"); code != http.StatusUnauthorized {
		t.Fatalf("malformed token status = %d, want %d", code, http.StatusUnauthorized)
	}
}

func cookieAuthRequest(t *testing.T, method string, path string, token string, csrfCookie string, csrfHeader string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)

	secret := "0123456789abcdef0123456789abcdef"
	password := "admin-password"
	router := gin.New()
	router.Handle(method, path, RequireAuth(secret, password), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: AdminTokenCookieName, Value: token})
	if csrfCookie != "" {
		req.AddCookie(&http.Cookie{Name: AdminCSRFCookieName, Value: csrfCookie})
	}
	if csrfHeader != "" {
		req.Header.Set("X-CSRF-Token", csrfHeader)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder.Code
}

func TestRequireAuthCookieNeedsCSRFForUnsafeMethods(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	password := "admin-password"
	token, err := NewToken(secret, password, time.Hour)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}

	if code := cookieAuthRequest(t, http.MethodGet, "/ok", token, "", ""); code != http.StatusOK {
		t.Fatalf("GET with cookie status = %d, want %d", code, http.StatusOK)
	}
	if code := cookieAuthRequest(t, http.MethodPost, "/ok", token, "", ""); code != http.StatusForbidden {
		t.Fatalf("POST without CSRF status = %d, want %d", code, http.StatusForbidden)
	}
	if code := cookieAuthRequest(t, http.MethodPost, "/ok", token, "csrf-token", "csrf-token"); code != http.StatusOK {
		t.Fatalf("POST with CSRF status = %d, want %d", code, http.StatusOK)
	}
	if code := cookieAuthRequest(t, http.MethodPost, "/ok", token, "csrf-token", "wrong"); code != http.StatusForbidden {
		t.Fatalf("POST with wrong CSRF status = %d, want %d", code, http.StatusForbidden)
	}
}

func TestRequireAuthBearerSkipsCSRF(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	password := "admin-password"
	token, err := NewToken(secret, password, time.Hour)
	if err != nil {
		t.Fatalf("NewToken: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/ok", RequireAuth(secret, password), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodPost, "/ok", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("Bearer POST status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

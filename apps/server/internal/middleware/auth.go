package middleware

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	AdminTokenCookieName = "portfolio_admin_token"
	AdminCSRFCookieName  = "portfolio_admin_csrf"
)

type tokenPayload struct {
	Exp int64  `json:"exp"`
	Pwd string `json:"pwd"`
}

func NewToken(secret string, adminPassword string, ttl time.Duration) (string, error) {
	payload := tokenPayload{
		Exp: time.Now().Add(ttl).Unix(),
		Pwd: passwordFingerprint(secret, adminPassword),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	payloadPart := base64.RawURLEncoding.EncodeToString(raw)
	signature := sign(payloadPart, secret)
	return payloadPart + "." + signature, nil
}

func NewCSRFToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

func RequireAuth(secret string, adminPassword string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, fromCookie := requestToken(c)
		if token == "" || !validToken(token, secret, adminPassword) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "unauthorized",
			})
			return
		}
		if fromCookie && requiresCSRF(c.Request.Method) && !validCSRF(c) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"success": false,
				"error":   "csrf token invalid",
			})
			return
		}
		c.Next()
	}
}

func requestToken(c *gin.Context) (string, bool) {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(header, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
		if token != "" {
			return token, false
		}
	}
	cookie, err := c.Cookie(AdminTokenCookieName)
	if err == nil && cookie != "" {
		return cookie, true
	}
	return "", false
}

func requiresCSRF(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func validCSRF(c *gin.Context) bool {
	cookie, err := c.Cookie(AdminCSRFCookieName)
	if err != nil || cookie == "" {
		return false
	}
	header := strings.TrimSpace(c.GetHeader("X-CSRF-Token"))
	return header != "" && hmac.Equal([]byte(header), []byte(cookie))
}

func validToken(token string, secret string, adminPassword string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}

	expected := sign(parts[0], secret)
	if !hmac.Equal([]byte(expected), []byte(parts[1])) {
		return false
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	var payload tokenPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	if !hmac.Equal([]byte(payload.Pwd), []byte(passwordFingerprint(secret, adminPassword))) {
		return false
	}
	return payload.Exp > time.Now().Unix()
}

func passwordFingerprint(secret string, adminPassword string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("admin-password:"))
	mac.Write([]byte(adminPassword))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func sign(payload string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	mac.Write([]byte(strconv.Itoa(len(secret))))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

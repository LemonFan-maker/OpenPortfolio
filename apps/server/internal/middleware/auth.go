package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

func RequireAuth(secret string, adminPassword string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token := strings.TrimPrefix(header, "Bearer ")
		if token == "" || !validToken(token, secret, adminPassword) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "unauthorized",
			})
			return
		}
		c.Next()
	}
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

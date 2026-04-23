package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

const accessTokenPrefix = "ldv1"

type accessTokenClaims struct {
	Subject     string   `json:"sub"`
	DisplayName string   `json:"name"`
	Email       string   `json:"email"`
	Role        Role     `json:"role"`
	TenantID    string   `json:"tenantId,omitempty"`
	StoreIDs    []string `json:"storeIds,omitempty"`
	IssuedAt    int64    `json:"iat"`
	ExpiresAt   int64    `json:"exp"`
}

type HMACTokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewHMACTokenManager(secret string, ttl time.Duration) *HMACTokenManager {
	return &HMACTokenManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (manager *HMACTokenManager) Issue(user User) (SessionView, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(manager.ttl)
	claims := accessTokenClaims{
		Subject:     user.ID,
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Role:        user.Role,
		TenantID:    user.TenantID,
		StoreIDs:    append([]string{}, user.StoreIDs...),
		IssuedAt:    now.Unix(),
		ExpiresAt:   expiresAt.Unix(),
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return SessionView{}, err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := manager.sign(encodedPayload)
	token := strings.Join([]string{accessTokenPrefix, encodedPayload, signature}, ".")

	return SessionView{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresAt:   expiresAt,
	}, nil
}

func (manager *HMACTokenManager) Parse(token string) (Principal, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != accessTokenPrefix {
		return Principal{}, ErrUnauthorized
	}

	encodedPayload := parts[1]
	signature := parts[2]
	expectedSignature := manager.sign(encodedPayload)

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return Principal{}, ErrUnauthorized
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return Principal{}, ErrUnauthorized
	}

	var claims accessTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return Principal{}, ErrUnauthorized
	}

	if claims.Subject == "" || claims.Email == "" || !IsValidRole(claims.Role) {
		return Principal{}, ErrUnauthorized
	}

	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	if time.Now().UTC().After(expiresAt) {
		return Principal{}, ErrUnauthorized
	}

	return Principal{
		UserID:      claims.Subject,
		DisplayName: claims.DisplayName,
		Email:       claims.Email,
		Role:        claims.Role,
		TenantID:    claims.TenantID,
		StoreIDs:    append([]string{}, claims.StoreIDs...),
		ExpiresAt:   expiresAt,
	}, nil
}

func (manager *HMACTokenManager) sign(encodedPayload string) string {
	mac := hmac.New(sha256.New, manager.secret)
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

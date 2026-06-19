package automation

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// contextTokenPrefix identifica o context token do Omni Chat (Fase 2). Versionado
// (ctxv1) para permitir rotacao de formato sem ambiguidade no Parse.
const contextTokenPrefix = "ctxv1"

// ErrInvalidContextToken e' o unico erro retornado por Parse. Generico de
// proposito (formato/assinatura/expiracao) — nao vaza o motivo da rejeicao para
// quem chama (n8n), o que evitaria oraculos de timing/forja.
var ErrInvalidContextToken = errors.New("automation: context token invalido")

// ContextScope e o escopo multi-tenant carregado pelo context token, retornado
// por Parse. Toda query escopada da tool de dados usa ESTES campos (vindos do
// token assinado), NUNCA do query/body do n8n.
type ContextScope struct {
	AccountID string
	TenantID  string
	StoreIDs  []string
	UserID    string
	Role      string
}

// contextTokenClaims e o payload assinado do context token (HMAC-SHA256).
// Espelha o formato de auth/tokens.go (base64.RawURLEncoding(json) + assinatura).
type contextTokenClaims struct {
	AccountID string   `json:"accountId"`
	TenantID  string   `json:"tenantId,omitempty"`
	StoreIDs  []string `json:"storeIds,omitempty"`
	UserID    string   `json:"userId"`
	Role      string   `json:"role"`
	IssuedAt  int64    `json:"iat"`
	ExpiresAt int64    `json:"exp"`
}

// ContextTokenManager emite e valida o context token opaco do Omni Chat. Mesmo
// padrao do HMACTokenManager de auth: secret HMAC + TTL curto. TTL curto (300s)
// limita a janela de uso de um token vazado entre a emissao no /ask e a chamada
// da tool pelo n8n.
type ContextTokenManager struct {
	secret []byte
	ttl    time.Duration
}

// NewContextTokenManager cria o manager. secret vazio => Issue/Parse falham (o
// chamador trata a ausencia de secret como "nao configurado").
func NewContextTokenManager(secret []byte, ttl time.Duration) *ContextTokenManager {
	return &ContextTokenManager{
		secret: append([]byte{}, secret...),
		ttl:    ttl,
	}
}

// Issue assina um ContextScope e devolve o token opaco "ctxv1.payload.sig".
// O escopo vem do principal autenticado no /ask, nunca do body.
func (m *ContextTokenManager) Issue(scope ContextScope) (string, error) {
	if len(m.secret) == 0 {
		return "", ErrInvalidContextToken
	}
	now := time.Now().UTC()
	claims := contextTokenClaims{
		AccountID: scope.AccountID,
		TenantID:  scope.TenantID,
		StoreIDs:  append([]string{}, scope.StoreIDs...),
		UserID:    scope.UserID,
		Role:      scope.Role,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(m.ttl).Unix(),
	}

	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := m.sign(encodedPayload)
	return strings.Join([]string{contextTokenPrefix, encodedPayload, signature}, "."), nil
}

// Parse valida o token (3 partes + prefixo + assinatura constant-time + nao
// expirado) e devolve o ContextScope. Qualquer falha vira ErrInvalidContextToken.
func (m *ContextTokenManager) Parse(token string) (ContextScope, error) {
	if len(m.secret) == 0 {
		return ContextScope{}, ErrInvalidContextToken
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != contextTokenPrefix {
		return ContextScope{}, ErrInvalidContextToken
	}

	encodedPayload := parts[1]
	signature := parts[2]
	expectedSignature := m.sign(encodedPayload)
	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return ContextScope{}, ErrInvalidContextToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		return ContextScope{}, ErrInvalidContextToken
	}

	var claims contextTokenClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return ContextScope{}, ErrInvalidContextToken
	}
	if claims.AccountID == "" {
		return ContextScope{}, ErrInvalidContextToken
	}

	expiresAt := time.Unix(claims.ExpiresAt, 0).UTC()
	if time.Now().UTC().After(expiresAt) {
		return ContextScope{}, ErrInvalidContextToken
	}

	return ContextScope{
		AccountID: claims.AccountID,
		TenantID:  claims.TenantID,
		StoreIDs:  append([]string{}, claims.StoreIDs...),
		UserID:    claims.UserID,
		Role:      claims.Role,
	}, nil
}

// sign computa a assinatura HMAC-SHA256 do payload codificado.
func (m *ContextTokenManager) sign(encodedPayload string) string {
	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(encodedPayload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

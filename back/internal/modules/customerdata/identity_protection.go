package customerdata

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"
)

const identityKeyVersion = "v1"

type environmentIdentityProtector struct {
	aead    cipher.AEAD
	hmacKey []byte
}

// NewEnvironmentIdentityProtector exige duas chaves independentes de 32 bytes,
// base64: CUSTOMER_DATA_ENCRYPTION_KEY e CUSTOMER_DATA_HMAC_KEY.
func NewEnvironmentIdentityProtector() (IdentityProtector, error) {
	encryptionKey, err := decodeKey("CUSTOMER_DATA_ENCRYPTION_KEY")
	if err != nil {
		return nil, err
	}
	hmacKey, err := decodeKey("CUSTOMER_DATA_HMAC_KEY")
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("customer data: encryption key: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("customer data: gcm: %w", err)
	}
	return &environmentIdentityProtector{aead: aead, hmacKey: hmacKey}, nil
}

func decodeKey(name string) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || len(key) != 32 {
		return nil, fmt.Errorf("%s must be base64 for exactly 32 bytes", name)
	}
	return key, nil
}

func (p *environmentIdentityProtector) Protect(scope Scope, input IdentityInput) (ProtectedIdentity, error) {
	normalized, err := normalizeIdentityValue(input.Kind, input.Value)
	if err != nil {
		return ProtectedIdentity{}, err
	}
	issuer := strings.ToLower(strings.TrimSpace(input.Issuer))
	if issuer == "" || len(issuer) > 120 {
		return ProtectedIdentity{}, invalid("issuer", "invalid_length")
	}
	ciphertext, err := p.encrypt([]byte(normalized), identityAAD(scope, input.Kind, issuer))
	if err != nil {
		return ProtectedIdentity{}, err
	}
	mac := hmac.New(sha256.New, p.hmacKey)
	_, _ = mac.Write([]byte(strings.ToLower(strings.TrimSpace(input.Kind))))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(issuer))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(normalized))
	occurredAt := time.Now().UTC()
	if input.OccurredAt != nil && !input.OccurredAt.IsZero() {
		occurredAt = input.OccurredAt.UTC()
	}
	status := strings.TrimSpace(input.VerificationStatus)
	if status == "" {
		status = "unverified"
	}
	return ProtectedIdentity{
		Kind: strings.ToLower(strings.TrimSpace(input.Kind)), Issuer: issuer,
		Ciphertext: ciphertext, Fingerprint: hex.EncodeToString(mac.Sum(nil)),
		KeyVersion: identityKeyVersion, MaskedValue: maskIdentity(input.Kind, normalized),
		VerificationStatus: status, VerificationMethod: strings.TrimSpace(input.VerificationMethod),
		SourceRefType: strings.TrimSpace(input.SourceRefType), SourceRefID: strings.TrimSpace(input.SourceRefID),
		Metadata: input.Metadata, OccurredAt: occurredAt, IdempotencyKey: strings.TrimSpace(input.IdempotencyKey),
	}, nil
}

func (p *environmentIdentityProtector) ProtectContent(scope Scope, plaintext string) (string, string, error) {
	value := strings.TrimSpace(plaintext)
	if value == "" {
		return "", "", nil
	}
	out, err := p.encrypt([]byte(value), []byte(scope.AccountID+"\x00"+scope.ClientAccountID+"\x00offline"))
	return out, identityKeyVersion, err
}

func (p *environmentIdentityProtector) RevealContent(scope Scope, ciphertext, keyVersion string) (string, error) {
	if keyVersion != identityKeyVersion {
		return "", ErrIdentityProtectionUnavailable
	}
	raw, err := base64.RawStdEncoding.DecodeString(ciphertext)
	if err != nil || len(raw) < p.aead.NonceSize() {
		return "", ErrIdentityProtectionUnavailable
	}
	nonce := raw[:p.aead.NonceSize()]
	plaintext, err := p.aead.Open(nil, nonce, raw[p.aead.NonceSize():], []byte(scope.AccountID+"\x00"+scope.ClientAccountID+"\x00offline"))
	if err != nil {
		return "", ErrIdentityProtectionUnavailable
	}
	return string(plaintext), nil
}

func (p *environmentIdentityProtector) encrypt(plaintext, aad []byte) (string, error) {
	nonce := make([]byte, p.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := p.aead.Seal(nil, nonce, plaintext, aad)
	return base64.RawStdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func identityAAD(scope Scope, kind, issuer string) []byte {
	return []byte(scope.AccountID + "\x00" + scope.ClientAccountID + "\x00" + strings.ToLower(strings.TrimSpace(kind)) + "\x00" + issuer)
}

func normalizeIdentityValue(kind, value string) (string, error) {
	kind = strings.ToLower(strings.TrimSpace(kind))
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 {
		return "", invalid("value", "invalid_length")
	}
	switch kind {
	case "phone", "whatsapp":
		var b strings.Builder
		for i, r := range value {
			if unicode.IsDigit(r) || (r == '+' && i == 0) {
				b.WriteRune(r)
			}
		}
		value = b.String()
		if len(strings.TrimPrefix(value, "+")) < 8 || len(strings.TrimPrefix(value, "+")) > 15 {
			return "", invalid("value", "invalid_phone")
		}
	case "email":
		value = strings.ToLower(value)
		if !strings.Contains(value, "@") || strings.HasPrefix(value, "@") || strings.HasSuffix(value, "@") {
			return "", invalid("value", "invalid_email")
		}
	case "document":
		var b strings.Builder
		for _, r := range value {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				b.WriteRune(unicode.ToUpper(r))
			}
		}
		value = b.String()
		if len(value) < 5 || allSame(value) {
			return "", invalid("value", "generic_or_invalid_document")
		}
	case "instagram", "erp_customer", "site_visitor", "other":
		// Escopo forte é completado por kind + issuer + client.
	default:
		return "", invalid("kind", "unsupported")
	}
	return value, nil
}

func allSame(value string) bool {
	if value == "" {
		return true
	}
	runes := []rune(value)
	for _, current := range runes[1:] {
		if current != runes[0] {
			return false
		}
	}
	return true
}

func maskIdentity(kind, value string) string {
	if strings.EqualFold(kind, "email") {
		parts := strings.SplitN(value, "@", 2)
		if len(parts) == 2 {
			return "***@" + parts[1]
		}
	}
	runes := []rune(value)
	if len(runes) <= 4 {
		return "***"
	}
	return "***" + string(runes[len(runes)-4:])
}

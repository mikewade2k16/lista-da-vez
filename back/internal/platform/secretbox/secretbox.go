// Package secretbox cifra segredos EM REPOUSO (AES-256-GCM) para toda a plataforma.
//
// Por que existe (OMNI-F3.1): calendar/secrets.go ja entrega {set,last4} — o contrato
// de SAIDA esta certo e e o modelo seguido aqui (Status/Mask). O que falta la e a
// cifragem: calendar.ai_secrets.api_key e `text not null default ”` e a key e gravada
// CRUA. Mascaramento e de SAIDA, nao e cifragem — dai o pacote nascer em platform/ e
// nao dentro de um modulo (o 2o consumidor, o calendario, ja e previsivel).
//
// Regras inegociaveis:
//   - Nonce ALEATORIO de 12 bytes por Encrypt (nonce reusado em GCM quebra a cifra).
//   - Prefixo "v1:" obrigatorio: sem prefixo nao ha rotacao, so migracao manual.
//   - A key CRUA nunca e logada, nunca volta em erro, nunca volta ao front (so Status).
//   - Sem default/fallback de chave: perder a chave = perder os segredos, e um default
//     silencioso vira producao cifrada com chave de dev.
//
// ARMADILHA: ciphertext NAO e chave de busca. Como o nonce e aleatorio, o mesmo texto
// gera ciphertext diferente a cada Encrypt — nunca indexar nem usar em WHERE.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

// EnvKey e a env var que carrega a chave mestra: 32 bytes CRUS em base64
// (`openssl rand -base64 32`, 44 chars). Sem ela a api NAO SOBE (fail-fast).
const EnvKey = "OMNI_SECRETS_KEY"

// versionPrefix marca o esquema de cifragem do ciphertext. Decrypt despacha por
// prefixo, entao uma "v2:" futura convive lendo os "v1:" ja gravados.
const versionPrefix = "v1:"

// keySize e o tamanho da chave AES-256 em bytes.
const keySize = 32

var (
	// ErrKeySize: a chave nao tem exatamente 32 bytes. Nunca inclui a chave no erro.
	ErrKeySize = errors.New("secretbox: chave deve ter exatamente 32 bytes (AES-256)")

	// ErrKeyMissing: OMNI_SECRETS_KEY ausente ou vazia. Fail-fast no boot.
	ErrKeyMissing = fmt.Errorf("secretbox: %s ausente — a api nao sobe sem a chave de segredos", EnvKey)

	// ErrUnknownVersion: ciphertext com prefixo desconhecido (ou sem prefixo). Nao
	// tenta adivinhar o esquema: devolve erro.
	ErrUnknownVersion = errors.New("secretbox: versao de ciphertext desconhecida")

	// ErrMalformed: ciphertext corrompido/curto demais para conter nonce + dados.
	ErrMalformed = errors.New("secretbox: ciphertext malformado")

	// ErrDecrypt: autenticacao GCM falhou (chave errada ou dado adulterado). Falha
	// explicita — jamais devolve lixo como se fosse o segredo.
	ErrDecrypt = errors.New("secretbox: falha ao decifrar (chave errada ou dado adulterado)")
)

// Status e o status MASCARADO de um segredo (write-only), espelhando
// calendar.KeyStatus. Set = ha segredo gravado; Last4 = os ultimos 4 caracteres,
// para o usuario reconhecer a chave. O segredo cru nunca sai.
type Status struct {
	Set   bool   `json:"set"`
	Last4 string `json:"last4"`
}

// Box cifra e decifra segredos com uma chave AES-256-GCM. Seguro para uso
// concorrente: cipher.AEAD e stateless e o nonce e sorteado por operacao.
type Box struct {
	aead cipher.AEAD
}

// New constroi um Box a partir da chave CRUA de 32 bytes. Chave de tamanho
// diferente => ErrKeySize. A chave nunca e logada nem ecoada no erro.
func New(key []byte) (*Box, error) {
	if len(key) != keySize {
		return nil, ErrKeySize
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		// Nao interpola err com a chave; aes.NewCipher so falha por tamanho.
		return nil, ErrKeySize
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: inicializar GCM: %w", err)
	}
	return &Box{aead: aead}, nil
}

// FromEnv le OMNI_SECRETS_KEY (32 bytes em base64) e constroi o Box. Ausente ou
// invalida => erro, para o boot morrer com mensagem clara nomeando a env (canonico
// §13.2). NUNCA ha default nem fallback: um default silencioso cifraria producao com
// chave de dev, e perder a chave = perder todo segredo cifrado.
func FromEnv() (*Box, error) {
	raw := strings.TrimSpace(os.Getenv(EnvKey))
	if raw == "" {
		return nil, ErrKeyMissing
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		// O erro do base64 nao carrega o conteudo decodificado; ainda assim nao o
		// envolvemos, para nao arriscar vazar fragmento da chave em log.
		return nil, fmt.Errorf("secretbox: %s nao e base64 valido (use `openssl rand -base64 32`)", EnvKey)
	}
	if len(key) != keySize {
		return nil, fmt.Errorf("secretbox: %s decodificou %d bytes, esperado %d (use `openssl rand -base64 32`)",
			EnvKey, len(key), keySize)
	}
	return New(key)
}

// Encrypt cifra o texto e devolve "v1:" + base64std(nonce||ciphertext). O nonce e
// sorteado de crypto/rand a CADA chamada — reusar nonce em GCM quebra a cifra e
// vazaria o plaintext. Consequencia: duas cifragens do mesmo texto sao diferentes.
func (b *Box) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secretbox: sortear nonce: %w", err)
	}
	sealed := b.aead.Seal(nil, nonce, []byte(plaintext), nil)
	return versionPrefix + base64.StdEncoding.EncodeToString(append(nonce, sealed...)), nil
}

// Decrypt reverte Encrypt. Despacha por prefixo: desconhecido => ErrUnknownVersion.
// Chave errada ou dado adulterado => ErrDecrypt (nunca devolve lixo silenciosamente).
// O plaintext devolvido NUNCA pode ser logado pelo caller.
func (b *Box) Decrypt(encoded string) (string, error) {
	encoded = strings.TrimSpace(encoded)
	if !strings.HasPrefix(encoded, versionPrefix) {
		return "", ErrUnknownVersion
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(encoded, versionPrefix))
	if err != nil {
		return "", ErrMalformed
	}
	nonceSize := b.aead.NonceSize()
	if len(blob) < nonceSize+1 {
		return "", ErrMalformed
	}
	plaintext, err := b.aead.Open(nil, blob[:nonceSize], blob[nonceSize:], nil)
	if err != nil {
		return "", ErrDecrypt
	}
	return string(plaintext), nil
}

// Mask converte o segredo cru no status mascarado, com a MESMA regra do calendario
// (calendar/secrets.go:44): vazio => {false,""}; <=4 chars => o valor todo em Last4.
// E funcao de pacote (nao metodo) porque mascarar nao depende de chave.
func Mask(plaintext string) Status {
	plaintext = strings.TrimSpace(plaintext)
	if plaintext == "" {
		return Status{Set: false, Last4: ""}
	}
	last4 := plaintext
	if len(plaintext) > 4 {
		last4 = plaintext[len(plaintext)-4:]
	}
	return Status{Set: true, Last4: last4}
}

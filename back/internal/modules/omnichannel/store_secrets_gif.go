package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Persistencia da chave GLOBAL de GIF (F12 C5). Modelo do calendar/store_secrets.go: a key
// global vive em core.platform_settings (chave 'omnichannel_gif'), NAO por conta. O Store e
// persistencia BURRA: grava/le o JSON como esta. A cifragem/decifragem da apiKey (secretbox,
// prefixo v1:) e responsabilidade do GifService (secrets_gif.go) — aqui a apiKey ja chega
// cifrada. Sem migration: a tabela existe (0160_core_platform_settings.sql).

// gifSettingsKey e a chave do registro em core.platform_settings.
const gifSettingsKey = "omnichannel_gif"

// gifSecretConfig e o registro persistido. APIKey e o CIPHERTEXT (v1:...) ou "" (sem chave) —
// o Store nunca ve a chave crua. NUNCA serializado de volta ao front (o front recebe o status
// mascarado GifSettingsStatus).
type gifSecretConfig struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseUrl"`
	APIKey   string `json:"apiKey"`
}

// GetGifSecret le o registro global de GIF. Sem linha => zero value (provider/baseUrl/apiKey
// vazios), tratado como "sem chave" pelo service.
func (s *Store) GetGifSecret(ctx context.Context) (gifSecretConfig, error) {
	const q = `select config from core.platform_settings where key = $1`
	var out gifSecretConfig
	var raw json.RawMessage
	err := s.pool.QueryRow(ctx, q, gifSettingsKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, nil
	}
	if err != nil {
		return out, err
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out, nil
}

// PutGifSecret regrava o registro global inteiro (upsert). updatedBy = userID (uuid) ou "".
// A apiKey em cfg deve vir JA cifrada (o service e quem cifra).
func (s *Store) PutGifSecret(ctx context.Context, cfg gifSecretConfig, updatedBy string) error {
	body, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	const q = `
		insert into core.platform_settings (key, config, updated_at, updated_by)
		values ($1, $2::jsonb, now(), $3::uuid)
		on conflict (key) do update
		set config = excluded.config, updated_at = now(), updated_by = excluded.updated_by`
	_, err = s.pool.Exec(ctx, q, gifSettingsKey, body, gifNullUUID(updatedBy))
	return err
}

// gifNullUUID converte "" em NULL para a coluna updated_by (uuid nullable), evitando erro de
// cast de string vazia para uuid.
func gifNullUUID(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

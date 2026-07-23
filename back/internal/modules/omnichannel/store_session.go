package omnichannel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Persistencia do ciclo de sessao (instancias de WhatsApp: cadastro, default, telefone,
// credencial cifrada) e da credencial do provider para o webhook. Mesma regra da casa:
// TODA query filtra por account_id.

// sessionInstance e a linha da instancia usada pelo fluxo de sessao (subconjunto util,
// distinto do InstanceView servido ao front).
type sessionInstance struct {
	ID             string
	InstanceName   string
	DisplayName    *string
	PhoneNumber    *string
	Provider       string
	IsDefault      bool
	IsActive       bool
	CredentialsSet bool
}

// GetSessionInstance resolve a instancia pelo nome dentro da conta. Instancia de outra
// conta cai no filtro e volta pgx.ErrNoRows (o service traduz para 404).
func (s *Store) GetSessionInstance(ctx context.Context, accountID, instanceName string) (sessionInstance, error) {
	var i sessionInstance
	err := s.pool.QueryRow(ctx, `select id::text, instance_name, display_name, phone_number,
		provider, is_default, is_active, (credentials_ciphertext is not null)
		from messaging.whatsapp_instances
		where account_id = $1::uuid and instance_name = $2`,
		accountID, instanceName).
		Scan(&i.ID, &i.InstanceName, &i.DisplayName, &i.PhoneNumber, &i.Provider,
			&i.IsDefault, &i.IsActive, &i.CredentialsSet)
	return i, err
}

// GetSessionInstanceByRef resolve a instancia por id, senao por nome, senao a default/1a
// ativa da conta (mesma ordem/regra do ResolveInstanceForOps). Existe porque o front verbatim
// chama /status e /qrcode com instanceId (o id da instancia), enquanto connect/logout mandam o
// nome no body. Ambos vazios => a default/1a ativa (o inbox mostra a conexao da default quando
// nenhuma instancia esta selecionada). Sem correspondencia (conta sem instancia) => pgx.ErrNoRows
// (o service traduz para 404). Isolamento: filtra por account_id, entao instancia de outra conta
// nunca casa.
func (s *Store) GetSessionInstanceByRef(ctx context.Context, accountID, instanceID, instanceName string) (sessionInstance, error) {
	var i sessionInstance
	err := s.pool.QueryRow(ctx, `select id::text, instance_name, display_name, phone_number,
		provider, is_default, is_active, (credentials_ciphertext is not null)
		from messaging.whatsapp_instances
		where account_id = $1::uuid
			and ($2::text = '' or id = $2::uuid)
			and ($3::text = '' or instance_name = $3)
		order by is_default desc, is_active desc, instance_name
		limit 1`,
		accountID, strings.TrimSpace(instanceID), strings.TrimSpace(instanceName)).
		Scan(&i.ID, &i.InstanceName, &i.DisplayName, &i.PhoneNumber, &i.Provider,
			&i.IsDefault, &i.IsActive, &i.CredentialsSet)
	return i, err
}

// CreateInstance cadastra uma nova instancia. O par (account_id, instance_name) tem indice
// unico (0200) — nome repetido na conta volta violacao de unicidade. Devolve o id.
func (s *Store) CreateInstance(ctx context.Context, accountID, instanceName, displayName, provider, createdByUserID string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `insert into messaging.whatsapp_instances
		(account_id, instance_name, display_name, provider, created_by_user_id, responsible_user_id)
		values ($1::uuid, $2, nullif($3,''), $4, nullif($5,'')::uuid, nullif($5,'')::uuid)
		returning id::text`,
		accountID, instanceName, displayName, provider, createdByUserID).Scan(&id)
	return id, err
}

// PromoteDefault torna a instancia a default da conta (e desmarca as demais), numa
// transacao. Filtra por account TAMBEM (defesa em profundidade).
func (s *Store) PromoteDefault(ctx context.Context, accountID, instanceID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `update messaging.whatsapp_instances
		set is_default = false, updated_at = now()
		where account_id = $1::uuid and is_default = true`, accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update messaging.whatsapp_instances
		set is_default = true, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, instanceID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SetInstancePhone grava o numero resolvido apos conectar. A gravacao pode violar o indice
// unico parcial (account_id, phone_number) da 0201 — quem chama trata como ErrNumberInUse.
func (s *Store) SetInstancePhone(ctx context.Context, accountID, instanceID, phone string) error {
	_, err := s.pool.Exec(ctx, `update messaging.whatsapp_instances
		set phone_number = nullif($3,''), updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, instanceID, phone)
	return err
}

// ClearInstancePhone zera o numero no logout (a instancia continua cadastrada, so
// desconectada) — libera o numero para reconectar em outra instancia se for o caso.
func (s *Store) ClearInstancePhone(ctx context.Context, accountID, instanceID string) error {
	_, err := s.pool.Exec(ctx, `update messaging.whatsapp_instances
		set phone_number = null, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, instanceID)
	return err
}

// SetInstanceCredentials grava o ciphertext (ja cifrado pelo secretbox no service) da
// instancia. NUNCA recebe a chave crua — o service cifra antes. Filtra por account.
func (s *Store) SetInstanceCredentials(ctx context.Context, accountID, instanceID, ciphertext string) error {
	_, err := s.pool.Exec(ctx, `update messaging.whatsapp_instances
		set credentials_ciphertext = nullif($3,''), updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`, accountID, instanceID, ciphertext)
	return err
}

// FindInstanceUsingPhone diz qual OUTRA instancia da conta ja usa o numero (C6). Exclui a
// propria instancia (excludeInstanceID) para nao acusar colisao consigo mesma. found=false
// => numero livre.
func (s *Store) FindInstanceUsingPhone(ctx context.Context, accountID, phone, excludeInstanceID string) (string, bool, error) {
	var name string
	err := s.pool.QueryRow(ctx, `select instance_name from messaging.whatsapp_instances
		where account_id = $1::uuid and phone_number = $2 and id <> nullif($3,'')::uuid
		limit 1`, accountID, phone, excludeInstanceID).Scan(&name)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	default:
		return name, true, nil
	}
}

// FindProviderCredential resolve o material de credencial de (conta, provider) para o
// webhook: a primeira instancia ativa daquele provider com ciphertext preenchido, mais o
// provider_config (nao secreto). found=false => sem credencial (ex.: mock). O ciphertext
// so e DECIFRADO no service (secretbox), nunca aqui.
func (s *Store) FindProviderCredential(ctx context.Context, accountID, provider string) (ciphertext string, config map[string]string, found bool, err error) {
	return s.FindProviderCredentialForKey(ctx, accountID, provider, "")
}

// FindProviderCredentialForKey narrows a provider credential by its public
// callback key when the adapter can extract one before signature verification
// (Meta phone_number_id). Empty key preserves the legacy first-active behavior.
func (s *Store) FindProviderCredentialForKey(ctx context.Context, accountID, provider, instanceKey string) (ciphertext string, config map[string]string, found bool, err error) {
	var cipher *string
	var rawConfig []byte
	if provider == "instagram" {
		queryErr := s.pool.QueryRow(ctx, `select credentials_ciphertext, provider_config
			from messaging.instagram_accounts where account_id=$1::uuid and ($2='' or ig_user_id=$2) and is_active=true
			order by ig_user_id limit 1`, accountID, strings.TrimSpace(instanceKey)).Scan(&cipher, &rawConfig)
		switch {
		case errors.Is(queryErr, pgx.ErrNoRows):
			return "", nil, false, nil
		case queryErr != nil:
			return "", nil, false, queryErr
		}
		config = decodeStringMap(rawConfig)
		if cipher == nil {
			return "", config, false, nil
		}
		return *cipher, config, true, nil
	}
	queryErr := s.pool.QueryRow(ctx, `select credentials_ciphertext, provider_config
		from messaging.whatsapp_instances
		where account_id = $1::uuid and provider = $2 and is_active = true
		  and ($3 = '' or instance_name = $3 or provider_config->>'phoneNumberId' = $3)
		order by (credentials_ciphertext is not null) desc, is_default desc, instance_name
		limit 1`, accountID, provider, strings.TrimSpace(instanceKey)).Scan(&cipher, &rawConfig)
	switch {
	case errors.Is(queryErr, pgx.ErrNoRows):
		return "", nil, false, nil
	case queryErr != nil:
		return "", nil, false, queryErr
	}
	config = decodeStringMap(rawConfig)
	if cipher == nil {
		return "", config, false, nil
	}
	return *cipher, config, true, nil
}

func (s *Store) FindInstanceProviderByName(ctx context.Context, accountID, instanceName string) (string, bool, error) {
	var provider string
	err := s.pool.QueryRow(ctx, `select provider from messaging.whatsapp_instances
		where account_id=$1::uuid and instance_name=$2 and is_active=true limit 1`, accountID, strings.TrimSpace(instanceName)).Scan(&provider)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, err
	default:
		return strings.TrimSpace(provider), true, nil
	}
}

// decodeStringMap le um jsonb de config e mantem so os pares com valor string (o resto do
// provider_config nao interessa as Credentials). Config invalido => mapa vazio.
func decodeStringMap(raw []byte) map[string]string {
	out := map[string]string{}
	if len(raw) == 0 {
		return out
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return out
	}
	for k, v := range m {
		var s string
		if json.Unmarshal(v, &s) == nil {
			out[k] = s
		}
	}
	return out
}

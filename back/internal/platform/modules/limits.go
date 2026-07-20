package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// LimitReader resolve limites por conta a partir de core.account_modules.config (OMNI-F3.4).
// Sem migration: a coluna config jsonb ja existe (0100_core_schema.sql). Este pacote ja e o
// dono de core.account_modules (catalog_postgres.go), por isso o leitor mora aqui.
//
// F3 entrega o LEITOR e o erro. Quem APLICA e a F4 (max_whatsapp_numbers) e a F9
// (monthly_ai_runs) — cada uma chama Check antes de criar o recurso.
type LimitReader struct {
	pool *pgxpool.Pool
}

// NewLimitReader monta o leitor sobre o mesmo pool do catalogo.
func NewLimitReader(pool *pgxpool.Pool) *LimitReader {
	return &LimitReader{pool: pool}
}

// platformLimitsKey e a chave em core.platform_settings que guarda os defaults por modulo:
// { "<moduleID>": { "<limitKey>": N } }. Espelha o padrao calendar_ai_secrets/media_limits.
const platformLimitsKey = "module_limits"

// ErrLimitExceeded sinaliza estouro. Carrega o suficiente para uma resposta 409 ACIONAVEL
// (principio 5): a chave que estourou, o teto e onde o consumidor esta. Nunca falha silenciosa.
type ErrLimitExceeded struct {
	Key     string // ex.: "max_whatsapp_numbers"
	Limit   int64
	Current int64
}

func (e *ErrLimitExceeded) Error() string {
	return fmt.Sprintf("limite %q atingido: %d de %d", e.Key, e.Current, e.Limit)
}

// IsLimitExceeded ajuda o caller a mapear em 409 sem type assertion na mao.
func IsLimitExceeded(err error) bool {
	var e *ErrLimitExceeded
	return errors.As(err, &e)
}

// Limit e o resultado da resolucao. Set=false => SEM limite configurado (nem na conta nem
// no default): o recurso e ilimitado, e Check nunca estoura.
type Limit struct {
	Value  int64
	Set    bool
	Source string // "account" | "platform_default" | "" (nenhum)
}

// Resolve busca o teto de key para (accountID, moduleID). Ordem (a spec §F3.4):
//  1. core.account_modules.config->key da conta;
//  2. core.platform_settings[module_limits][moduleID][key];
//  3. ausente nos dois => sem limite (Set=false).
//
// accountID vem SEMPRE do Principal do caller — este leitor nao recebe nada de body.
func (r *LimitReader) Resolve(ctx context.Context, accountID, moduleID, key string) (Limit, error) {
	// 1) config da conta.
	var accountConfig []byte
	err := r.pool.QueryRow(ctx, `
		select config
		from core.account_modules
		where account_id = $1::uuid and module_id = $2
	`, accountID, moduleID).Scan(&accountConfig)
	switch {
	case err == nil:
		if v, ok := lookupInt(accountConfig, key); ok {
			return Limit{Value: v, Set: true, Source: "account"}, nil
		}
	case errors.Is(err, pgx.ErrNoRows):
		// conta nao tem a linha do modulo — cai para o default.
	default:
		return Limit{}, fmt.Errorf("ler account_modules.config: %w", err)
	}

	// 2) default de plataforma.
	var platformConfig []byte
	err = r.pool.QueryRow(ctx, `
		select config from core.platform_settings where key = $1
	`, platformLimitsKey).Scan(&platformConfig)
	switch {
	case err == nil:
		if v, ok := lookupModuleInt(platformConfig, moduleID, key); ok {
			return Limit{Value: v, Set: true, Source: "platform_default"}, nil
		}
	case errors.Is(err, pgx.ErrNoRows):
		// sem default configurado.
	default:
		return Limit{}, fmt.Errorf("ler platform_settings: %w", err)
	}

	// 3) ausente nos dois => sem limite.
	return Limit{Set: false}, nil
}

// Check resolve o limite e estoura se current JA alcancou o teto (novo recurso passaria).
// Sem limite configurado => nil (ilimitado). current e a contagem ATUAL do recurso.
func (r *LimitReader) Check(ctx context.Context, accountID, moduleID, key string, current int64) error {
	lim, err := r.Resolve(ctx, accountID, moduleID, key)
	if err != nil {
		return err
	}
	if !lim.Set {
		return nil
	}
	if current >= lim.Value {
		return &ErrLimitExceeded{Key: key, Limit: lim.Value, Current: current}
	}
	return nil
}

// lookupInt le config[key] como inteiro. jsonb numerico chega como float64 no Go.
func lookupInt(raw []byte, key string) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return 0, false
	}
	return asInt(m[key])
}

// lookupModuleInt le config[moduleID][key] (o default de plataforma e aninhado por modulo).
func lookupModuleInt(raw []byte, moduleID, key string) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var m map[string]map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return 0, false
	}
	mod, ok := m[moduleID]
	if !ok {
		return 0, false
	}
	return asInt(mod[key])
}

// asInt converte um numero JSON em int64. Rejeita valor negativo (limite negativo nao faz
// sentido — trata como "nao configurado" em vez de bloquear tudo).
func asInt(raw json.RawMessage) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var f float64
	if json.Unmarshal(raw, &f) != nil {
		return 0, false
	}
	if f < 0 {
		return 0, false
	}
	return int64(f), true
}

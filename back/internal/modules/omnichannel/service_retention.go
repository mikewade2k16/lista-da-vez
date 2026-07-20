package omnichannel

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ============================================================================
// F13 — Resolucao da politica de retencao (conta -> plataforma -> constante Go)
// ============================================================================
//
// Cada CLASSE de dado tem um prazo. O purge (retention_purge/store_retention) le esta
// politica para saber o cutoff de cada classe. Sem politica NAO ha purge, e sem purge a
// politica nao existe (canonico §9.2 F13).
//
// ARMADILHA VERIFICADA (OMNI-F13 C2) — NAO reusar o fallback "ausente = sem limite" da F3
// (platform/modules/limits.go). Para *limite* "ausente" = ilimitado; para *retencao*
// "ausente" significaria NUNCA APAGAR, que e o caso comum (a config quase sempre falta). A
// cadeia da retencao termina SEMPRE numa CONSTANTE Go — nunca em "sem limite".

// Classes de retencao (C1). As quatro com prazo + a varredura de orfaos de midia (sem prazo,
// e semanal por natureza — nao tem coluna de data propria).
const (
	classAudit        = "audit"
	classConversation = "conversation"
	classAIIO         = "ai_io"
	classEphemeral    = "ephemeral"
	classMediaOrphan  = "media_orphan"
)

// Defaults da plataforma — a ULTIMA linha de defesa (C2). Nunca "sem limite".
const (
	defaultAuditDays        = 365
	defaultConversationDays = 180
	defaultAIIODays         = 90
	defaultEphemeralDays    = 30
	// defaultMaxRetentionDays limita o valor aceito da config (1..maxDays). Fora da faixa =>
	// o valor e ignorado e cai para a proxima fonte (o writer de painel, F10, rejeita com 409;
	// aqui, na leitura, um valor invalido gravado por fora nao pode desligar a obrigacao legal).
	defaultMaxRetentionDays = 3650
)

// platformRetentionKey e a chave singleton em core.platform_settings com os defaults de
// plataforma. Shape: { "audit":365, "conversation":180, "ai_io":90, "ephemeral":30,
// "maxDays":3650 }. Padrao key-value de 0160 (precedente: calendar_ai_secrets, module_limits).
//
// Por conta, os prazos vivem em core.account_modules.config->retention_days (sub-chave
// { "retention_days": { "audit":365, ... } }, ao lado de max_whatsapp_numbers/monthly_ai_runs).
const platformRetentionKey = "omnichannel_retention"

// ClassRetention e o prazo resolvido de UMA classe + a procedencia (C2: valor honesto com
// origem, nunca default mudo). Source ∈ account | platform | default.
type ClassRetention struct {
	Days   int    `json:"days"`
	Source string `json:"source"`
}

// RetentionPolicy e a politica resolvida das quatro classes com prazo.
type RetentionPolicy struct {
	Audit        ClassRetention `json:"audit"`
	Conversation ClassRetention `json:"conversation"`
	AIIO         ClassRetention `json:"aiIo"`
	Ephemeral    ClassRetention `json:"ephemeral"`
}

// RetentionResolver le a politica de retencao por conta. So leitura — o writer de painel e da
// F10 (igual ao LimitReader da F3.4). accountID vem SEMPRE do Principal/enfileirador, nunca do body.
type RetentionResolver struct {
	pool *pgxpool.Pool
}

// NewRetentionResolver monta o resolver sobre o pool da plataforma.
func NewRetentionResolver(pool *pgxpool.Pool) *RetentionResolver {
	return &RetentionResolver{pool: pool}
}

// Resolve devolve a politica das quatro classes para a conta. A cadeia por classe:
// account_modules.config->retention_days->{classe} -> platform_settings[omnichannel_retention]
// -> constante Go. Valor fora de 1..maxDays em qualquer fonte e ignorado (cai para a proxima).
func (r *RetentionResolver) Resolve(ctx context.Context, accountID string) (RetentionPolicy, error) {
	accountVals, err := r.accountValues(ctx, accountID)
	if err != nil {
		return RetentionPolicy{}, err
	}
	platformVals, maxDays, err := r.platformValues(ctx)
	if err != nil {
		return RetentionPolicy{}, err
	}
	return buildPolicy(accountVals, platformVals, maxDays), nil
}

// accountValues le account_modules.config->retention_days como map classe->dias. Ausente/ilegivel
// => map vazio (cai para o default). Filtra por conta E modulo (defesa em profundidade).
func (r *RetentionResolver) accountValues(ctx context.Context, accountID string) (map[string]int, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `select config from core.account_modules
		where account_id = $1::uuid and module_id = 'omnichannel'`, accountID).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return map[string]int{}, nil
	case err != nil:
		return nil, err
	}
	var cfg struct {
		RetentionDays map[string]int `json:"retention_days"`
	}
	if json.Unmarshal(raw, &cfg) != nil {
		return map[string]int{}, nil
	}
	return cfg.RetentionDays, nil
}

// platformValues le core.platform_settings[omnichannel_retention] como map classe->dias + maxDays.
// Ausente/ilegivel => (map vazio, default de maxDays).
func (r *RetentionResolver) platformValues(ctx context.Context) (map[string]int, int, error) {
	var raw []byte
	err := r.pool.QueryRow(ctx, `select config from core.platform_settings
		where key = $1`, platformRetentionKey).Scan(&raw)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return map[string]int{}, defaultMaxRetentionDays, nil
	case err != nil:
		return nil, 0, err
	}
	vals := map[string]int{}
	if json.Unmarshal(raw, &vals) != nil {
		return map[string]int{}, defaultMaxRetentionDays, nil
	}
	maxDays := defaultMaxRetentionDays
	if v, ok := vals["maxDays"]; ok && v >= 1 {
		maxDays = v
	}
	return vals, maxDays, nil
}

// buildPolicy resolve as quatro classes. Funcao PURA (sem IO) — o nucleo testavel da cadeia.
func buildPolicy(account, platform map[string]int, maxDays int) RetentionPolicy {
	if maxDays < 1 {
		maxDays = defaultMaxRetentionDays
	}
	return RetentionPolicy{
		Audit:        pickDays(classAudit, account, platform, defaultAuditDays, maxDays),
		Conversation: pickDays(classConversation, account, platform, defaultConversationDays, maxDays),
		AIIO:         pickDays(classAIIO, account, platform, defaultAIIODays, maxDays),
		Ephemeral:    pickDays(classEphemeral, account, platform, defaultEphemeralDays, maxDays),
	}
}

// pickDays escolhe o prazo de uma classe: conta -> plataforma -> constante. So aceita valor em
// 1..maxDays (0/negativo/acima do teto = "nao configurado", nunca "desligar retencao").
func pickDays(class string, account, platform map[string]int, def, maxDays int) ClassRetention {
	if v, ok := account[class]; ok && validDays(v, maxDays) {
		return ClassRetention{Days: v, Source: "account"}
	}
	if v, ok := platform[class]; ok && validDays(v, maxDays) {
		return ClassRetention{Days: v, Source: "platform"}
	}
	return ClassRetention{Days: def, Source: "default"}
}

// validDays aceita apenas 1..maxDays. Desligar obrigacao legal por config nao e feature (C2).
func validDays(v, maxDays int) bool {
	return v >= 1 && v <= maxDays
}

package omnichannel

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Persistencia de contatos, config da conta e instancias. Mesma regra do
// store_postgres.go: TODA query filtra por account_id.

// ============================================================================
// Contatos
// ============================================================================

// contactCols sai na ordem esperada por scanContact. account_id::text volta como
// TenantID: o contrato do front exige `tenantId` obrigatorio no Contact (types:136) e o
// Omni mapeia tenantId -> account_id.
const contactCols = `ct.id::text, ct.account_id::text, ct.name, coalesce(ct.phone, ''), ct.avatar_url,
	ct.source, ct.created_at, ct.updated_at`

// lastConversationCols resolve o resumo da ultima conversa do contato como subqueries
// ESCALARES sobre a MESMA account (defesa em profundidade). O legado faz
// `conversations: { take: 1, orderBy: lastMessageAt desc }` e le 4 campos dela.
// `state` sai cru e vira `status` na projecao (projectStatus) — status nao e coluna.
const lastConversationCols = `,
	lc.id::text, lc.last_message_at, lc.channel, lc.state`

const lastConversationJoin = `
	left join lateral (
		select cv.id, cv.last_message_at, cv.channel, cv.state
		from messaging.conversations cv
		where cv.contact_id = ct.id and cv.account_id = ct.account_id
		order by cv.last_message_at desc
		limit 1
	) lc on true`

// contactRow e a linha crua (state ainda nao projetado).
type contactRow struct {
	ID                      string
	AccountID               string
	Name                    string
	Phone                   string
	AvatarURL               *string
	Source                  string
	CreatedAt               time.Time
	UpdatedAt               time.Time
	LastConversationID      *string
	LastConversationAt      *time.Time
	LastConversationChannel *string
	LastConversationState   *string
}

func scanContact(row rowScanner) (contactRow, error) {
	var c contactRow
	err := row.Scan(&c.ID, &c.AccountID, &c.Name, &c.Phone, &c.AvatarURL, &c.Source,
		&c.CreatedAt, &c.UpdatedAt, &c.LastConversationID, &c.LastConversationAt,
		&c.LastConversationChannel, &c.LastConversationState)
	return c, err
}

// ListContacts devolve os contatos da account (ordem do legado: updated_at, created_at).
func (s *Store) ListContacts(ctx context.Context, accountID string) ([]contactRow, error) {
	query := `select ` + contactCols + lastConversationCols + `
		from messaging.contacts ct` + lastConversationJoin + `
		where ct.account_id = $1::uuid
		  and not exists (select 1 from messaging.contact_suppressions suppression
		      where suppression.account_id=ct.account_id and suppression.contact_id=ct.id
		        and suppression.is_hidden=true)
		order by ct.updated_at desc, ct.created_at desc`

	rows, err := s.pool.Query(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]contactRow, 0)
	for rows.Next() {
		c, err := scanContact(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListVisibleContacts deriva o CRM exclusivamente de conversas visiveis ao ator.
func (s *Store) ListVisibleContacts(ctx context.Context, accountID string, scope VisibilityScope) ([]contactRow, error) {
	join := ` left join lateral (select cv.id,cv.last_message_at,cv.channel,cv.state
		from messaging.conversations cv where cv.contact_id=ct.id and cv.account_id=ct.account_id`
	args := []any{accountID}
	join, args = appendConversationVisibility(join, args, "cv", scope)
	join += s.historyVisibleConversationPredicate("cv")
	join += ` order by cv.last_message_at desc limit 1) lc on true`
	query := `select ` + contactCols + lastConversationCols + ` from messaging.contacts ct` + join + `
		where ct.account_id=$1::uuid and lc.id is not null
		and not exists (select 1 from messaging.contact_suppressions suppression
			where suppression.account_id=ct.account_id and suppression.contact_id=ct.id and suppression.is_hidden=true)
		order by ct.updated_at desc,ct.created_at desc`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]contactRow, 0)
	for rows.Next() {
		row, scanErr := scanContact(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *Store) GetVisibleContact(ctx context.Context, accountID, contactID string, scope VisibilityScope) (contactRow, error) {
	join := ` left join lateral (select cv.id,cv.last_message_at,cv.channel,cv.state
		from messaging.conversations cv where cv.contact_id=ct.id and cv.account_id=ct.account_id`
	args := []any{accountID}
	join, args = appendConversationVisibility(join, args, "cv", scope)
	join += s.historyVisibleConversationPredicate("cv")
	join += ` order by cv.last_message_at desc limit 1) lc on true`
	args = append(args, contactID)
	query := `select ` + contactCols + lastConversationCols + ` from messaging.contacts ct` + join +
		` where ct.account_id=$1::uuid and ct.id=$` + strconv.Itoa(len(args)) + `::uuid and lc.id is not null
		and not exists (select 1 from messaging.contact_suppressions suppression
			where suppression.account_id=ct.account_id and suppression.contact_id=ct.id and suppression.is_hidden=true)`
	return scanContact(s.pool.QueryRow(ctx, query, args...))
}

// GetContact devolve um contato da account. Contato de outra conta -> pgx.ErrNoRows -> 404.
func (s *Store) GetContact(ctx context.Context, accountID, id string) (contactRow, error) {
	query := `select ` + contactCols + lastConversationCols + `
		from messaging.contacts ct` + lastConversationJoin + `
		where ct.account_id = $1::uuid and ct.id = $2::uuid`
	return scanContact(s.pool.QueryRow(ctx, query, accountID, id))
}

// UpsertContact grava o contato por (account_id, phone) — a chave natural do legado
// (Prisma faz `contact.upsert` no par tenantId+phone). Devolve o id gravado.
//
// O update sobrescreve avatar_url com o valor resolvido, INCLUSIVE quando ele e null —
// e o comportamento do legado (contacts.ts:906-910 seta avatarUrl: resolvedAvatarUrl
// direto, sem coalesce). Portado como esta: e o contrato da tela, nao um bug de escopo.
func (s *Store) UpsertContact(ctx context.Context, accountID, name, phone, source string, avatarURL *string) (string, error) {
	var id string
	err := s.pool.QueryRow(ctx, `insert into messaging.contacts
		(account_id, name, phone, avatar_url, source)
		values ($1::uuid, $2, $3, $4, $5)
		on conflict (account_id, phone) where phone is not null and phone <> '' do update
			set name = excluded.name,
				avatar_url = excluded.avatar_url,
				source = excluded.source,
				updated_at = now()
		returning id::text`,
		accountID, name, phone, avatarURL, source).Scan(&id)
	return id, err
}

// LinkConversationsByPhone amarra ao contato TODAS as conversas da account com aquele
// telefone (o legado faz o mesmo updateMany apos o upsert: contacts.ts:920-928).
func (s *Store) LinkConversationsByPhone(ctx context.Context, accountID, phone, contactID string) error {
	_, err := s.pool.Exec(ctx, `update messaging.conversations
		set contact_id = $3::uuid, updated_at = now()
		where account_id = $1::uuid and contact_phone = $2`,
		accountID, phone, contactID)
	return err
}

func (s *Store) LinkVisibleConversationsByPhone(ctx context.Context, accountID, phone, contactID string, scope VisibilityScope) error {
	query := `update messaging.conversations c set contact_id=$3::uuid,updated_at=now()
		where c.account_id=$1::uuid and c.contact_phone=$2`
	args := []any{accountID, phone, contactID}
	query, args = appendConversationVisibility(query, args, "c", scope)
	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

// UpdateConversationContact carimba os dados do contato na conversa indicada
// (contacts.ts:932-946). Filtra por account TAMBEM — defesa em profundidade.
func (s *Store) UpdateConversationContact(ctx context.Context, accountID, conversationID, contactID, name, phone string, avatarURL *string) error {
	_, err := s.pool.Exec(ctx, `update messaging.conversations
		set contact_id = $3::uuid, contact_name = $4, contact_phone = $5,
			contact_avatar_url = $6, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, conversationID, contactID, name, phone, avatarURL)
	return err
}

func (s *Store) UpdateVisibleConversationContact(ctx context.Context, accountID, conversationID, contactID, name, phone string, avatarURL *string, scope VisibilityScope) error {
	query := `update messaging.conversations c
		set contact_id=$3::uuid,contact_name=$4,contact_phone=$5,contact_avatar_url=$6,updated_at=now()
		where c.account_id=$1::uuid and c.id=$2::uuid`
	args := []any{accountID, conversationID, contactID, name, phone, avatarURL}
	query, args = appendConversationVisibility(query, args, "c", scope)
	command, err := s.pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// UpdateContact aplica o PATCH. Os tres campos ja chegam resolvidos do service (nome
// normalizado, telefone so digitos, avatar como esta ou limpo).
func (s *Store) UpdateContact(ctx context.Context, accountID, id, name, phone string, avatarURL *string) error {
	_, err := s.pool.Exec(ctx, `update messaging.contacts
		set name = $3, phone = $4, avatar_url = $5, updated_at = now()
		where account_id = $1::uuid and id = $2::uuid`,
		accountID, id, name, phone, avatarURL)
	return err
}

// SyncConversationPhone propaga a troca de telefone do contato para as conversas dele
// (o legado faz o mesmo updateMany apos o PATCH quando o telefone muda).
func (s *Store) SyncConversationPhone(ctx context.Context, accountID, contactID, phone string) error {
	_, err := s.pool.Exec(ctx, `update messaging.conversations
		set contact_phone = $3, updated_at = now()
		where account_id = $1::uuid and contact_id = $2::uuid`,
		accountID, contactID, phone)
	return err
}

func (s *Store) SyncVisibleConversationPhone(ctx context.Context, accountID, contactID, phone string, scope VisibilityScope) error {
	query := `update messaging.conversations c set contact_phone=$3,updated_at=now()
		where c.account_id=$1::uuid and c.contact_id=$2::uuid`
	args := []any{accountID, contactID, phone}
	query, args = appendConversationVisibility(query, args, "c", scope)
	_, err := s.pool.Exec(ctx, query, args...)
	return err
}

// FindContactIDByPhone resolve o contato pelo telefone na account. Vazio = nao existe.
func (s *Store) FindContactIDByPhone(ctx context.Context, accountID, phone string) (string, error) {
	rows, err := s.pool.Query(ctx, `select id::text from messaging.contacts
		where account_id = $1::uuid and phone = $2 limit 1`, accountID, phone)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if !rows.Next() {
		return "", rows.Err()
	}
	var id string
	if err := rows.Scan(&id); err != nil {
		return "", err
	}
	return id, rows.Err()
}

// ============================================================================
// Config do atendimento (messaging.account_config)
// ============================================================================

// GetAccountConfig le retention_days/max_upload_mb da conta. Linha ausente = os
// defaults do legado (15/500), os MESMOS do default da coluna — nao ha segunda verdade:
// a coluna e a fonte, e o fallback so cobre a linha que ainda nao foi criada.
func (s *Store) GetAccountConfig(ctx context.Context, accountID string) (retentionDays, maxUploadMb int, err error) {
	rows, err := s.pool.Query(ctx, `select retention_days, max_upload_mb
		from messaging.account_config where account_id = $1::uuid`, accountID)
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()
	if !rows.Next() {
		return defaultRetentionDays, defaultMaxUploadMb, rows.Err()
	}
	if err := rows.Scan(&retentionDays, &maxUploadMb); err != nil {
		return 0, 0, err
	}
	return retentionDays, maxUploadMb, rows.Err()
}

// UpsertAccountConfig grava os limites da conta (PATCH /account).
func (s *Store) UpsertAccountConfig(ctx context.Context, accountID string, retentionDays, maxUploadMb int) error {
	_, err := s.pool.Exec(ctx, `insert into messaging.account_config
		(account_id, retention_days, max_upload_mb)
		values ($1::uuid, $2, $3)
		on conflict (account_id) do update
			set retention_days = excluded.retention_days,
				max_upload_mb = excluded.max_upload_mb,
				updated_at = now()`,
		accountID, retentionDays, maxUploadMb)
	return err
}

// ============================================================================
// Conta (core.accounts) e limites (core.account_modules.config)
// ============================================================================

// accountRow e o cabecalho da conta — vem de core.accounts, NUNCA duplicado em
// messaging.* (spec C4: id/slug/name tem uma fonte so).
type accountRow struct {
	ID        string
	Slug      string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// GetAccount le a conta ativa. Conta inexistente -> pgx.ErrNoRows -> 404.
func (s *Store) GetAccount(ctx context.Context, accountID string) (accountRow, error) {
	var a accountRow
	err := s.pool.QueryRow(ctx, `select id::text, slug, name, created_at, updated_at
		from core.accounts where id = $1::uuid`, accountID).
		Scan(&a.ID, &a.Slug, &a.Name, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

// GetModuleLimits le max_whatsapp_numbers/max_users de core.account_modules.config
// (jsonb; a coluna ja existe desde 0100_core_schema.sql:120 — sem migration nova,
// canonico §5.3). Chave ausente => cai no default da plataforma
// (core.platform_settings, key 'omnichannel_limits'); ausente la tambem => o default
// do codigo. E um FALLBACK EM CASCATA, nao um valor cravado: quem manda e o banco.
func (s *Store) GetModuleLimits(ctx context.Context, accountID string) (maxChannels, maxUsers int, err error) {
	err = s.pool.QueryRow(ctx, `
		select
			coalesce(
				(am.config ->> 'max_whatsapp_numbers')::int,
				(ps.config ->> 'max_whatsapp_numbers')::int,
				$2::int),
			coalesce(
				(am.config ->> 'max_users')::int,
				(ps.config ->> 'max_users')::int,
				$3::int)
		from core.account_modules am
		left join core.platform_settings ps on ps.key = 'omnichannel_limits'
		where am.account_id = $1::uuid and am.module_id = 'omnichannel'`,
		accountID, defaultMaxChannels, defaultMaxUsers).Scan(&maxChannels, &maxUsers)
	// Sem linha em account_modules (ex.: platform_admin, que tem bypass do gate e pode
	// operar numa conta que nao habilitou o modulo) => defaults. Nao e erro.
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultMaxChannels, defaultMaxUsers, nil
	}
	return maxChannels, maxUsers, err
}

// SetModuleMaxChannels atualiza somente a chave do teto de WhatsApp, preservando
// qualquer outra configuracao JSON do modulo. A account deve ter o modulo habilitado.
func (s *Store) SetModuleMaxChannels(ctx context.Context, accountID string, maxChannels int) error {
	tag, err := s.pool.Exec(ctx, `
		update core.account_modules
		set config = jsonb_set(
			coalesce(config, '{}'::jsonb),
			'{max_whatsapp_numbers}',
			to_jsonb($2::int),
			true)
		where account_id = $1::uuid and module_id = 'omnichannel'`,
		accountID, maxChannels)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// CountAccountUsers conta os membros ativos da conta (currentUsers do TenantSettings).
func (s *Store) CountAccountUsers(ctx context.Context, accountID string) (int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `select count(*) from core.account_users
		where account_id = $1::uuid and is_active = true`, accountID).Scan(&total)
	return total, err
}

// ListAssignableUsers devolve os membros ativos da conta (o array `users` do
// WhatsAppInstanceManagementResponse). O papel legado (ADMIN/SUPERVISOR/AGENT/VIEWER)
// e derivado no service — aqui so sai o papel do Omni.
func (s *Store) ListAssignableUsers(ctx context.Context, accountID string) ([]AssignableUserView, error) {
	rows, err := s.pool.Query(ctx, `select u.id::text, u.email, u.display_name
		from core.account_users au
		join core.users u on u.id = au.user_id
		where au.account_id = $1::uuid and au.is_active = true
		order by u.display_name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]AssignableUserView, 0)
	for rows.Next() {
		var u AssignableUserView
		if err := rows.Scan(&u.ID, &u.Email, &u.Name); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// ============================================================================
// Instancias de WhatsApp
// ============================================================================

// instanceCols sai na ordem esperada por scanInstance. has_evolution_api_key deriva de
// credentials_ciphertext is not null — NAO de env global: env global nao sobrevive a D-A
// (multi-provider por conta/numero). A coluna fica vazia ate a F3/F4 cifrarem.
//
// provider_config vem por ULTIMO: userScopePolicy/assignedUserIds sao persistidos ali
// (chaves proprias do jsonb) pela gestao de instancia — o front tem controles reais para
// os dois (seletor de politica + PUT .../users), entao nao sao mais constantes fixas.
const instanceCols = `wi.id::text, wi.account_id::text, wi.instance_name, wi.provider, wi.display_name,
	wi.access_policy, wi.access_revision,
	wi.phone_number, wi.queue_label, wi.responsible_user_id::text, ru.display_name, ru.email,
	wi.is_default, wi.is_active, (wi.credentials_ciphertext is not null),
	wi.history_visible_from, wi.history_reset_revision,
	wi.created_at, wi.updated_at, wi.provider_config`

func scanInstance(row rowScanner) (InstanceView, error) {
	var i InstanceView
	var config []byte
	err := row.Scan(&i.ID, &i.TenantID, &i.InstanceName, &i.Provider, &i.DisplayName,
		&i.AccessPolicy, &i.AccessRevision, &i.PhoneNumber, &i.QueueLabel,
		&i.ResponsibleUserID, &i.ResponsibleUserName, &i.ResponsibleUserMail,
		&i.IsDefault, &i.IsActive, &i.HasEvolutionAPIKey, &i.HistoryVisibleFrom,
		&i.HistoryResetRevision, &i.CreatedAt, &i.UpdatedAt, &config)
	// userScopePolicy/assignedUserIds saem de provider_config (chaves proprias), com
	// fallback para os defaults do legado quando ausentes. Ver parseInstanceScope.
	i.UserScopePolicy, i.AssignedUserIDs = parseInstanceScope(config)
	return i, err
}

// InstanceFilter filtra a listagem de instancias.
type InstanceFilter struct {
	// ActiveOnly restringe as instancias ativas (o /access do legado so lista ativas).
	ActiveOnly bool
	// ResponsibleUserID aplica o filtro de acesso do A2: quando preenchido, so saem as
	// instancias sem responsavel ou cujo responsavel e este usuario. Vazio = sem filtro
	// (admin ve todas).
	ResponsibleUserID string
	// IDs restringe a leitura ao conjunto autorizado pelo resolver P1B. Slice vazio
	// significa nenhuma instancia; nil mantem o uso interno sem filtro.
	IDs []string
}

// ListInstances devolve as instancias da account.
func (s *Store) ListInstances(ctx context.Context, accountID string, f InstanceFilter) ([]InstanceView, error) {
	query := `select ` + instanceCols + `
		from messaging.whatsapp_instances wi
		left join core.users ru on ru.id = wi.responsible_user_id
		where wi.account_id = $1::uuid`
	args := []any{accountID}

	if f.ActiveOnly {
		query += " and wi.is_active = true"
	}
	if f.IDs != nil {
		args = append(args, f.IDs)
		query += " and wi.id=any($" + strconv.Itoa(len(args)) + "::uuid[])"
	}
	if strings.TrimSpace(f.ResponsibleUserID) != "" {
		args = append(args, strings.TrimSpace(f.ResponsibleUserID))
		// A2 corrigido: nao-admin ve as instancias sem dono + as das quais e responsavel.
		query += " and (wi.responsible_user_id is null or wi.responsible_user_id = $" +
			strconv.Itoa(len(args)) + "::uuid)"
	}
	query += " order by wi.is_default desc, wi.instance_name"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]InstanceView, 0)
	for rows.Next() {
		i, err := scanInstance(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// CountActiveInstances conta os canais ativos da conta (currentChannels).
func (s *Store) CountActiveInstances(ctx context.Context, accountID string) (int, error) {
	var total int
	err := s.pool.QueryRow(ctx, `select count(*) from messaging.whatsapp_instances
		where account_id = $1::uuid and is_active = true`, accountID).Scan(&total)
	return total, err
}

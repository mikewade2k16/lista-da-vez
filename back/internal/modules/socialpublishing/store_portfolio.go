package socialpublishing

import (
	"context"
	"fmt"
	"time"
)

const platformPublishingScopeQuery = `
	select client.id::text, client.name
	from core.accounts client
	where client.is_active = true
	  and client.is_agency = false
	  and exists (
		select 1
		from core.account_modules enabled_module
		where enabled_module.account_id = client.id
		  and enabled_module.module_id = 'social_publishing'
		  and enabled_module.enabled = true
	  )
	order by lower(client.name), client.id`

const accountPublishingScopeQuery = `
	with current_account as (
		select account.id, account.organization_id, account.is_agency
		from core.accounts account
		where account.id = $1::uuid
		  and account.is_active = true
	)
	select current_account.is_agency, client.id::text, client.name
	from current_account
	left join core.accounts client
	  on client.is_active = true
	 and client.is_agency = false
	 and (
		(
			current_account.is_agency = true
			and current_account.organization_id is not null
			and client.organization_id = current_account.organization_id
		)
		or (
			current_account.is_agency = false
			and client.id = current_account.id
		)
	 )
	 and (
		exists (
			select 1
			from core.organization_users organization_member
			where organization_member.user_id = $2::uuid
			  and organization_member.organization_id = client.organization_id
			  and organization_member.org_role = 'agency_owner'
		)
		or exists (
			select 1
			from core.account_users account_member
			where account_member.user_id = $2::uuid
			  and account_member.account_id = client.id
			  and account_member.is_active = true
		)
	 )
	 and exists (
		select 1
		from core.account_modules enabled_module
		where enabled_module.account_id = client.id
		  and enabled_module.module_id = 'social_publishing'
		  and enabled_module.enabled = true
	 )
	order by lower(client.name) nulls last, client.id`

// portfolioAggregateQuery executa uma unica leitura agregada para todas as
// contas previamente resolvidas pelo PublishingScope. As restricoes de conta
// ativa, conta-cliente e modulo habilitado sao repetidas como defesa em
// profundidade. Nenhuma coluna de segredo/conexao tecnica entra na projecao.
const portfolioAggregateQuery = `
	select
		account.id::text,
		account.name,
		connected_connection.id is not null as connected,
		coalesce(connected_connection.username, ''),
		count(post.id) filter (where post.status = 'draft')::bigint,
		count(post.id) filter (where post.status = 'scheduled')::bigint,
		count(post.id) filter (where post.status = 'publishing')::bigint,
		count(post.id) filter (where post.status = 'published')::bigint,
		count(post.id) filter (where post.status = 'failed')::bigint,
		coalesce(sum(analytics.views), 0)::bigint,
		coalesce(sum(analytics.reach), 0)::bigint,
		coalesce(sum(analytics.total_interactions), 0)::bigint,
		coalesce(sum(analytics.likes), 0)::bigint,
		coalesce(sum(analytics.comments), 0)::bigint,
		coalesce(sum(analytics.saved), 0)::bigint,
		coalesce(sum(analytics.shares), 0)::bigint,
		max(analytics.captured_at),
		min(post.scheduled_for) filter (
			where post.status = 'scheduled'
			  and post.scheduled_for >= $2::timestamptz
		)
	from core.accounts account
	left join social_publishing.connections connected_connection
	  on connected_connection.account_id = account.id
	 and connected_connection.status = 'connected'
	left join social_publishing.posts post
	  on post.account_id = account.id
	left join social_publishing.post_analytics analytics
	  on analytics.account_id = account.id
	 and analytics.post_id = post.id
	where account.id = any($1::text[]::uuid[])
	  and account.is_active = true
	  and account.is_agency = false
	  and exists (
		select 1
		from core.account_modules enabled_module
		where enabled_module.account_id = account.id
		  and enabled_module.module_id = 'social_publishing'
		  and enabled_module.enabled = true
	  )
	group by account.id, account.name, connected_connection.id, connected_connection.username
	order by lower(account.name), account.id`

func (s *Store) PublishingScope(
	ctx context.Context,
	accountID, userID string,
	platformAdmin bool,
) (PublishingScope, error) {
	if platformAdmin {
		return s.platformPublishingScope(ctx)
	}
	return s.accountPublishingScope(ctx, accountID, userID)
}

func (s *Store) platformPublishingScope(ctx context.Context) (PublishingScope, error) {
	rows, err := s.pool.Query(ctx, platformPublishingScopeQuery)
	if err != nil {
		return PublishingScope{}, fmt.Errorf("social publishing: listar escopo global: %w", err)
	}
	defer rows.Close()

	scope := PublishingScope{
		CanSelect: true,
		Clients:   []PublishingClient{},
	}
	for rows.Next() {
		var client PublishingClient
		if err := rows.Scan(&client.ID, &client.Name); err != nil {
			return PublishingScope{}, fmt.Errorf("social publishing: ler escopo global: %w", err)
		}
		scope.Clients = append(scope.Clients, client)
	}
	if err := rows.Err(); err != nil {
		return PublishingScope{}, fmt.Errorf("social publishing: iterar escopo global: %w", err)
	}
	return scope, nil
}

func (s *Store) accountPublishingScope(
	ctx context.Context,
	accountID, userID string,
) (PublishingScope, error) {
	rows, err := s.pool.Query(ctx, accountPublishingScopeQuery, accountID, userID)
	if err != nil {
		return PublishingScope{}, fmt.Errorf("social publishing: resolver escopo da conta: %w", err)
	}
	defer rows.Close()

	scope := PublishingScope{Clients: []PublishingClient{}}
	foundAccount := false
	for rows.Next() {
		var (
			isAgency   bool
			clientID   *string
			clientName *string
		)
		if err := rows.Scan(&isAgency, &clientID, &clientName); err != nil {
			return PublishingScope{}, fmt.Errorf("social publishing: ler escopo da conta: %w", err)
		}
		foundAccount = true
		scope.CanSelect = isAgency
		if !isAgency {
			scope.LockedClientID = accountID
		}
		if clientID != nil && clientName != nil {
			scope.Clients = append(scope.Clients, PublishingClient{
				ID:   *clientID,
				Name: *clientName,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return PublishingScope{}, fmt.Errorf("social publishing: iterar escopo da conta: %w", err)
	}
	if !foundAccount {
		return PublishingScope{}, ErrNotFound
	}
	return scope, nil
}

func (s *Store) ListPortfolio(
	ctx context.Context,
	accountIDs []string,
	now time.Time,
) ([]portfolioClientRecord, error) {
	if len(accountIDs) == 0 {
		return []portfolioClientRecord{}, nil
	}
	rows, err := s.pool.Query(ctx, portfolioAggregateQuery, accountIDs, now)
	if err != nil {
		return nil, fmt.Errorf("social publishing: consultar portfolio: %w", err)
	}
	defer rows.Close()

	records := make([]portfolioClientRecord, 0, len(accountIDs))
	for rows.Next() {
		var record portfolioClientRecord
		if err := rows.Scan(
			&record.Client.AccountID,
			&record.Client.AccountName,
			&record.Client.Connected,
			&record.Client.Username,
			&record.Client.Draft,
			&record.Client.Scheduled,
			&record.Client.Publishing,
			&record.Client.Published,
			&record.Client.Failed,
			&record.Views,
			&record.Client.Reach,
			&record.Client.TotalInteractions,
			&record.Likes,
			&record.Comments,
			&record.Saved,
			&record.Shares,
			&record.CapturedAt,
			&record.Client.NextScheduledFor,
		); err != nil {
			return nil, fmt.Errorf("social publishing: ler portfolio: %w", err)
		}
		normalizePortfolioTimes(&record)
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("social publishing: iterar portfolio: %w", err)
	}
	return records, nil
}

func normalizePortfolioTimes(record *portfolioClientRecord) {
	if record.CapturedAt != nil {
		capturedAt := record.CapturedAt.UTC()
		record.CapturedAt = &capturedAt
	}
	if record.Client.NextScheduledFor != nil {
		scheduledFor := record.Client.NextScheduledFor.UTC()
		record.Client.NextScheduledFor = &scheduledFor
	}
}

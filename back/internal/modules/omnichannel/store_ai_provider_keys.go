package omnichannel

import "context"

func (s *Store) AgentActiveProvider(ctx context.Context, accountID, agentID string) (string, error) {
	var provider string
	err := s.pool.QueryRow(ctx, `select coalesce(v.provider, '')
		from messaging.ai_agents a
		left join messaging.ai_agent_versions v
		  on v.account_id=a.account_id and v.agent_id=a.id and v.id=a.active_version_id
		where a.account_id=$1::uuid and a.id=$2::uuid`, accountID, agentID).Scan(&provider)
	return provider, err
}

func (s *Store) UpdateAgentProviderKeys(ctx context.Context, accountID, agentID, ciphertext, last4 string) (agentRow, error) {
	return scanAgent(s.pool.QueryRow(ctx, `update messaging.ai_agents set
		provider_key_ciphertext=$3, provider_key_last4=$4, updated_at=now()
		where account_id=$1::uuid and id=$2::uuid returning `+agentCols,
		accountID, agentID, ciphertext, last4))
}

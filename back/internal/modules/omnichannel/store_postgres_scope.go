package omnichannel

import "context"

// ============================================================================
// F7 — Escopo de instancia CORRIGIDO (spec OMNI-F7 C3) + reconciliacao do
// assigned_to_id (a coluna que o front verbatim le).
// ============================================================================
//
// O BUG do legado (whatsapp-instances.ts:681-683): o ternario
// `isTenantAdmin || active.length <= 1 ? active : active` devolve o MESMO valor nos dois
// ramos — o filtro por responsible_user_id NUNCA rodou e todo usuario ve todas as
// instancias. O indice (tenantId, responsibleUserId) existe e nao serve a ninguem.
//
// Aqui o filtro roda de verdade, no REPOSITORIO (defesa em profundidade, principio 2), com a
// intencao legivel reconstruida do proprio codigo morto (o guard `<= 1` evita trancar todos
// fora quando so ha um numero). DIVERGE DE PROPOSITO do legado — registrado em docs/LEGADO.md
// e no AGENT.md. O gate de fila (F8, queue_members) se soma a este com AND no service.

// instanceScopeRow e a tupla lida por AccessibleScopeKeys: o nome (= instance_scope_key) e
// se o usuario e o responsavel pela instancia.
type instanceScopeRow struct {
	Name          string
	IsResponsible bool
}

// AccessibleScopeKeys resolve os instance_scope_key que o ator alcanca, conforme a tabela do
// C3 corrigido. Devolve (keys, unrestricted): unrestricted=true => sem filtro (admin ve todas
// as ativas); keys vazio com unrestricted=false => o ator nao ve NADA (fail-close — nunca cai
// em "ve tudo"). Uma unica query (sem N+1).
//
//	| Ator                                   | Instancias visiveis                     |
//	| admin da conta / platform_admin        | todas as ativas (unrestricted)          |
//	| qualquer usuario, conta com <= 1 ativa | a unica ativa                           |
//	| demais                                 | ativas com responsible_user_id = ator   |
func (s *Store) AccessibleScopeKeys(ctx context.Context, accountID, userID string, isAdmin bool) (keys []string, unrestricted bool, err error) {
	if isAdmin {
		return nil, true, nil
	}
	rows, err := s.pool.Query(ctx, `select instance_name,
			(responsible_user_id is not null and responsible_user_id = $2::uuid) as is_responsible
		from messaging.whatsapp_instances
		where account_id = $1::uuid and is_active = true
		order by instance_name`, accountID, userID)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()

	all := make([]string, 0)
	mine := make([]string, 0)
	for rows.Next() {
		var r instanceScopeRow
		if err := rows.Scan(&r.Name, &r.IsResponsible); err != nil {
			return nil, false, err
		}
		all = append(all, r.Name)
		if r.IsResponsible {
			mine = append(mine, r.Name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	// Guard `<= 1`: conta com no maximo uma instancia ativa nao tranca ninguem fora — todos
	// veem a unica ativa (0 ou 1 nome). Reconstroi a intencao do codigo morto do legado.
	if len(all) <= 1 {
		return all, false, nil
	}
	return mine, false, nil
}

// SyncAssignedToID reconcilia a coluna LEGADA assigned_to_id (texto, servida ao front
// verbatim como `assignedToId`) com a coluna AUTORITATIVA da maquina de estados
// assigned_user_id (uuid). A migration 0200 deixou explicito que "quem reconcilia e a
// F7/F8 via maquina de estados": a F8 (ApplyTransition) grava so assigned_user_id, entao a
// F7 espelha o valor no assigned_to_id apos assign/unassign para o inbox mostrar o dono.
//
// NAO e escrita de state/status (risco 4): assigned_to_id e projecao de exibicao, nao o
// ciclo de vida. assigned_user_id null => assigned_to_id null (desatribuicao limpa os dois).
func (s *Store) SyncAssignedToID(ctx context.Context, accountID, convID string) error {
	_, err := s.pool.Exec(ctx, `update messaging.conversations
		set assigned_to_id = assigned_user_id::text, updated_at = now()
		where id = $1::uuid and account_id = $2::uuid`, convID, accountID)
	return err
}

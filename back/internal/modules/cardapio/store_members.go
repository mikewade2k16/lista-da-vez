package cardapio

import "context"

// cardapioMemberQuery: vinculo ATIVO do usuario com uma account ATIVA que tem o
// modulo cardapio habilitado. Base do escopo multi-cliente (usuario membro de
// mais de um cliente); a permissao fina continua no gate por account.
const cardapioMemberQuery = `
	from core.account_users au
	join core.accounts a on a.id = au.account_id and a.is_active = true
	join core.account_modules am
		on am.account_id = au.account_id and am.module_id = 'cardapio' and am.enabled = true
	where au.user_id = $1::uuid and au.is_active = true`

// IsCardapioMember confirma o vinculo do usuario com a account (cardapio ligado).
func (s *Store) IsCardapioMember(ctx context.Context, accountID, userID string) (bool, error) {
	const q = `select exists (select 1 ` + cardapioMemberQuery + ` and au.account_id = $2::uuid)`
	var ok bool
	if err := s.pool.QueryRow(ctx, q, userID, accountID).Scan(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

// CardapioMemberAccounts lista as accounts (cardapio ligado) em que o usuario e membro.
func (s *Store) CardapioMemberAccounts(ctx context.Context, userID string) ([]string, error) {
	const q = `select au.account_id::text ` + cardapioMemberQuery + ` order by a.name`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

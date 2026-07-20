package omnichannel

import (
	"context"
	"errors"
)

// ============================================================================
// F8 — Persistencia de setores e filas (schema messaging.*)
// ============================================================================
//
// REGRA DA CASA, sem excecao: TODA query filtra por account_id (defesa em profundidade,
// principio 2). IDs string + cast no SQL ($1::uuid). Recurso de outra conta cai no filtro
// e volta pgx.ErrNoRows -> o service traduz para ErrNotFound (404, nunca 403).

const departmentCols = `id::text, slug, name, is_default, is_active, created_at, updated_at`

func scanDepartment(row rowScanner) (DepartmentView, error) {
	var d DepartmentView
	err := row.Scan(&d.ID, &d.Slug, &d.Name, &d.IsDefault, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

// CreateDepartment insere um setor. Quando isDefault, limpa o default anterior na MESMA
// transacao antes de inserir (o indice parcial messaging_departments_default_uk so permite
// um default por conta; sem a limpeza o segundo default falharia).
func (s *Store) CreateDepartment(ctx context.Context, accountID, slug, name string, isDefault bool) (DepartmentView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DepartmentView{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if isDefault {
		if _, err := tx.Exec(ctx, `update messaging.departments set is_default = false, updated_at = now()
			where account_id = $1::uuid and is_default`, accountID); err != nil {
			return DepartmentView{}, err
		}
	}
	d, err := scanDepartment(tx.QueryRow(ctx, `insert into messaging.departments
		(account_id, slug, name, is_default) values ($1::uuid, $2, $3, $4)
		returning `+departmentCols, accountID, slug, name, isDefault))
	if err != nil {
		return DepartmentView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DepartmentView{}, err
	}
	return d, nil
}

// ListDepartments devolve todos os setores da conta (ativos e inativos — a F10 gerencia
// ambos), ordenados por nome.
func (s *Store) ListDepartments(ctx context.Context, accountID string) ([]DepartmentView, error) {
	rows, err := s.pool.Query(ctx, `select `+departmentCols+`
		from messaging.departments where account_id = $1::uuid order by name`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DepartmentView, 0)
	for rows.Next() {
		d, err := scanDepartment(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// UpdateDepartment aplica o patch parcial. isDefault=true limpa o default anterior na mesma
// tx. Setor de outra conta => ErrNoRows -> ErrNotFound.
func (s *Store) UpdateDepartment(ctx context.Context, accountID, id string, patch DepartmentPatch) (DepartmentView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DepartmentView{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if patch.IsDefault != nil && *patch.IsDefault {
		if _, err := tx.Exec(ctx, `update messaging.departments set is_default = false, updated_at = now()
			where account_id = $1::uuid and is_default and id <> $2::uuid`, accountID, id); err != nil {
			return DepartmentView{}, err
		}
	}
	d, err := scanDepartment(tx.QueryRow(ctx, `update messaging.departments set
		name = coalesce($3, name),
		is_default = coalesce($4, is_default),
		is_active = coalesce($5, is_active),
		updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		returning `+departmentCols, accountID, id, patch.Name, patch.IsDefault, patch.IsActive))
	if err != nil {
		return DepartmentView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return DepartmentView{}, err
	}
	return d, nil
}

// SoftDeleteDepartment desativa o setor (is_active=false). NAO apaga: conversas na fila do
// setor continuam visiveis (principio 3, DELETE = soft na spec). Setor de outra conta => 404.
func (s *Store) SoftDeleteDepartment(ctx context.Context, accountID, id string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.departments set is_active = false, is_default = false,
		updated_at = now() where account_id = $1::uuid and id = $2::uuid`, accountID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// departmentExists valida que o setor e da conta (usado antes de criar fila). so id ativo?
// Nao: a fila pode nascer sob um setor inativo em migracao; a spec so exige "da conta".
func (s *Store) departmentExists(ctx context.Context, accountID, departmentID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from messaging.departments
		where account_id = $1::uuid and id = $2::uuid)`, accountID, departmentID).Scan(&ok)
	return ok, err
}

// ============================================================================
// Filas
// ============================================================================

const queueCols = `id::text, department_id::text, slug, name, is_default, is_active, created_at, updated_at`

func scanQueue(row rowScanner) (QueueView, error) {
	var q QueueView
	err := row.Scan(&q.ID, &q.DepartmentID, &q.Slug, &q.Name, &q.IsDefault, &q.IsActive,
		&q.CreatedAt, &q.UpdatedAt)
	return q, err
}

// CreateQueue insere uma fila sob um setor da conta (o departamento e validado no service).
// isDefault limpa o default anterior do MESMO setor na mesma tx.
func (s *Store) CreateQueue(ctx context.Context, accountID, departmentID, slug, name string, isDefault bool) (QueueView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return QueueView{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if isDefault {
		if _, err := tx.Exec(ctx, `update messaging.queues set is_default = false, updated_at = now()
			where account_id = $1::uuid and department_id = $2::uuid and is_default`,
			accountID, departmentID); err != nil {
			return QueueView{}, err
		}
	}
	q, err := scanQueue(tx.QueryRow(ctx, `insert into messaging.queues
		(account_id, department_id, slug, name, is_default) values ($1::uuid, $2::uuid, $3, $4, $5)
		returning `+queueCols, accountID, departmentID, slug, name, isDefault))
	if err != nil {
		return QueueView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QueueView{}, err
	}
	return q, nil
}

// ListQueues devolve as filas da conta; departmentID opcional filtra por setor.
func (s *Store) ListQueues(ctx context.Context, accountID, departmentID string) ([]QueueView, error) {
	query := `select ` + queueCols + ` from messaging.queues where account_id = $1::uuid`
	args := []any{accountID}
	if departmentID != "" {
		args = append(args, departmentID)
		query += ` and department_id = $2::uuid`
	}
	query += ` order by name`

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]QueueView, 0)
	for rows.Next() {
		q, err := scanQueue(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// UpdateQueue aplica o patch parcial. isDefault limpa o default do mesmo setor. Precisa
// resolver o department_id da fila antes de limpar (a limpeza e por setor).
func (s *Store) UpdateQueue(ctx context.Context, accountID, id string, patch QueuePatch) (QueueView, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return QueueView{}, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if patch.IsDefault != nil && *patch.IsDefault {
		if _, err := tx.Exec(ctx, `update messaging.queues q set is_default = false, updated_at = now()
			from messaging.queues target
			where target.id = $2::uuid and target.account_id = $1::uuid
			  and q.account_id = $1::uuid and q.department_id = target.department_id and q.id <> target.id
			  and q.is_default`, accountID, id); err != nil {
			return QueueView{}, err
		}
	}
	q, err := scanQueue(tx.QueryRow(ctx, `update messaging.queues set
		name = coalesce($3, name),
		is_default = coalesce($4, is_default),
		is_active = coalesce($5, is_active),
		updated_at = now()
		where account_id = $1::uuid and id = $2::uuid
		returning `+queueCols, accountID, id, patch.Name, patch.IsDefault, patch.IsActive))
	if err != nil {
		return QueueView{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return QueueView{}, err
	}
	return q, nil
}

// SoftDeleteQueue desativa a fila (is_active=false). Conversas na fila seguem visiveis.
func (s *Store) SoftDeleteQueue(ctx context.Context, accountID, id string) error {
	tag, err := s.pool.Exec(ctx, `update messaging.queues set is_active = false, is_default = false,
		updated_at = now() where account_id = $1::uuid and id = $2::uuid`, accountID, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// queueInAccount valida que a fila e da conta (usada por membros e regras/transferencia).
func (s *Store) queueInAccount(ctx context.Context, accountID, queueID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from messaging.queues
		where account_id = $1::uuid and id = $2::uuid)`, accountID, queueID).Scan(&ok)
	return ok, err
}

// activeQueueInAccount valida fila ATIVA da conta (target de regra: inativa/de outra conta
// => 404, spec Contrato 6).
func (s *Store) activeQueueInAccount(ctx context.Context, accountID, queueID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from messaging.queues
		where account_id = $1::uuid and id = $2::uuid and is_active)`, accountID, queueID).Scan(&ok)
	return ok, err
}

// isUniqueViolation reconhece a colisao de slug (indice unico) para o service devolver 409.
func isUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

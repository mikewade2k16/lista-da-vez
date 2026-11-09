package finance

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func scanFiscalTemplate(row pgx.Row) (FiscalTemplate, error) {
	var out FiscalTemplate
	var raw []byte
	var updated time.Time
	err := row.Scan(&out.ID, &out.Name, &raw, &out.IsDefault, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrFiscalNotFound
	}
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(raw, &out.Fields); err != nil {
		return out, err
	}
	out.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return out, nil
}
func (s *PostgresFiscalStore) ListTemplates(ctx context.Context, account, user string) ([]FiscalTemplate, error) {
	rows, err := s.pool.Query(ctx, `select id::text,name,data,is_default,updated_at from finance.user_fiscal_templates
 where account_id=$1::uuid and user_id=$2::uuid order by is_default desc,lower(name),id limit 100`, account, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FiscalTemplate{}
	for rows.Next() {
		value, e := scanFiscalTemplate(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, value)
	}
	return out, rows.Err()
}
func (s *PostgresFiscalStore) SaveTemplate(ctx context.Context, account, user, id string, in FiscalTemplateInput) (FiscalTemplate, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FiscalTemplate{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return FiscalTemplate{}, err
	}
	if id != "" {
		current, e := scanFiscalTemplate(tx.QueryRow(ctx, `select id::text,name,data,is_default,updated_at from finance.user_fiscal_templates where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid for update`, account, user, id))
		if e != nil {
			return FiscalTemplate{}, e
		}
		if current.UpdatedAt != in.ExpectedUpdatedAt {
			return FiscalTemplate{}, ErrFiscalConflict
		}
	} else {
		var count int
		if err = tx.QueryRow(ctx, `select count(*) from finance.user_fiscal_templates where account_id=$1::uuid and user_id=$2::uuid`, account, user).Scan(&count); err != nil {
			return FiscalTemplate{}, err
		}
		if count >= 100 {
			return FiscalTemplate{}, invalidFiscal("Limite de 100 templates por perfil atingido.")
		}
	}
	if in.IsDefault {
		if _, err = tx.Exec(ctx, `update finance.user_fiscal_templates set is_default=false,updated_at=clock_timestamp() where account_id=$1::uuid and user_id=$2::uuid and is_default and id<>coalesce(nullif($3,'')::uuid,'00000000-0000-0000-0000-000000000000'::uuid)`, account, user, id); err != nil {
			return FiscalTemplate{}, err
		}
	}
	raw, err := json.Marshal(in.Fields)
	if err != nil {
		return FiscalTemplate{}, err
	}
	var row pgx.Row
	if id == "" {
		row = tx.QueryRow(ctx, `insert into finance.user_fiscal_templates(account_id,user_id,name,data,is_default) values($1::uuid,$2::uuid,$3,$4::jsonb,$5) returning id::text,name,data,is_default,updated_at`, account, user, in.Name, raw, in.IsDefault)
	} else {
		row = tx.QueryRow(ctx, `update finance.user_fiscal_templates set name=$4,data=$5::jsonb,is_default=$6,updated_at=clock_timestamp() where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid returning id::text,name,data,is_default,updated_at`, account, user, id, in.Name, raw, in.IsDefault)
	}
	out, err := scanFiscalTemplate(row)
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
func (s *PostgresFiscalStore) DeleteTemplate(ctx context.Context, account, user, id, expected string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return err
	}
	current, err := scanFiscalTemplate(tx.QueryRow(ctx, `select id::text,name,data,is_default,updated_at from finance.user_fiscal_templates where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid for update`, account, user, id))
	if err != nil {
		return err
	}
	if current.UpdatedAt != expected {
		return ErrFiscalConflict
	}
	if _, err = tx.Exec(ctx, `delete from finance.user_fiscal_templates where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid`, account, user, id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

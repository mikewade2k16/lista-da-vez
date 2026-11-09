package finance

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Recurring customers (tomadores) of the personal issuer, scoped by account and
// session user like the fiscal profile and templates. Versions are append-only.
type FiscalCustomer struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Document           string   `json:"document"`
	ServiceDescription string   `json:"serviceDescription"`
	TemplateID         string   `json:"templateId,omitempty"`
	Amount             *float64 `json:"amount"`
	Version            int      `json:"version"`
	LastUsedAt         string   `json:"lastUsedAt"`
	UpdatedAt          string   `json:"updatedAt"`
}
type FiscalCustomerVersion struct {
	Version       int             `json:"version"`
	Data          json.RawMessage `json:"data"`
	SourceDraftID string          `json:"sourceDraftId,omitempty"`
	CreatedAt     string          `json:"createdAt"`
}
type FiscalCustomerStore struct{ pool *pgxpool.Pool }

const fiscalCustomerColumns = `id::text,name,document,service_description,coalesce(template_id::text,''),amount::float8,version,last_used_at,updated_at`
const fiscalCustomerLimit = 1000

func normalizeCustomerDocument(value string) string {
	return strings.ToUpper(strings.NewReplacer(".", "", "/", "", "-", "", " ", "").Replace(strings.TrimSpace(value)))
}

func scanFiscalCustomer(row pgx.Row) (FiscalCustomer, error) {
	var out FiscalCustomer
	var used, updated time.Time
	err := row.Scan(&out.ID, &out.Name, &out.Document, &out.ServiceDescription, &out.TemplateID, &out.Amount, &out.Version, &used, &updated)
	if errors.Is(err, pgx.ErrNoRows) {
		return out, ErrFiscalNotFound
	}
	out.LastUsedAt = used.UTC().Format(time.RFC3339Nano)
	out.UpdatedAt = updated.UTC().Format(time.RFC3339Nano)
	return out, err
}

func (s *FiscalCustomerStore) query(ctx context.Context, sql string, args ...any) ([]FiscalCustomer, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FiscalCustomer{}
	for rows.Next() {
		c, e := scanFiscalCustomer(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *FiscalCustomerStore) List(ctx context.Context, account, user string) ([]FiscalCustomer, error) {
	return s.query(ctx, `select `+fiscalCustomerColumns+` from finance.fiscal_customers
 where account_id=$1::uuid and user_id=$2::uuid order by last_used_at desc,id limit 200`, account, user)
}

// FindByName matches the full name or its first words, newest use first, so a
// numbered choice stays stable between the question and the answer.
func (s *FiscalCustomerStore) FindByName(ctx context.Context, account, user, name string) ([]FiscalCustomer, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return []FiscalCustomer{}, nil
	}
	prefix := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(name) + ` %`
	return s.query(ctx, `select `+fiscalCustomerColumns+` from finance.fiscal_customers
 where account_id=$1::uuid and user_id=$2::uuid and (lower(name)=$3 or lower(name) like $4 escape '\')
 order by last_used_at desc,id limit 5`, account, user, name, prefix)
}

func (s *FiscalCustomerStore) FindByDocument(ctx context.Context, account, user, document string) (FiscalCustomer, error) {
	return scanFiscalCustomer(s.pool.QueryRow(ctx, `select `+fiscalCustomerColumns+` from finance.fiscal_customers
 where account_id=$1::uuid and user_id=$2::uuid and document=$3`, account, user, normalizeCustomerDocument(document)))
}

func (s *FiscalCustomerStore) Versions(ctx context.Context, account, user, id string) ([]FiscalCustomerVersion, error) {
	if !isUUID(id) {
		return nil, ErrFiscalNotFound
	}
	rows, err := s.pool.Query(ctx, `select version,data,coalesce(source_draft_id::text,''),created_at from finance.fiscal_customer_versions
 where account_id=$1::uuid and user_id=$2::uuid and customer_id=$3::uuid order by version desc limit 100`, account, user, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FiscalCustomerVersion{}
	for rows.Next() {
		var v FiscalCustomerVersion
		var created time.Time
		if err = rows.Scan(&v.Version, &v.Data, &v.SourceDraftID, &created); err != nil {
			return nil, err
		}
		v.CreatedAt = created.UTC().Format(time.RFC3339Nano)
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, ErrFiscalNotFound
	}
	return out, nil
}

func roundedAmount(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := math.Round(*v*100) / 100
	return &r
}
func sameAmount(a, b *float64) bool {
	a, b = roundedAmount(a), roundedAmount(b)
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// Remember saves the confirmed customer of a complete draft. It returns
// "created", "updated" (new version) or "unchanged" (only last use refreshed).
func (s *FiscalCustomerStore) Remember(ctx context.Context, account, user string, f DocumentFields, draftID string) (FiscalCustomer, string, error) {
	document := normalizeCustomerDocument(f.CustomerDocument)
	name := strings.TrimSpace(f.CustomerName)
	if !fiscalDocumentPattern.MatchString(document) || name == "" || len(name) > 300 {
		return FiscalCustomer{}, "", invalidFiscal("Cliente sem nome ou CPF/CNPJ válido.")
	}
	service := strings.TrimSpace(f.Service.ServiceDescription)
	template := ""
	if isUUID(f.TemplateID) {
		template = f.TemplateID
	}
	amount := roundedAmount(f.Amount)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FiscalCustomer{}, "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return FiscalCustomer{}, "", err
	}
	current, err := scanFiscalCustomer(tx.QueryRow(ctx, `select `+fiscalCustomerColumns+` from finance.fiscal_customers
 where account_id=$1::uuid and user_id=$2::uuid and document=$3 for update`, account, user, document))
	state := "updated"
	var saved FiscalCustomer
	switch {
	case errors.Is(err, ErrFiscalNotFound):
		var count int
		if err = tx.QueryRow(ctx, `select count(*) from finance.fiscal_customers where account_id=$1::uuid and user_id=$2::uuid`, account, user).Scan(&count); err != nil {
			return FiscalCustomer{}, "", err
		}
		if count >= fiscalCustomerLimit {
			return FiscalCustomer{}, "", invalidFiscal("Limite de clientes salvos atingido.")
		}
		state = "created"
		saved, err = scanFiscalCustomer(tx.QueryRow(ctx, `insert into finance.fiscal_customers(account_id,user_id,name,document,service_description,template_id,amount)
 values($1::uuid,$2::uuid,$3,$4,$5,nullif($6,'')::uuid,$7) returning `+fiscalCustomerColumns, account, user, name, document, service, template, amount))
	case err != nil:
		return FiscalCustomer{}, "", err
	case current.Name == name && current.ServiceDescription == service && current.TemplateID == template && sameAmount(current.Amount, amount):
		state = "unchanged"
		saved, err = scanFiscalCustomer(tx.QueryRow(ctx, `update finance.fiscal_customers set last_used_at=clock_timestamp()
 where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid returning `+fiscalCustomerColumns, account, user, current.ID))
	default:
		saved, err = scanFiscalCustomer(tx.QueryRow(ctx, `update finance.fiscal_customers set name=$4,service_description=$5,template_id=nullif($6,'')::uuid,amount=$7,
 version=version+1,last_used_at=clock_timestamp(),updated_at=clock_timestamp()
 where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid returning `+fiscalCustomerColumns, account, user, current.ID, name, service, template, amount))
	}
	if err != nil {
		return FiscalCustomer{}, "", err
	}
	if state != "unchanged" {
		data, e := json.Marshal(map[string]any{"name": saved.Name, "document": saved.Document, "serviceDescription": saved.ServiceDescription, "templateId": saved.TemplateID, "amount": saved.Amount})
		if e != nil {
			return FiscalCustomer{}, "", e
		}
		if _, err = tx.Exec(ctx, `insert into finance.fiscal_customer_versions(customer_id,account_id,user_id,version,data,source_draft_id)
 values($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb,nullif($6,'')::uuid)`, saved.ID, account, user, saved.Version, data, draftID); err != nil {
			return FiscalCustomer{}, "", err
		}
	}
	return saved, state, tx.Commit(ctx)
}

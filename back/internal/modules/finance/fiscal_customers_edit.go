package finance

import (
	"context"
	"encoding/json"
	"strings"
)

// FiscalCustomerInput é a edição manual feita na tela de clientes. O CPF/CNPJ é a
// identidade do cadastro e não muda; um documento diferente é outro cliente.
type FiscalCustomerInput struct {
	Name               string   `json:"name"`
	ServiceDescription string   `json:"serviceDescription"`
	Amount             *float64 `json:"amount"`
	ExpectedVersion    int      `json:"expectedVersion"`
}

// Update grava uma nova versão imutável com CAS pela versão atual.
func (s *FiscalCustomerStore) Update(ctx context.Context, account, user, id string, in FiscalCustomerInput) (FiscalCustomer, error) {
	name, service := strings.TrimSpace(in.Name), strings.TrimSpace(in.ServiceDescription)
	if !isUUID(id) {
		return FiscalCustomer{}, ErrFiscalNotFound
	}
	if name == "" || len(name) > 300 || len(service) > 4000 || (in.Amount != nil && (*in.Amount <= 0 || *in.Amount > 999999999999)) {
		return FiscalCustomer{}, invalidFiscal("Informe nome, serviço até 4000 caracteres e valor positivo.")
	}
	amount := roundedAmount(in.Amount)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FiscalCustomer{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = lockFiscalScope(ctx, tx, account, user); err != nil {
		return FiscalCustomer{}, err
	}
	current, err := scanFiscalCustomer(tx.QueryRow(ctx, `select `+fiscalCustomerColumns+` from finance.fiscal_customers
 where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid for update`, account, user, id))
	if err != nil {
		return FiscalCustomer{}, err
	}
	if current.Version != in.ExpectedVersion {
		return FiscalCustomer{}, ErrFiscalConflict
	}
	if current.Name == name && current.ServiceDescription == service && sameAmount(current.Amount, amount) {
		return current, nil
	}
	saved, err := scanFiscalCustomer(tx.QueryRow(ctx, `update finance.fiscal_customers set name=$4,service_description=$5,amount=$6,version=version+1,updated_at=clock_timestamp()
 where account_id=$1::uuid and user_id=$2::uuid and id=$3::uuid returning `+fiscalCustomerColumns, account, user, id, name, service, amount))
	if err != nil {
		return FiscalCustomer{}, err
	}
	data, err := json.Marshal(map[string]any{"name": saved.Name, "document": saved.Document, "serviceDescription": saved.ServiceDescription, "templateId": saved.TemplateID, "amount": saved.Amount, "editedManually": true})
	if err != nil {
		return FiscalCustomer{}, err
	}
	if _, err = tx.Exec(ctx, `insert into finance.fiscal_customer_versions(customer_id,account_id,user_id,version,data) values($1::uuid,$2::uuid,$3::uuid,$4,$5::jsonb)`,
		saved.ID, account, user, saved.Version, data); err != nil {
		return FiscalCustomer{}, err
	}
	return saved, tx.Commit(ctx)
}

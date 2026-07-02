package erp

import (
	"context"
	"strings"
	"unicode"
)

// enrichOrderRecords adiciona campos legiveis aos pedidos da PAGINA ja carregada,
// sem N+1: customer_name (pelo CPF == erp_customer_raw.identifier), store_label
// (CNPJ -> alias da loja) e product_names (sku -> nome em queue.erp_item_current).
//
// Performance: enriquece apenas as ~50 linhas visiveis (ou a pagina do export),
// nunca a tabela inteira. Os dois lookups sao em lote (= any($array)) e usam
// indices: erp_customer_raw(tenant_id, identifier) e erp_item_current(tenant_id,
// sku). Solucao temporaria ate haver tabelas desnormalizadas com esses nomes.
func (repository *PostgresRepository) enrichOrderRecords(ctx context.Context, store StoreScope, items []map[string]any) error {
	if len(items) == 0 {
		return nil
	}

	customerIDs := make(map[string]struct{})
	employeeIDs := make(map[string]struct{})
	skus := make(map[string]struct{})
	for _, item := range items {
		if cpf := strings.TrimSpace(recordString(item["customer_id"])); cpf != "" {
			customerIDs[cpf] = struct{}{}
		}
		if employeeID := strings.TrimSpace(recordString(item["employee_id"])); employeeID != "" {
			employeeIDs[employeeID] = struct{}{}
		}
		for _, sku := range splitSKUList(recordString(item["sku"])) {
			skus[sku] = struct{}{}
		}
	}

	contacts, err := repository.loadCustomerContactsByIdentifier(ctx, store.TenantID, distinctKeys(customerIDs))
	if err != nil {
		return err
	}
	employeeNames, err := repository.loadEmployeeNamesByOriginalID(ctx, store.TenantID, distinctKeys(employeeIDs))
	if err != nil {
		return err
	}
	productNames, err := repository.loadProductNamesBySKU(ctx, store.TenantID, distinctKeys(skus))
	if err != nil {
		return err
	}

	for _, item := range items {
		// Nomes do FTP vem em CAIXA ALTA; exibimos em Title Case (raw intacto).
		contact := contacts[strings.TrimSpace(recordString(item["customer_id"]))]
		item["customer_name"] = toTitleCase(contact.Name)
		// Contato do cliente (mesmo lote recente do nome) para acao de whats/email.
		item["customer_email"] = strings.ToLower(contact.Email)
		item["customer_phone"] = contact.Phone
		item["customer_mobile"] = contact.Mobile
		item["employee_name"] = toTitleCase(employeeNames[strings.TrimSpace(recordString(item["employee_id"]))])

		storeRaw := strings.TrimSpace(recordString(item["store_id_raw"]))
		if storeRaw == "" {
			storeRaw = strings.TrimSpace(recordString(item["store_cnpj"]))
		}
		item["store_label"] = resolveERPStoreLabel(storeRaw)

		names := make([]string, 0, 4)
		seen := make(map[string]struct{}, 4)
		for _, sku := range splitSKUList(recordString(item["sku"])) {
			name := productNames[sku]
			if name == "" {
				continue
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			names = append(names, toTitleCase(name))
		}
		item["product_names"] = strings.Join(names, ", ")
	}

	return nil
}

// customerContact reune o nome exibido e os contatos do cliente, todos do MESMO
// lote (o mais recente por CPF) — para a acao de whats/email na aba Compras.
type customerContact struct {
	Name   string
	Email  string
	Phone  string
	Mobile string
}

// loadCustomerContactsByIdentifier resolve, por CPF (identifier), o nome e os
// contatos do lote mais recente. Batch (= any($array)), sem N+1; usa o indice
// erp_customer_raw(tenant_id, identifier, source_batch_date desc, ...). O lote mais
// recente vence (mesma regra do nome exibido e da busca alinhada).
func (repository *PostgresRepository) loadCustomerContactsByIdentifier(ctx context.Context, tenantID string, identifiers []string) (map[string]customerContact, error) {
	result := make(map[string]customerContact, len(identifiers))
	if len(identifiers) == 0 {
		return result, nil
	}

	rows, err := repository.pool.Query(ctx, `
		select coalesce(identifier, ''), coalesce(name, ''), coalesce(nickname, ''),
		       coalesce(email, ''), coalesce(phone, ''), coalesce(mobile, '')
		from erp_customer_raw
		where tenant_id = $1::uuid and identifier = any($2::text[])
		order by source_batch_date desc, created_at_imported desc
	`, tenantID, identifiers)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var identifier, name, nickname, email, phone, mobile string
		if err := rows.Scan(&identifier, &name, &nickname, &email, &phone, &mobile); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(identifier)
		if key == "" {
			continue
		}
		// Primeiro match (lote mais recente, pela ordenacao) vence.
		if _, exists := result[key]; exists {
			continue
		}
		result[key] = customerContact{
			Name:   firstNonEmpty(strings.TrimSpace(name), strings.TrimSpace(nickname)),
			Email:  strings.TrimSpace(email),
			Phone:  strings.TrimSpace(phone),
			Mobile: strings.TrimSpace(mobile),
		}
	}

	return result, rows.Err()
}

func (repository *PostgresRepository) loadEmployeeNamesByOriginalID(ctx context.Context, tenantID string, ids []string) (map[string]string, error) {
	result := make(map[string]string, len(ids))
	if len(ids) == 0 {
		return result, nil
	}

	rows, err := repository.pool.Query(ctx, `
		select coalesce(original_id, ''), coalesce(name, '')
		from erp_employee_raw
		where tenant_id = $1::uuid and original_id = any($2::text[])
		order by source_batch_date desc, created_at_imported desc
	`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var originalID, name string
		if err := rows.Scan(&originalID, &name); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(originalID)
		if key == "" {
			continue
		}
		if _, exists := result[key]; exists {
			continue
		}
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			result[key] = trimmed
		}
	}

	return result, rows.Err()
}

func (repository *PostgresRepository) loadProductNamesBySKU(ctx context.Context, tenantID string, skus []string) (map[string]string, error) {
	result := make(map[string]string, len(skus))
	if len(skus) == 0 {
		return result, nil
	}

	rows, err := repository.pool.Query(ctx, `
		select sku, coalesce(name, '')
		from queue.erp_item_current
		where tenant_id = $1::uuid and sku = any($2::text[])
	`, tenantID, skus)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var sku, name string
		if err := rows.Scan(&sku, &name); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(sku)
		if key == "" {
			continue
		}
		if _, exists := result[key]; exists {
			continue
		}
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			result[key] = trimmed
		}
	}

	return result, rows.Err()
}

func resolveERPStoreLabel(storeRaw string) string {
	if strings.TrimSpace(storeRaw) == "" {
		return ""
	}
	if alias, ok := resolveCRMStoreAlias(storeRaw); ok {
		return alias.Label
	}
	return storeRaw
}

func recordString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func splitSKUList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func distinctKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	return out
}

// toTitleCase deixa a primeira letra de cada palavra maiuscula e o resto minuscula.
// Nomes de pessoa e produtos vem do FTP em CAIXA ALTA; isto e' so formatacao de
// exibicao na resposta da API — o dado raw continua intacto.
func toTitleCase(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	fields := strings.Fields(value)
	for index, field := range fields {
		runes := []rune(field)
		runes[0] = unicode.ToUpper(runes[0])
		fields[index] = string(runes)
	}
	return strings.Join(fields, " ")
}

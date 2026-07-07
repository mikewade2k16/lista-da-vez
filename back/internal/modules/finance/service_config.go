package finance

import "context"

// Regras de config (categorias/contas fixas/recorrencias) portadas de
// financeMockStore.ts (saveConfig). Separado de service.go para respeitar o
// limite de ~450 linhas por arquivo.

// GetConfig devolve a config do escopo (categorias/contas fixas/recorrencias).
func (s *Service) GetConfig(ctx context.Context, accountID, coreTenantID string) (ConfigData, error) {
	return s.config.GetConfig(ctx, accountID, scopeKey(coreTenantID))
}

// SaveConfig faz o full-replace da config e devolve o estado gravado.
func (s *Service) SaveConfig(ctx context.Context, accountID string, in ConfigInput) (ConfigData, error) {
	d := ConfigData{
		CoreTenantID:     scopeKey(strVal(in.CoreTenantID)),
		Categories:       normalizeCategories(in.Categories),
		FixedAccounts:    normalizeFixedAccounts(in.FixedAccounts),
		RecurringEntries: normalizeRecurring(in.RecurringEntries),
	}
	return s.config.SaveConfig(ctx, accountID, d)
}

// ListRecurringClients delega ao store (read model core.accounts + queue.stores).
func (s *Service) ListRecurringClients(ctx context.Context) ([]RecurringClient, error) {
	return s.config.ListRecurringClients(ctx)
}

// normalizeKind restringe kind a entrada/saida/ambas (default ambas).
func normalizeKind(k string) string {
	switch k {
	case "entrada", "saida", "ambas":
		return k
	default:
		return "ambas"
	}
}

func normalizeCategories(in []Category) []Category {
	out := make([]Category, 0, len(in))
	for _, c := range in {
		out = append(out, Category{
			ID:          normalizeID(c.ID),
			Name:        normText(c.Name, 120),
			Kind:        normalizeKind(c.Kind),
			Description: normText(c.Description, 400),
		})
	}
	return out
}

func normalizeFixedAccounts(in []FixedAccount) []FixedAccount {
	out := make([]FixedAccount, 0, len(in))
	for _, a := range in {
		members := make([]FixedAccountMember, 0, len(a.Members))
		for _, m := range a.Members {
			members = append(members, FixedAccountMember{
				ID:     normalizeID(m.ID),
				Name:   normText(m.Name, 120),
				Amount: num(m.Amount, false),
			})
		}
		out = append(out, FixedAccount{
			ID:            normalizeID(a.ID),
			Name:          normText(a.Name, 120),
			Kind:          normalizeKind(a.Kind),
			CategoryID:    normText(a.CategoryID, 90),
			DefaultAmount: num(a.DefaultAmount, false),
			Notes:         normText(a.Notes, 500),
			Members:       members,
		})
	}
	return out
}

func normalizeRecurring(in []RecurringEntry) []RecurringEntry {
	out := make([]RecurringEntry, 0, len(in))
	for _, e := range in {
		out = append(out, RecurringEntry{
			SourceCoreTenantID: normText(e.SourceCoreTenantID, 90),
			AdjustmentAmount:   num(e.AdjustmentAmount, true),
			Notes:              normText(e.Notes, 240),
		})
	}
	return out
}

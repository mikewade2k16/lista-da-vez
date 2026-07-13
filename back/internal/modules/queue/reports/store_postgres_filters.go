package reports

import (
	"context"
	"fmt"
	"strings"
)

// appendHistoryFilters escreve os predicados dinamicos do historico (mesma
// semantica do antigo miolo de listHistoryQuery) numerando placeholders por
// len(args)+1. Compartilhado entre listagem e count para nunca divergirem.
// NAO inclui o LIMIT — o teto e' aplicado so na listagem, apos o order by.
func appendHistoryFilters(query *strings.Builder, args []any, filters repositoryFilters) []any {
	// Auto-encerramento (2h): pendencias canceladas pelo gerente saem da metrica
	// (a linha fica no historico para auditoria). Predicado estatico, sempre aplicado.
	query.WriteString(" and h.validation_status <> 'cancelled'")

	if filters.FinishedAtFrom != nil {
		fmt.Fprintf(query, " and h.finished_at >= $%d", len(args)+1)
		args = append(args, *filters.FinishedAtFrom)
	}

	if filters.FinishedAtTo != nil {
		fmt.Fprintf(query, " and h.finished_at <= $%d", len(args)+1)
		args = append(args, *filters.FinishedAtTo)
	}

	if len(filters.ConsultantIDs) > 0 {
		fmt.Fprintf(query, " and h.person_id::text = any($%d)", len(args)+1)
		args = append(args, filters.ConsultantIDs)
	}

	if len(filters.Outcomes) > 0 {
		fmt.Fprintf(query, " and h.finish_outcome = any($%d)", len(args)+1)
		args = append(args, filters.Outcomes)
	}

	if len(filters.StartModes) > 0 {
		fmt.Fprintf(query, " and h.start_mode = any($%d)", len(args)+1)
		args = append(args, filters.StartModes)
	}

	if filters.IsExistingCustomer != nil {
		fmt.Fprintf(query, " and h.is_existing_customer = $%d", len(args)+1)
		args = append(args, *filters.IsExistingCustomer)
	}

	if filters.MinSaleAmount != nil {
		fmt.Fprintf(query, " and h.sale_amount >= $%d", len(args)+1)
		args = append(args, *filters.MinSaleAmount)
	}

	if filters.MaxSaleAmount != nil {
		fmt.Fprintf(query, " and h.sale_amount <= $%d", len(args)+1)
		args = append(args, *filters.MaxSaleAmount)
	}

	return args
}

// CountHistory conta o universo SQL-filtrado (sem join com queue.stores — o
// join da listagem existe so para store_name; store_id tem FK). Usado para
// preencher HistoryWindow.Total quando a janela bruta bate no teto.
func (repository *PostgresRepository) CountHistory(
	ctx context.Context,
	storeIDs []string,
	filters repositoryFilters,
) (int, error) {
	if len(storeIDs) == 0 {
		return 0, nil
	}

	query := strings.Builder{}
	query.WriteString("select count(*) from operation_service_history h where h.store_id::text = any($1)")

	args := []any{storeIDs}
	args = appendHistoryFilters(&query, args, filters)
	query.WriteString(";")

	var total int
	if err := repository.pool.QueryRow(ctx, query.String(), args...).Scan(&total); err != nil {
		return 0, err
	}

	return total, nil
}

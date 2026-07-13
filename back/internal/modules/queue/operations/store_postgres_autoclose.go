package operations

import (
	"context"
	"encoding/json"
	"strings"
)

// ValidateAutoClose promove a pendencia (validation_status='pending') a validada,
// gravando o desfecho real e os dados do modal de fechamento preenchidos pelo
// gerente. Preserva os campos IMUTAVEIS de tempo/fila/paralelo/consultor
// (started_at/finished_at/duration_ms/queue_*/parallel_*/person_*), que sao a
// metrica que o auto-encerramento existe para preservar. Devolve ErrPendingNotFound
// se nao houver linha pendente para (store_id, service_id).
func (repository *PostgresRepository) ValidateAutoClose(ctx context.Context, storeID string, entry ServiceHistoryEntry, validatedBy string, validatedAt int64) error {
	productsSeenRaw, err := json.Marshal(entry.ProductsSeen)
	if err != nil {
		return err
	}
	productsClosedRaw, err := json.Marshal(entry.ProductsClosed)
	if err != nil {
		return err
	}
	productsNotFoundRaw, err := json.Marshal(entry.ProductsNotFound)
	if err != nil {
		return err
	}
	visitReasonsRaw, err := json.Marshal(entry.VisitReasons)
	if err != nil {
		return err
	}
	visitReasonDetailsRaw, err := json.Marshal(entry.VisitReasonDetails)
	if err != nil {
		return err
	}
	customerSourcesRaw, err := json.Marshal(entry.CustomerSources)
	if err != nil {
		return err
	}
	customerSourceDetailsRaw, err := json.Marshal(entry.CustomerSourceDetails)
	if err != nil {
		return err
	}
	lossReasonsRaw, err := json.Marshal(entry.LossReasons)
	if err != nil {
		return err
	}
	lossReasonDetailsRaw, err := json.Marshal(entry.LossReasonDetails)
	if err != nil {
		return err
	}
	campaignMatchesRaw, err := json.Marshal(entry.CampaignMatches)
	if err != nil {
		return err
	}

	tag, err := repository.pool.Exec(ctx, `
		update operation_service_history set
			finish_outcome = $3,
			is_window_service = $4,
			is_gift = $5,
			product_seen = $6,
			product_closed = $7,
			purchase_code = $8,
			product_details = $9,
			products_seen_json = $10::jsonb,
			products_closed_json = $11::jsonb,
			products_not_found_json = $12::jsonb,
			products_seen_none = $13,
			visit_reasons_not_informed = $14,
			customer_sources_not_informed = $15,
			customer_name = $16,
			customer_phone = $17,
			customer_email = $18,
			is_existing_customer = $19,
			visit_reasons_json = $20::jsonb,
			visit_reason_details_json = $21::jsonb,
			customer_sources_json = $22::jsonb,
			customer_source_details_json = $23::jsonb,
			loss_reasons_json = $24::jsonb,
			loss_reason_details_json = $25::jsonb,
			loss_reason_id = $26,
			loss_reason = $27,
			sale_amount = $28,
			customer_profession = $29,
			queue_jump_reason = $30,
			notes = $31,
			campaign_matches_json = $32::jsonb,
			campaign_bonus_total = $33,
			validation_status = 'validated',
			validated_by = nullif($34::text, '')::uuid,
			validated_at = $35,
			validation_reason = $36
		where store_id = $1::uuid
			and service_id = $2
			and validation_status = 'pending';
	`,
		storeID,
		entry.ServiceID,
		entry.FinishOutcome,
		entry.IsWindowService,
		entry.IsGift,
		entry.ProductSeen,
		entry.ProductClosed,
		entry.PurchaseCode,
		entry.ProductDetails,
		string(productsSeenRaw),
		string(productsClosedRaw),
		string(productsNotFoundRaw),
		entry.ProductsSeenNone,
		entry.VisitReasonsNotInformed,
		entry.CustomerSourcesNotInformed,
		entry.CustomerName,
		entry.CustomerPhone,
		entry.CustomerEmail,
		entry.IsExistingCustomer,
		string(visitReasonsRaw),
		string(visitReasonDetailsRaw),
		string(customerSourcesRaw),
		string(customerSourceDetailsRaw),
		string(lossReasonsRaw),
		string(lossReasonDetailsRaw),
		entry.LossReasonID,
		entry.LossReason,
		entry.SaleAmount,
		entry.CustomerProfession,
		entry.QueueJumpReason,
		entry.Notes,
		string(campaignMatchesRaw),
		entry.CampaignBonusTotal,
		strings.TrimSpace(validatedBy),
		validatedAt,
		strings.TrimSpace(entry.ValidationReason),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPendingNotFound
	}
	return nil
}

// CancelAutoClose marca a pendencia como cancelled: sai da metrica (o filtro de
// analytics/reports ignora validation_status='cancelled') mas a linha e preservada
// para auditoria, com o motivo do gerente. ErrPendingNotFound se nao houver pendencia.
func (repository *PostgresRepository) CancelAutoClose(ctx context.Context, storeID string, serviceID string, cancelReason string, validatedBy string, validatedAt int64) error {
	tag, err := repository.pool.Exec(ctx, `
		update operation_service_history set
			validation_status = 'cancelled',
			cancel_reason = $3,
			validated_by = nullif($4::text, '')::uuid,
			validated_at = $5
		where store_id = $1::uuid
			and service_id = $2
			and validation_status = 'pending';
	`, storeID, serviceID, cancelReason, strings.TrimSpace(validatedBy), validatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrPendingNotFound
	}
	return nil
}

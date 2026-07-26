package erp

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
)

const (
	CustomerIntelligenceEntityCustomer      = "customer"
	CustomerIntelligenceEntityOrder         = "order"
	CustomerIntelligenceEntityOrderCanceled = "order_canceled"
	maxCustomerIntelligenceEntityIDLength   = 200
	maxCustomerIntelligenceTextRunes        = 2_000
	maxCustomerIntelligenceSKUs             = 100
)

var ErrCustomerIntelligenceEvidenceNotFound = errors.New("erp: customer intelligence evidence not found")

type CustomerIntelligenceEvidenceRequest struct {
	ClientAccountID string
	EntityType      string
	EntityID        string
	Fields          []string
}

type CustomerIntelligenceEvidence struct {
	EntityType string
	EntityID   string
	Version    string
	OccurredAt *time.Time
	Fields     map[string]any
}

type erpEvidenceRaw struct {
	EntityType         string
	EntityID           string
	Version            string
	SourceBatchDate    time.Time
	ObservedAt         time.Time
	OccurredAt         *time.Time
	Name               string
	Nickname           string
	RegisteredAt       string
	Birthday           string
	Gender             string
	City               string
	State              string
	Country            string
	Tags               string
	TotalAmountCents   *int64
	ProductReturnCents *int64
	SKUs               []string
	Quantity           int64
	PaymentType        string
	StoreCode          string
	Cancelled          bool
}

// ReadCustomerIntelligenceEvidence is the ERP-owned, exact-reference facade.
// EntityID is never searched by name, phone, e-mail, CPF, or fuzzy matching:
// customer means original_id; order/order_canceled mean order_id.
func (service *Service) ReadCustomerIntelligenceEvidence(
	ctx context.Context,
	request CustomerIntelligenceEvidenceRequest,
) (CustomerIntelligenceEvidence, error) {
	if service == nil || service.repository == nil {
		return CustomerIntelligenceEvidence{}, ErrCustomerIntelligenceEvidenceNotFound
	}
	request.ClientAccountID = strings.TrimSpace(request.ClientAccountID)
	request.EntityType = strings.ToLower(strings.TrimSpace(request.EntityType))
	request.EntityID = strings.TrimSpace(request.EntityID)
	if request.ClientAccountID == "" ||
		request.EntityID == "" ||
		len(request.EntityID) > maxCustomerIntelligenceEntityIDLength ||
		len(request.Fields) == 0 {
		return CustomerIntelligenceEvidence{}, ErrValidation
	}
	var (
		raw erpEvidenceRaw
		err error
	)
	switch request.EntityType {
	case CustomerIntelligenceEntityCustomer:
		raw, err = service.repository.readCustomerIntelligenceCustomer(
			ctx,
			request.ClientAccountID,
			request.EntityID,
		)
	case CustomerIntelligenceEntityOrder, CustomerIntelligenceEntityOrderCanceled:
		raw, err = service.repository.readCustomerIntelligenceOrder(
			ctx,
			request.ClientAccountID,
			request.EntityType,
			request.EntityID,
		)
	default:
		return CustomerIntelligenceEvidence{}, ErrUnsupportedDataType
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return CustomerIntelligenceEvidence{}, ErrCustomerIntelligenceEvidenceNotFound
	}
	if err != nil {
		return CustomerIntelligenceEvidence{}, err
	}
	fields := projectCustomerIntelligenceERPFields(raw, request.Fields)
	if len(fields) == 0 {
		return CustomerIntelligenceEvidence{}, ErrValidation
	}
	return CustomerIntelligenceEvidence{
		EntityType: raw.EntityType,
		EntityID:   raw.EntityID,
		Version:    raw.Version,
		OccurredAt: raw.OccurredAt,
		Fields:     fields,
	}, nil
}

func (repository *PostgresRepository) readCustomerIntelligenceCustomer(
	ctx context.Context,
	clientAccountID, originalID string,
) (erpEvidenceRaw, error) {
	var raw erpEvidenceRaw
	raw.EntityType = CustomerIntelligenceEntityCustomer
	raw.EntityID = originalID
	err := repository.pool.QueryRow(ctx, `
		select file_id::text, source_batch_date, created_at_imported,
		       coalesce(name, ''), coalesce(nickname, ''),
		       coalesce(registered_at_raw, ''), coalesce(birthday_raw, ''),
		       coalesce(gender, ''), coalesce(city, ''), coalesce(uf, ''),
		       coalesce(country, ''), coalesce(tags, '')
		from public.erp_customer_raw
		where tenant_id = $1::uuid
		  and original_id = $2
		  and btrim(original_id) <> ''
		order by source_batch_date desc, created_at_imported desc, id desc
		limit 1`,
		clientAccountID,
		originalID,
	).Scan(
		&raw.Version,
		&raw.SourceBatchDate,
		&raw.ObservedAt,
		&raw.Name,
		&raw.Nickname,
		&raw.RegisteredAt,
		&raw.Birthday,
		&raw.Gender,
		&raw.City,
		&raw.State,
		&raw.Country,
		&raw.Tags,
	)
	if err != nil {
		return erpEvidenceRaw{}, err
	}
	observedAt := raw.ObservedAt.UTC()
	raw.OccurredAt = &observedAt
	return raw, nil
}

func (repository *PostgresRepository) readCustomerIntelligenceOrder(
	ctx context.Context,
	clientAccountID, entityType, orderID string,
) (erpEvidenceRaw, error) {
	tableName := "public.erp_order_raw"
	cancelled := false
	if entityType == CustomerIntelligenceEntityOrderCanceled {
		tableName = "public.erp_order_canceled_raw"
		cancelled = true
	}
	query := `
		with latest_file as (
		    select file_id, source_batch_date
		    from ` + tableName + `
		    where tenant_id = $1::uuid
		      and order_id = $2
		      and btrim(order_id) <> ''
		    order by source_batch_date desc, created_at_imported desc, file_id desc
		    limit 1
		)
		select latest_file.file_id::text,
		       latest_file.source_batch_date,
		       max(orders.created_at_imported),
		       max(orders.order_date),
		       max(orders.total_amount_cents),
		       max(orders.product_return_cents),
		       coalesce(
		           array_agg(distinct nullif(btrim(orders.sku), ''))
		               filter (where btrim(orders.sku) <> ''),
		           '{}'::text[]
		       ),
		       coalesce(sum(orders.quantity), 0)::bigint,
		       coalesce(max(orders.payment_type), ''),
		       coalesce(max(orders.store_code), '')
		from latest_file
		join ` + tableName + ` orders
		  on orders.tenant_id = $1::uuid
		 and orders.file_id = latest_file.file_id
		 and orders.order_id = $2
		group by latest_file.file_id, latest_file.source_batch_date`

	raw := erpEvidenceRaw{
		EntityType: entityType,
		EntityID:   orderID,
		Cancelled:  cancelled,
	}
	err := repository.pool.QueryRow(ctx, query, clientAccountID, orderID).Scan(
		&raw.Version,
		&raw.SourceBatchDate,
		&raw.ObservedAt,
		&raw.OccurredAt,
		&raw.TotalAmountCents,
		&raw.ProductReturnCents,
		&raw.SKUs,
		&raw.Quantity,
		&raw.PaymentType,
		&raw.StoreCode,
	)
	if err != nil {
		return erpEvidenceRaw{}, err
	}
	if raw.OccurredAt == nil {
		observedAt := raw.ObservedAt.UTC()
		raw.OccurredAt = &observedAt
	} else {
		occurredAt := raw.OccurredAt.UTC()
		raw.OccurredAt = &occurredAt
	}
	return raw, nil
}

func projectCustomerIntelligenceERPFields(
	raw erpEvidenceRaw,
	requested []string,
) map[string]any {
	allowed := make(map[string]bool, len(requested))
	for _, field := range requested {
		allowed[strings.TrimSpace(field)] = true
	}
	out := make(map[string]any)
	put := func(key string, value any) {
		if !allowed[key] {
			return
		}
		switch typed := value.(type) {
		case string:
			value = boundedCustomerIntelligenceText(typed)
			if value == "" {
				return
			}
		case []string:
			value = boundedCustomerIntelligenceSKUs(typed)
			if len(value.([]string)) == 0 {
				return
			}
		case *int64:
			if typed == nil {
				return
			}
			value = *typed
		}
		out[key] = value
	}
	put("source_batch_date", raw.SourceBatchDate.Format(time.DateOnly))
	switch raw.EntityType {
	case CustomerIntelligenceEntityCustomer:
		put("preferred_name", firstNonEmpty(
			strings.TrimSpace(raw.Nickname),
			strings.TrimSpace(raw.Name),
		))
		put("registered_at", raw.RegisteredAt)
		put("birthday", raw.Birthday)
		put("gender", raw.Gender)
		put("city", raw.City)
		put("state", raw.State)
		put("country", raw.Country)
		put("tags", raw.Tags)
	case CustomerIntelligenceEntityOrder, CustomerIntelligenceEntityOrderCanceled:
		if raw.OccurredAt != nil {
			put("order_date", raw.OccurredAt.Format(time.RFC3339))
		}
		put("total_amount_cents", raw.TotalAmountCents)
		put("product_return_cents", raw.ProductReturnCents)
		put("skus", raw.SKUs)
		put("quantity", raw.Quantity)
		put("payment_type", raw.PaymentType)
		put("store_code", raw.StoreCode)
		put("cancelled", raw.Cancelled)
	}
	return out
}

func boundedCustomerIntelligenceText(value string) string {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= maxCustomerIntelligenceTextRunes {
		return value
	}
	return string([]rune(value)[:maxCustomerIntelligenceTextRunes])
}

func boundedCustomerIntelligenceSKUs(values []string) []string {
	if len(values) > maxCustomerIntelligenceSKUs {
		values = values[:maxCustomerIntelligenceSKUs]
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, candidate := range values {
		value := boundedCustomerIntelligenceText(candidate)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

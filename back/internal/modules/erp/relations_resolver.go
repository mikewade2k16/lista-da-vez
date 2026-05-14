package erp

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	platformmodules "github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type relationResolver struct {
	pool     *pgxpool.Pool
	moduleID string
}

type resolvedResource struct {
	Label    string
	URL      string
	Status   string
	Metadata map[string]any
}

func NewRelationResolver(pool *pgxpool.Pool) platformmodules.RelationResolver {
	return &relationResolver{pool: pool, moduleID: "erp"}
}

func NewCRMRelationResolver(pool *pgxpool.Pool) platformmodules.RelationResolver {
	return &relationResolver{pool: pool, moduleID: "crm"}
}

func (resolver *relationResolver) ModuleID() string {
	return resolver.moduleID
}

func (resolver *relationResolver) ResolveMany(ctx context.Context, accountID string, refs []platformmodules.RelationRef) ([]platformmodules.RelationResult, error) {
	results := make([]platformmodules.RelationResult, 0, len(refs))
	if resolver.pool == nil || strings.TrimSpace(accountID) == "" || len(refs) == 0 {
		for _, ref := range refs {
			results = append(results, unknownRelation(ref))
		}
		return results, nil
	}

	customerIDs := make(map[string]struct{})
	employeeIDs := make(map[string]struct{})
	orderIDs := make(map[string]struct{})
	canceledOrderIDs := make(map[string]struct{})
	recordIDs := make(map[string]struct{})

	for _, ref := range refs {
		resourceID := strings.TrimSpace(ref.ResourceID)
		if resourceID == "" {
			continue
		}
		switch resolver.normalizeResourceType(ref.ResourceType) {
		case "customer":
			customerIDs[resourceID] = struct{}{}
		case "employee":
			employeeIDs[resourceID] = struct{}{}
		case "order":
			orderIDs[resourceID] = struct{}{}
		case "order_canceled":
			canceledOrderIDs[resourceID] = struct{}{}
		case "record":
			recordIDs[resourceID] = struct{}{}
		}
	}

	customerMatches, err := resolver.resolveCustomers(ctx, accountID, joinSets(customerIDs, recordIDs))
	if err != nil {
		return nil, err
	}
	employeeMatches, err := resolver.resolveEmployees(ctx, accountID, joinSets(employeeIDs, recordIDs))
	if err != nil {
		return nil, err
	}
	orderMatches, err := resolver.resolveOrders(ctx, accountID, joinSets(orderIDs, recordIDs), false)
	if err != nil {
		return nil, err
	}
	canceledMatches, err := resolver.resolveOrders(ctx, accountID, joinSets(canceledOrderIDs, recordIDs), true)
	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		resourceID := strings.TrimSpace(ref.ResourceID)
		if resourceID == "" {
			results = append(results, unknownRelation(ref))
			continue
		}

		var resource resolvedResource
		var ok bool
		switch resolver.normalizeResourceType(ref.ResourceType) {
		case "customer":
			resource, ok = customerMatches[resourceID]
		case "employee":
			resource, ok = employeeMatches[resourceID]
		case "order":
			resource, ok = orderMatches[resourceID]
		case "order_canceled":
			resource, ok = canceledMatches[resourceID]
		case "record":
			resource, ok = customerMatches[resourceID]
			if !ok {
				resource, ok = employeeMatches[resourceID]
			}
			if !ok {
				resource, ok = orderMatches[resourceID]
			}
			if !ok {
				resource, ok = canceledMatches[resourceID]
			}
		}

		if !ok {
			results = append(results, unknownRelation(ref))
			continue
		}

		results = append(results, platformmodules.RelationResult{
			ModuleID:     ref.ModuleID,
			ResourceType: ref.ResourceType,
			ResourceID:   ref.ResourceID,
			Label:        resource.Label,
			URL:          resource.URL,
			Status:       resource.Status,
			Metadata:     cloneMetadata(resource.Metadata),
		})
	}

	return results, nil
}

func (resolver *relationResolver) resolveCustomers(ctx context.Context, accountID string, ids []string) (map[string]resolvedResource, error) {
	results := make(map[string]resolvedResource)
	if len(ids) == 0 {
		return results, nil
	}

	requested := toSet(ids)
	rows, err := resolver.pool.Query(ctx, `
		select identifier, original_id, name, nickname, email, mobile, phone,
		       store_id::text, store_code
		from erp_customer_raw
		where tenant_id = $1::uuid
		  and (identifier = any($2::text[]) or original_id = any($2::text[]))
		order by source_batch_date desc, created_at_imported desc
	`, accountID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var identifier string
		var originalID string
		var name string
		var nickname string
		var email string
		var mobile string
		var phone string
		var storeID string
		var storeCode string
		if err := rows.Scan(&identifier, &originalID, &name, &nickname, &email, &mobile, &phone, &storeID, &storeCode); err != nil {
			return nil, err
		}

		label := firstNonEmpty(name, nickname, identifier, originalID)
		for _, key := range []string{strings.TrimSpace(identifier), strings.TrimSpace(originalID)} {
			if key == "" {
				continue
			}
			if _, ok := requested[key]; !ok {
				continue
			}
			if _, exists := results[key]; exists {
				continue
			}
			results[key] = resolvedResource{
				Label:  label,
				URL:    resolver.workspaceURL("customers", key),
				Status: "resolved",
				Metadata: map[string]any{
					"dataType":  "customer",
					"storeId":   storeID,
					"storeCode": storeCode,
					"email":     strings.TrimSpace(email),
					"phone":     relationFirstNonEmpty(mobile, phone),
				},
			}
		}
	}

	return results, rows.Err()
}

func (resolver *relationResolver) resolveEmployees(ctx context.Context, accountID string, ids []string) (map[string]resolvedResource, error) {
	results := make(map[string]resolvedResource)
	if len(ids) == 0 {
		return results, nil
	}

	requested := toSet(ids)
	rows, err := resolver.pool.Query(ctx, `
		select original_id, name, store_id::text, store_code, is_active_raw
		from erp_employee_raw
		where tenant_id = $1::uuid
		  and original_id = any($2::text[])
		order by source_batch_date desc, created_at_imported desc
	`, accountID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var originalID string
		var name string
		var storeID string
		var storeCode string
		var isActiveRaw string
		if err := rows.Scan(&originalID, &name, &storeID, &storeCode, &isActiveRaw); err != nil {
			return nil, err
		}
		key := strings.TrimSpace(originalID)
		if _, ok := requested[key]; !ok {
			continue
		}
		if _, exists := results[key]; exists {
			continue
		}
		results[key] = resolvedResource{
			Label:  relationFirstNonEmpty(name, key),
			URL:    resolver.workspaceURL("employees", key),
			Status: statusFromFlag(isActiveRaw),
			Metadata: map[string]any{
				"dataType":  "employee",
				"storeId":   storeID,
				"storeCode": storeCode,
			},
		}
	}

	return results, rows.Err()
}

func (resolver *relationResolver) resolveOrders(ctx context.Context, accountID string, ids []string, canceled bool) (map[string]resolvedResource, error) {
	results := make(map[string]resolvedResource)
	if len(ids) == 0 {
		return results, nil
	}

	requested := toSet(ids)
	tableName := "erp_order_raw"
	viewTab := "orders"
	dataType := "order"
	if canceled {
		tableName = "erp_order_canceled_raw"
		viewTab = "orders-canceled"
		dataType = "order_canceled"
	}

	rows, err := resolver.pool.Query(ctx, `
		select order_id, identifier, customer_id, employee_id, store_id::text, store_code,
		       total_amount_cents, payment_type, order_date
		from `+tableName+`
		where tenant_id = $1::uuid
		  and (order_id = any($2::text[]) or identifier = any($2::text[]))
		order by source_batch_date desc, created_at_imported desc
	`, accountID, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var orderID string
		var identifier string
		var customerID string
		var employeeID string
		var storeID string
		var storeCode string
		var totalAmountCents *int64
		var paymentType string
		var orderDate *time.Time
		if err := rows.Scan(&orderID, &identifier, &customerID, &employeeID, &storeID, &storeCode, &totalAmountCents, &paymentType, &orderDate); err != nil {
			return nil, err
		}

		label := "Pedido " + relationFirstNonEmpty(identifier, orderID)
		for _, key := range []string{strings.TrimSpace(orderID), strings.TrimSpace(identifier)} {
			if key == "" {
				continue
			}
			if _, ok := requested[key]; !ok {
				continue
			}
			if _, exists := results[key]; exists {
				continue
			}
			metadata := map[string]any{
				"dataType":         dataType,
				"storeId":          storeID,
				"storeCode":        storeCode,
				"customerId":       strings.TrimSpace(customerID),
				"employeeId":       strings.TrimSpace(employeeID),
				"paymentType":      strings.TrimSpace(paymentType),
				"totalAmountCents": totalAmountCents,
			}
			if orderDate != nil {
				metadata["orderDate"] = orderDate.UTC()
			}
			results[key] = resolvedResource{
				Label:    label,
				URL:      resolver.workspaceURL(viewTab, key),
				Status:   "resolved",
				Metadata: metadata,
			}
		}
	}

	return results, rows.Err()
}

func (resolver *relationResolver) normalizeResourceType(resourceType string) string {
	switch strings.TrimSpace(strings.ToLower(resourceType)) {
	case "customer", "contact":
		return "customer"
	case "employee", "consultant":
		return "employee"
	case "order", "lead":
		return "order"
	case "order_canceled", "order-canceled", "canceled_order":
		return "order_canceled"
	case "record":
		return "record"
	default:
		return ""
	}
}

func (resolver *relationResolver) workspaceURL(tab string, identifier string) string {
	params := url.Values{}
	params.Set("search", identifier)
	if resolver.moduleID == "crm" {
		return "/operacao/crm?" + params.Encode()
	}
	params.Set("tab", tab)
	return "/operacao/erp?" + params.Encode()
}

func statusFromFlag(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "resolved"
	}
	if value == "1" || value == "true" || value == "sim" || value == "s" {
		return "active"
	}
	if value == "0" || value == "false" || value == "nao" || value == "n" {
		return "inactive"
	}
	return value
}

func unknownRelation(ref platformmodules.RelationRef) platformmodules.RelationResult {
	return platformmodules.RelationResult{
		ModuleID:     ref.ModuleID,
		ResourceType: ref.ResourceType,
		ResourceID:   ref.ResourceID,
		Status:       "unknown",
		Metadata:     map[string]any{"status": "unknown"},
	}
}

func joinSets(primary map[string]struct{}, secondary map[string]struct{}) []string {
	merged := make(map[string]struct{}, len(primary)+len(secondary))
	for key := range primary {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			merged[trimmed] = struct{}{}
		}
	}
	for key := range secondary {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			merged[trimmed] = struct{}{}
		}
	}
	result := make([]string, 0, len(merged))
	for key := range merged {
		result = append(result, key)
	}
	return result
}

func toSet(values []string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

func cloneMetadata(source map[string]any) map[string]any {
	if len(source) == 0 {
		return map[string]any{}
	}
	cloned := make(map[string]any, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func relationFirstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

package operations

import (
	"context"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	platformmodules "github.com/mikewade2k16/lista-da-vez/back/internal/platform/modules"
)

type relationResolver struct {
	pool *pgxpool.Pool
}

func NewRelationResolver(pool *pgxpool.Pool) platformmodules.RelationResolver {
	return &relationResolver{pool: pool}
}

func (resolver *relationResolver) ModuleID() string {
	return "operations"
}

func (resolver *relationResolver) ResolveMany(ctx context.Context, accountID string, refs []platformmodules.RelationRef) ([]platformmodules.RelationResult, error) {
	results := make([]platformmodules.RelationResult, 0, len(refs))
	if resolver.pool == nil || strings.TrimSpace(accountID) == "" || len(refs) == 0 {
		for _, ref := range refs {
			results = append(results, unknownOperationsRelation(ref))
		}
		return results, nil
	}

	requested := make(map[string]struct{})
	for _, ref := range refs {
		if resolver.normalizeType(ref.ResourceType) != "service_history" {
			continue
		}
		serviceID := strings.TrimSpace(ref.ResourceID)
		if serviceID != "" {
			requested[serviceID] = struct{}{}
		}
	}

	historyMatches, err := resolver.resolveHistory(ctx, accountID, requested)
	if err != nil {
		return nil, err
	}
	activeMatches, err := resolver.resolveActive(ctx, accountID, requested)
	if err != nil {
		return nil, err
	}

	for _, ref := range refs {
		serviceID := strings.TrimSpace(ref.ResourceID)
		if serviceID == "" || resolver.normalizeType(ref.ResourceType) != "service_history" {
			results = append(results, unknownOperationsRelation(ref))
			continue
		}
		if result, ok := historyMatches[serviceID]; ok {
			results = append(results, resultForOperations(ref, result))
			continue
		}
		if result, ok := activeMatches[serviceID]; ok {
			results = append(results, resultForOperations(ref, result))
			continue
		}
		results = append(results, unknownOperationsRelation(ref))
	}

	return results, nil
}

func (resolver *relationResolver) resolveHistory(ctx context.Context, accountID string, requested map[string]struct{}) (map[string]platformmodules.RelationResult, error) {
	results := make(map[string]platformmodules.RelationResult)
	serviceIDs := setKeys(requested)
	if len(serviceIDs) == 0 {
		return results, nil
	}

	rows, err := resolver.pool.Query(ctx, `
		select h.service_id, h.store_id::text, s.code, s.name, h.person_id::text, h.person_name,
		       h.finish_outcome, h.started_at, h.finished_at
		from operation_service_history h
		join stores s on s.id = h.store_id
		where s.tenant_id = $1::uuid
		  and h.service_id = any($2::text[])
		order by h.started_at desc
	`, accountID, serviceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var serviceID string
		var storeID string
		var storeCode string
		var storeName string
		var personID string
		var personName string
		var finishOutcome string
		var startedAt int64
		var finishedAt int64
		if err := rows.Scan(&serviceID, &storeID, &storeCode, &storeName, &personID, &personName, &finishOutcome, &startedAt, &finishedAt); err != nil {
			return nil, err
		}
		serviceID = strings.TrimSpace(serviceID)
		if serviceID == "" {
			continue
		}
		if _, exists := results[serviceID]; exists {
			continue
		}
		results[serviceID] = platformmodules.RelationResult{
			ResourceID: serviceID,
			Label:      operationsFirstNonEmpty(personName, serviceID) + " - " + operationsFirstNonEmpty(storeName, storeCode),
			URL:        operationsURL(storeID, serviceID),
			Status:     operationsFirstNonEmpty(finishOutcome, "resolved"),
			Metadata: map[string]any{
				"storeId":    storeID,
				"storeCode":  storeCode,
				"storeName":  storeName,
				"personId":   personID,
				"personName": personName,
				"startedAt":  startedAt,
				"finishedAt": finishedAt,
			},
		}
	}

	return results, rows.Err()
}

func (resolver *relationResolver) resolveActive(ctx context.Context, accountID string, requested map[string]struct{}) (map[string]platformmodules.RelationResult, error) {
	results := make(map[string]platformmodules.RelationResult)
	serviceIDs := setKeys(requested)
	if len(serviceIDs) == 0 {
		return results, nil
	}

	rows, err := resolver.pool.Query(ctx, `
		select a.service_id, a.store_id::text, s.code, s.name, c.id::text, c.name,
		       a.service_started_at, coalesce(a.stopped_at, 0)
		from operation_active_services a
		join stores s on s.id = a.store_id
		join consultants c on c.id = a.consultant_id
		where s.tenant_id = $1::uuid
		  and a.service_id = any($2::text[])
		order by a.service_started_at desc
	`, accountID, serviceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var serviceID string
		var storeID string
		var storeCode string
		var storeName string
		var personID string
		var personName string
		var startedAt int64
		var stoppedAt int64
		if err := rows.Scan(&serviceID, &storeID, &storeCode, &storeName, &personID, &personName, &startedAt, &stoppedAt); err != nil {
			return nil, err
		}
		serviceID = strings.TrimSpace(serviceID)
		if serviceID == "" {
			continue
		}
		if _, exists := results[serviceID]; exists {
			continue
		}
		results[serviceID] = platformmodules.RelationResult{
			ResourceID: serviceID,
			Label:      operationsFirstNonEmpty(personName, serviceID) + " - " + operationsFirstNonEmpty(storeName, storeCode),
			URL:        operationsURL(storeID, serviceID),
			Status:     "active",
			Metadata: map[string]any{
				"storeId":    storeID,
				"storeCode":  storeCode,
				"storeName":  storeName,
				"personId":   personID,
				"personName": personName,
				"startedAt":  startedAt,
				"stoppedAt":  stoppedAt,
			},
		}
	}

	return results, rows.Err()
}

func (resolver *relationResolver) normalizeType(resourceType string) string {
	switch strings.TrimSpace(strings.ToLower(resourceType)) {
	case "service_history", "service-history", "service":
		return "service_history"
	default:
		return ""
	}
}

func resultForOperations(ref platformmodules.RelationRef, result platformmodules.RelationResult) platformmodules.RelationResult {
	return platformmodules.RelationResult{
		ModuleID:     ref.ModuleID,
		ResourceType: ref.ResourceType,
		ResourceID:   ref.ResourceID,
		Label:        result.Label,
		URL:          result.URL,
		Status:       result.Status,
		Metadata:     cloneMetadata(result.Metadata),
	}
}

func unknownOperationsRelation(ref platformmodules.RelationRef) platformmodules.RelationResult {
	return platformmodules.RelationResult{
		ModuleID:     ref.ModuleID,
		ResourceType: ref.ResourceType,
		ResourceID:   ref.ResourceID,
		Status:       "unknown",
		Metadata:     map[string]any{"status": "unknown"},
	}
}

func operationsURL(storeID string, serviceID string) string {
	params := url.Values{}
	if strings.TrimSpace(storeID) != "" {
		params.Set("storeId", strings.TrimSpace(storeID))
	}
	if strings.TrimSpace(serviceID) != "" {
		params.Set("serviceId", strings.TrimSpace(serviceID))
	}
	encoded := params.Encode()
	if encoded == "" {
		return "/operacao"
	}
	return "/operacao?" + encoded
}

func setKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		trimmed := strings.TrimSpace(key)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	return keys
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

func operationsFirstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

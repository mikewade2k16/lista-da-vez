package bi

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
)

var perolaFilterOperators = map[string]string{
	"eq":         "equalsTo",
	"neq":        "differentTo",
	"gt":         "greaterThan",
	"gte":        "greaterMoreThan",
	"lt":         "lessThan",
	"lte":        "lessMoreThan",
	"contains":   "content",
	"startsWith": "startWith",
	"endsWith":   "endWith",
}

type normalizedPerolaDatasetQuery struct {
	PageNumber int
	Limit      int
	OrderBy    PerolaDatasetOrderInput
	Filters    []PerolaDatasetFilterInput
	Body       []byte
}

func (service *Service) PerolaDatasetCatalog() PerolaDatasetCatalogResponse {
	return perolaDatasetCatalog()
}

func (service *Service) QueryPerolaDataset(
	ctx context.Context,
	datasetID string,
	input PerolaDatasetQueryInput,
) (PerolaDatasetQueryResponse, error) {
	spec, ok := findPerolaDatasetSpec(datasetID)
	if !ok {
		return PerolaDatasetQueryResponse{}, ErrUnsupportedDataset
	}

	query, err := normalizePerolaDatasetQuery(spec, input)
	if err != nil {
		return PerolaDatasetQueryResponse{}, err
	}

	requestCtx := ctx
	cancel := func() {}
	if spec.RequestTimeout > 0 {
		requestCtx, cancel = context.WithTimeout(ctx, spec.RequestTimeout)
	}
	defer cancel()

	upstream, err := service.perola.Find(requestCtx, spec.Endpoint, query.Body)
	if err != nil {
		return PerolaDatasetQueryResponse{}, err
	}
	if !upstream.OK {
		return PerolaDatasetQueryResponse{}, fmt.Errorf("%w: dataset %s returned status %d", ErrUpstream, spec.ID, upstream.UpstreamStatus)
	}

	records := sanitizePerolaDatasetRecords(extractRecords(upstream.Body))
	totalRecords, totalPages := extractPagination(upstream.Body)
	if totalRecords == 0 {
		totalRecords = len(records)
	}
	if totalPages == 0 && totalRecords > 0 {
		totalPages = int(math.Ceil(float64(totalRecords) / float64(query.Limit)))
	}

	return PerolaDatasetQueryResponse{
		DatasetID:    spec.ID,
		DatasetLabel: spec.Label,
		PageNumber:   query.PageNumber,
		Limit:        query.Limit,
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
		Returned:     len(records),
		HasMore:      totalPages > query.PageNumber,
		OrderBy:      query.OrderBy,
		FilterCount:  len(query.Filters),
		DurationMs:   upstream.DurationMs,
		Records:      records,
	}, nil
}

func normalizePerolaDatasetQuery(
	spec perolaDatasetSpec,
	input PerolaDatasetQueryInput,
) (normalizedPerolaDatasetQuery, error) {
	pageNumber := input.PageNumber
	if pageNumber == 0 {
		pageNumber = 1
	}
	if pageNumber < 1 || pageNumber > perolaDatasetMaxPageNumber {
		return normalizedPerolaDatasetQuery{}, fmt.Errorf("%w: invalid pageNumber", ErrValidation)
	}

	limit := input.Limit
	if limit <= 0 {
		limit = spec.DefaultLimit
	}
	if limit <= 0 {
		limit = perolaDatasetDefaultQueryLimit
	}
	if limit > spec.MaxLimit {
		limit = spec.MaxLimit
	}

	orderField := strings.TrimSpace(input.OrderBy.Field)
	if orderField == "" {
		orderField = spec.DefaultOrderField
	}
	if !slices.Contains(spec.AllowedOrderFields, orderField) {
		return normalizedPerolaDatasetQuery{}, fmt.Errorf("%w: unsupported order field", ErrValidation)
	}

	orderDirection := strings.ToUpper(strings.TrimSpace(input.OrderBy.Direction))
	if orderDirection == "" {
		orderDirection = spec.DefaultOrderDirection
	}
	if orderDirection != "ASC" && orderDirection != "DESC" {
		return normalizedPerolaDatasetQuery{}, fmt.Errorf("%w: unsupported order direction", ErrValidation)
	}

	filters, conditions, selectors, err := normalizePerolaDatasetFilters(spec, input.Filters)
	if err != nil {
		return normalizedPerolaDatasetQuery{}, err
	}
	if !matchesRequiredFilterAlternative(spec.RequiredAlternatives, selectors) {
		return normalizedPerolaDatasetQuery{}, fmt.Errorf("%w: %s", ErrFilterRequired, spec.RequiredFilterRule)
	}
	if err := validatePerolaDateRange(spec.DateRange, filters); err != nil {
		return normalizedPerolaDatasetQuery{}, err
	}

	body, err := json.Marshal(perolaPagePayload{
		PageNumber: pageNumber,
		Limit:      limit,
		OrderBy:    map[string]string{orderField: orderDirection},
		Conditions: conditions,
	})
	if err != nil {
		return normalizedPerolaDatasetQuery{}, fmt.Errorf("%w: encode query", ErrValidation)
	}

	return normalizedPerolaDatasetQuery{
		PageNumber: pageNumber,
		Limit:      limit,
		OrderBy:    PerolaDatasetOrderInput{Field: orderField, Direction: orderDirection},
		Filters:    filters,
		Body:       body,
	}, nil
}

func normalizePerolaDatasetFilters(
	spec perolaDatasetSpec,
	input []PerolaDatasetFilterInput,
) ([]PerolaDatasetFilterInput, map[string]map[string]any, map[perolaFilterSelector]bool, error) {
	if len(input) > perolaDatasetMaxFilters {
		return nil, nil, nil, fmt.Errorf("%w: too many filters", ErrValidation)
	}

	conditions := emptyPerolaConditions()
	selectors := make(map[perolaFilterSelector]bool, len(input))
	filters := make([]PerolaDatasetFilterInput, 0, len(input))
	for _, candidate := range input {
		field := strings.TrimSpace(candidate.Field)
		operator := normalizePerolaFilterOperator(candidate.Operator)
		rule, ok := spec.Filters[field]
		if !ok || !slices.Contains(rule.Operators, operator) {
			return nil, nil, nil, fmt.Errorf("%w: unsupported filter", ErrValidation)
		}

		selector := perolaFilterSelector{Field: field, Operator: operator}
		if selectors[selector] {
			return nil, nil, nil, fmt.Errorf("%w: duplicated filter", ErrValidation)
		}

		value, err := normalizePerolaFilterValue(rule.ValueType, candidate.Value)
		if err != nil {
			return nil, nil, nil, err
		}
		upstreamOperator := perolaFilterOperators[operator]
		conditions[upstreamOperator][field] = value
		selectors[selector] = true
		filters = append(filters, PerolaDatasetFilterInput{Field: field, Operator: operator, Value: value})
	}

	return filters, conditions, selectors, nil
}

func normalizePerolaFilterOperator(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return "eq"
	}
	if strings.EqualFold(normalized, "startswith") {
		return "startsWith"
	}
	if strings.EqualFold(normalized, "endswith") {
		return "endsWith"
	}
	return strings.ToLower(normalized)
}

func normalizePerolaFilterValue(valueType perolaFilterValueType, value any) (any, error) {
	switch valueType {
	case perolaFilterString:
		text, ok := value.(string)
		text = strings.TrimSpace(text)
		if !ok || text == "" || len(text) > perolaDatasetMaxStringFilterLen {
			return nil, fmt.Errorf("%w: invalid string filter", ErrValidation)
		}
		return text, nil
	case perolaFilterInteger:
		return normalizePerolaInteger(value)
	case perolaFilterDate:
		text, ok := value.(string)
		text = strings.TrimSpace(text)
		if !ok {
			return nil, fmt.Errorf("%w: invalid date filter", ErrValidation)
		}
		if _, err := time.Parse(time.DateOnly, text); err != nil {
			return nil, fmt.Errorf("%w: date must use YYYY-MM-DD", ErrValidation)
		}
		return text, nil
	case perolaFilterBoolean:
		resolved, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("%w: invalid boolean filter", ErrValidation)
		}
		return resolved, nil
	default:
		return nil, fmt.Errorf("%w: unsupported filter type", ErrValidation)
	}
}

func normalizePerolaInteger(value any) (int64, error) {
	switch typed := value.(type) {
	case int:
		if typed >= 0 {
			return int64(typed), nil
		}
	case int64:
		if typed >= 0 {
			return typed, nil
		}
	case float64:
		if math.Trunc(typed) == typed && typed >= 0 && typed <= math.MaxInt64 {
			return int64(typed), nil
		}
	case json.Number:
		resolved, err := typed.Int64()
		if err == nil && resolved >= 0 {
			return resolved, nil
		}
	case string:
		resolved, err := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		if err == nil && resolved >= 0 {
			return resolved, nil
		}
	}
	return 0, fmt.Errorf("%w: invalid integer filter", ErrValidation)
}

func matchesRequiredFilterAlternative(
	alternatives [][]perolaFilterSelector,
	selectors map[perolaFilterSelector]bool,
) bool {
	for _, alternative := range alternatives {
		matches := true
		for _, selector := range alternative {
			if !selectors[selector] {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

func validatePerolaDateRange(rule *perolaDateRangeRule, filters []PerolaDatasetFilterInput) error {
	if rule == nil {
		return nil
	}

	var dateFrom, dateTo string
	for _, filter := range filters {
		if filter.Field != rule.Field {
			continue
		}
		if filter.Operator == "gte" {
			dateFrom, _ = filter.Value.(string)
		}
		if filter.Operator == "lte" {
			dateTo, _ = filter.Value.(string)
		}
	}
	if dateFrom == "" && dateTo == "" {
		return nil
	}
	if dateFrom == "" || dateTo == "" {
		return fmt.Errorf("%w: date range must have gte and lte", ErrFilterRequired)
	}

	from, _ := time.Parse(time.DateOnly, dateFrom)
	to, _ := time.Parse(time.DateOnly, dateTo)
	maxSpan := time.Duration(rule.MaxDays-1) * 24 * time.Hour
	if to.Before(from) || to.Sub(from) > maxSpan {
		return fmt.Errorf("%w: date range exceeds allowed window", ErrValidation)
	}
	return nil
}

func emptyPerolaConditions() map[string]map[string]any {
	return map[string]map[string]any{
		"startWith":       {},
		"endWith":         {},
		"content":         {},
		"equalsTo":        {},
		"differentTo":     {},
		"greaterThan":     {},
		"greaterMoreThan": {},
		"lessThan":        {},
		"lessMoreThan":    {},
	}
}

func sanitizePerolaDatasetRecords(records []map[string]any) []map[string]any {
	sanitized := make([]map[string]any, 0, len(records))
	for _, record := range records {
		item := make(map[string]any, len(record))
		for key, value := range record {
			switch typed := value.(type) {
			case nil, bool, float64:
				item[key] = typed
			case string:
				item[key] = strings.TrimSpace(typed)
			}
		}
		sanitized = append(sanitized, item)
	}
	return sanitized
}

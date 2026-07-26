package bi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	perolaRecentSalesEndpoint = "/vendas/colaboradores"
	perolaRecentSalesLimit    = 20
	perolaRecentSalesLookback = 3650
)

type PerolaRecentSalesResponse struct {
	GeneratedAt string           `json:"generatedAt"`
	PeriodStart string           `json:"periodStart"`
	PeriodEnd   string           `json:"periodEnd"`
	Limit       int              `json:"limit"`
	Returned    int              `json:"returned"`
	DurationMs  int64            `json:"durationMs"`
	Records     []map[string]any `json:"records"`
}

func (service *Service) PerolaRecentSales(ctx context.Context) (PerolaRecentSalesResponse, error) {
	now := service.perola.now().UTC()
	periodEnd := now.Format(time.DateOnly)
	periodStart := now.AddDate(0, 0, -perolaRecentSalesLookback).Format(time.DateOnly)

	query := url.Values{}
	query.Set("dtInicio", periodStart)
	query.Set("dtFinal", periodEnd)

	upstream, err := service.perola.Get(ctx, perolaRecentSalesEndpoint, query)
	if err != nil {
		return PerolaRecentSalesResponse{}, err
	}
	if upstream.UpstreamStatus == http.StatusUnauthorized || upstream.UpstreamStatus == http.StatusForbidden {
		return PerolaRecentSalesResponse{}, ErrSalesUnauthorized
	}
	if !upstream.OK {
		return PerolaRecentSalesResponse{}, fmt.Errorf(
			"%w: recent sales returned status %d",
			ErrUpstream,
			upstream.UpstreamStatus,
		)
	}

	records := sanitizeRecentSalesRecords(extractRecentSalesRecords(upstream.Body))
	if len(records) == 0 {
		slog.Warn(
			"bi_recent_sales_empty_upstream",
			slog.String("payload_shape", recentSalesPayloadShape(upstream.Body)),
			slog.Int("upstream_status", upstream.UpstreamStatus),
			slog.Int64("duration_ms", upstream.DurationMs),
		)
	}
	sort.SliceStable(records, func(left, right int) bool {
		return recentSaleTime(records[left]).After(recentSaleTime(records[right]))
	})
	if len(records) > perolaRecentSalesLimit {
		records = records[:perolaRecentSalesLimit]
	}

	return PerolaRecentSalesResponse{
		GeneratedAt: now.Format(time.RFC3339),
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		Limit:       perolaRecentSalesLimit,
		Returned:    len(records),
		DurationMs:  upstream.DurationMs,
		Records:     records,
	}, nil
}

func recentSalesPayloadShape(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case []any:
		if len(typed) == 0 {
			return "array[0]"
		}
		return fmt.Sprintf("array[%d]<%s>", len(typed), recentSalesPayloadShape(typed[0]))
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key, nested := range typed {
			keys = append(keys, fmt.Sprintf("%s:%s", key, recentSalesPayloadShape(nested)))
		}
		sort.Strings(keys)
		return "object{" + strings.Join(keys, ",") + "}"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func extractRecentSalesRecords(body any) []map[string]any {
	switch typed := body.(type) {
	case []any:
		records := make([]map[string]any, 0, len(typed))
		for _, candidate := range typed {
			if record, ok := candidate.(map[string]any); ok {
				records = append(records, record)
			}
		}
		return records
	case map[string]any:
		for _, key := range []string{"registros", "vendas", "dados", "data", "resultado", "results"} {
			if nested, exists := typed[key]; exists {
				if records := extractRecentSalesRecords(nested); len(records) > 0 {
					return records
				}
			}
		}
	}
	return []map[string]any{}
}

func sanitizeRecentSalesRecords(records []map[string]any) []map[string]any {
	sanitized := sanitizePerolaDatasetRecords(records)
	for _, record := range sanitized {
		for key := range record {
			if isSensitiveRecentSalesField(key) {
				delete(record, key)
			}
		}
	}
	return sanitized
}

func isSensitiveRecentSalesField(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, fragment := range []string{
		"cpf", "cnpj", "rg", "email", "telefone", "celular",
		"cep", "logradouro", "endereco", "bairro",
	} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func recentSaleTime(record map[string]any) time.Time {
	for _, key := range []string{
		"dataVenda", "dtVenda", "data", "dataEmissao",
		"created", "modified", "createdAt", "updatedAt",
	} {
		raw, ok := record[key].(string)
		if !ok {
			continue
		}
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"02/01/2006 15:04:05",
			time.DateOnly,
			"02/01/2006",
		} {
			if parsed, err := time.Parse(layout, strings.TrimSpace(raw)); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

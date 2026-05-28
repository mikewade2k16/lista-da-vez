package catalog

import "strings"

type productSearchScope string

const (
	productSearchScopeStore        productSearchScope = "store"
	productSearchScopeTenantShared productSearchScope = "tenant_shared"
)

type productSearchSource struct {
	Key               ProductSourceKey
	Scope             productSearchScope
	TableName         string
	IDExpression      string
	CodeExpression    string
	NameExpression    string
	PriceCentsExpr    string
	PrefixExpressions []string
}

var productSearchSources = map[ProductSourceKey]productSearchSource{
	ProductSourceERPCurrent: {
		Key:               ProductSourceERPCurrent,
		Scope:             productSearchScopeTenantShared,
		TableName:         "erp_item_current",
		IDExpression:      "sku",
		CodeExpression:    "sku",
		NameExpression:    "name",
		PriceCentsExpr:    "price_cents",
		PrefixExpressions: []string{"sku"},
	},
}

func resolveProductSearchSource(key ProductSourceKey) (productSearchSource, bool) {
	source, ok := productSearchSources[key]
	return source, ok
}

func buildPrefixSearchCondition(expressions []string, placeholder string) string {
	parts := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		normalized := strings.TrimSpace(expression)
		if normalized == "" {
			continue
		}
		parts = append(parts, normalized+" ilike "+placeholder)
	}
	if len(parts) == 0 {
		return "false"
	}
	return strings.Join(parts, " or ")
}

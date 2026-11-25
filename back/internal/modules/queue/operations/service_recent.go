package operations

import (
	"context"
	"strings"
)

const (
	recentServicesDefaultPerPage = 10
	recentServicesMaxPerPage     = 50
)

// RecentServiceItem e a projecao enxuta de um atendimento encerrado para a
// auditoria "ultimos atendimentos" (platform_admin): quem atendeu, em qual loja,
// quando e com qual desfecho. Le de queue.operation_service_history.
type RecentServiceItem struct {
	ServiceID          string  `json:"serviceId"`
	StoreID            string  `json:"storeId"`
	StoreName          string  `json:"storeName"`
	PersonID           string  `json:"personId"`
	PersonName         string  `json:"personName"`
	StartedAt          int64   `json:"startedAt"`
	FinishedAt         int64   `json:"finishedAt"`
	DurationMs         int64   `json:"durationMs"`
	FinishOutcome      string  `json:"finishOutcome"`
	StartMode          string  `json:"startMode"`
	QueueWaitMs        int64   `json:"queueWaitMs"`
	SkippedCount       int     `json:"skippedCount"`
	SaleAmount         float64 `json:"saleAmount"`
	CustomerName       string  `json:"customerName"`
	IsExistingCustomer bool    `json:"isExistingCustomer"`
	CloseReason        string  `json:"closeReason"`
	ValidationStatus   string  `json:"validationStatus"`
}

// RecentServicesPage e o body de GET /v1/operations/recent-services.
type RecentServicesPage struct {
	Items   []RecentServiceItem `json:"items"`
	Total   int                 `json:"total"`
	Page    int                 `json:"page"`
	PerPage int                 `json:"perPage"`
}

// RecentServicesInput chega do handler; StoreID vazio = todas as lojas acessiveis.
type RecentServicesInput struct {
	StoreID string
	Page    int
	PerPage int
}

// RecentServices lista os atendimentos encerrados mais recentes, do mais novo
// para o mais antigo. Exclusivo de platform_admin. As lojas consultadas saem do
// provedor de escopo restrito a account ativa (nunca da query crua): storeId
// fora desse conjunto responde 404.
func (service *Service) RecentServices(ctx context.Context, access AccessContext, input RecentServicesInput) (RecentServicesPage, error) {
	if access.Role != RolePlatformAdmin || service.storeScopeProvider == nil {
		return RecentServicesPage{}, ErrForbidden
	}

	stores, err := service.storeScopeProvider.ListAccessible(ctx, access, StoreScopeFilter{TenantID: access.ScopeTenantID()})
	if err != nil {
		return RecentServicesPage{}, err
	}

	storeNames := make(map[string]string, len(stores))
	storeIDs := make([]string, 0, len(stores))
	for _, store := range stores {
		id := strings.TrimSpace(store.ID)
		if id == "" {
			continue
		}
		storeNames[id] = store.Name
		storeIDs = append(storeIDs, id)
	}

	if requested := strings.TrimSpace(input.StoreID); requested != "" {
		if _, ok := storeNames[requested]; !ok {
			return RecentServicesPage{}, ErrStoreNotFound
		}
		storeIDs = []string{requested}
	}

	page, perPage := normalizeRecentServicesPaging(input.Page, input.PerPage)
	result := RecentServicesPage{Items: []RecentServiceItem{}, Page: page, PerPage: perPage}
	if len(storeIDs) == 0 {
		return result, nil
	}

	items, total, err := service.repository.ListRecentServices(ctx, storeIDs, perPage, (page-1)*perPage)
	if err != nil {
		return RecentServicesPage{}, err
	}
	for index := range items {
		items[index].StoreName = storeNames[items[index].StoreID]
	}

	result.Items = items
	result.Total = total
	return result, nil
}

func normalizeRecentServicesPaging(page, perPage int) (int, int) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = recentServicesDefaultPerPage
	}
	if perPage > recentServicesMaxPerPage {
		perPage = recentServicesMaxPerPage
	}
	return page, perPage
}

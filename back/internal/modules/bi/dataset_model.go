package bi

type PerolaDatasetOrderInput struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type PerolaDatasetFilterInput struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type PerolaDatasetQueryInput struct {
	PageNumber int                        `json:"pageNumber"`
	Limit      int                        `json:"limit"`
	OrderBy    PerolaDatasetOrderInput    `json:"orderBy"`
	Filters    []PerolaDatasetFilterInput `json:"filters"`
}

type PerolaDatasetQueryResponse struct {
	DatasetID    string                  `json:"datasetId"`
	DatasetLabel string                  `json:"datasetLabel"`
	PageNumber   int                     `json:"pageNumber"`
	Limit        int                     `json:"limit"`
	TotalRecords int                     `json:"totalRecords"`
	TotalPages   int                     `json:"totalPages"`
	Returned     int                     `json:"returned"`
	HasMore      bool                    `json:"hasMore"`
	OrderBy      PerolaDatasetOrderInput `json:"orderBy"`
	FilterCount  int                     `json:"filterCount"`
	DurationMs   int64                   `json:"durationMs"`
	Records      []map[string]any        `json:"records"`
}

type PerolaDatasetCatalogResponse struct {
	Datasets []PerolaDatasetCatalogItem `json:"datasets"`
}

type PerolaDatasetCatalogItem struct {
	ID                         string                                 `json:"id"`
	Label                      string                                 `json:"label"`
	Description                string                                 `json:"description"`
	DefaultLimit               int                                    `json:"defaultLimit"`
	MaxLimit                   int                                    `json:"maxLimit"`
	DefaultOrderBy             PerolaDatasetOrderInput                `json:"defaultOrderBy"`
	AllowedOrderFields         []string                               `json:"allowedOrderFields"`
	Filters                    []PerolaDatasetFilterCatalog           `json:"filters"`
	RequiredFilterRule         string                                 `json:"requiredFilterRule"`
	RequiredFilterAlternatives [][]PerolaDatasetFilterSelectorCatalog `json:"requiredFilterAlternatives"`
	DateRange                  *PerolaDatasetDateRangeCatalog         `json:"dateRange,omitempty"`
}

type PerolaDatasetFilterCatalog struct {
	Field     string   `json:"field"`
	ValueType string   `json:"valueType"`
	Operators []string `json:"operators"`
}

type PerolaDatasetFilterSelectorCatalog struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
}

type PerolaDatasetDateRangeCatalog struct {
	Field   string `json:"field"`
	MaxDays int    `json:"maxDays"`
}

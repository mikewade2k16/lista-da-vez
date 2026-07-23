package bi

import "encoding/json"

type Options struct {
	CompanyKey         string
	Login              string
	Pass               string
	StaticToken        string
	DefaultCNPJEmpresa string
	TokenTTL           string
	RequestTimeout     string
	PageLimit          int
	MaxPages           int
}

type PerolaLoginInput struct {
	CompanyKey  string `json:"companyKey"`
	CNPJEmpresa string `json:"cnpjEmpresa"`
	Login       string `json:"login"`
	Pass        string `json:"pass"`
}

type PerolaFindInput struct {
	Endpoint    string          `json:"endpoint"`
	CompanyKey  string          `json:"companyKey"`
	CNPJEmpresa string          `json:"cnpjEmpresa"`
	Token       string          `json:"token"`
	Body        json.RawMessage `json:"body"`
}

type PerolaOverviewInput struct {
	CompanyKey       string `json:"companyKey"`
	CNPJEmpresa      string `json:"cnpjEmpresa"`
	Token            string `json:"token"`
	IncludeInventory bool   `json:"includeInventory"`
}

type PerolaProxyResponse struct {
	OK                 bool           `json:"ok"`
	UpstreamStatus     int            `json:"upstreamStatus"`
	UpstreamStatusText string         `json:"upstreamStatusText"`
	UpstreamURL        string         `json:"upstreamUrl"`
	DurationMs         int64          `json:"durationMs"`
	Token              string         `json:"token,omitempty"`
	Headers            map[string]any `json:"headers,omitempty"`
	Body               any            `json:"body,omitempty"`
	RawBody            string         `json:"rawBody,omitempty"`
}

type PerolaOverviewResponse struct {
	OK          bool                        `json:"ok"`
	GeneratedAt string                      `json:"generatedAt"`
	CNPJEmpresa string                      `json:"cnpjEmpresa"`
	Metrics     []PerolaMetric              `json:"metrics"`
	Sources     []PerolaSource              `json:"sources"`
	Insights    []PerolaInsight             `json:"insights"`
	Sections    []PerolaIntelligenceSection `json:"sections"`
	Tables      []PerolaDataTable           `json:"tables"`
}

type PerolaMetric struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
	Tone   string `json:"tone,omitempty"`
}

type PerolaSource struct {
	Key            string `json:"key"`
	Label          string `json:"label"`
	Endpoint       string `json:"endpoint"`
	OK             bool   `json:"ok"`
	Pending        bool   `json:"pending,omitempty"`
	UpstreamStatus int    `json:"upstreamStatus"`
	Fetched        int    `json:"fetched"`
	Total          int    `json:"total"`
	DurationMs     int64  `json:"durationMs"`
	Truncated      bool   `json:"truncated,omitempty"`
	Error          string `json:"error,omitempty"`
}

type PerolaInsight struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Tone  string `json:"tone,omitempty"`
}

type PerolaDataTable struct {
	Key         string              `json:"key"`
	Label       string              `json:"label"`
	Description string              `json:"description"`
	Pending     bool                `json:"pending,omitempty"`
	Total       int                 `json:"total"`
	Fetched     int                 `json:"fetched"`
	Columns     []PerolaTableColumn `json:"columns"`
	Filters     []PerolaTableFilter `json:"filters"`
	Rows        []map[string]any    `json:"rows"`
}

type PerolaIntelligenceSection struct {
	Key     string                   `json:"key"`
	Title   string                   `json:"title"`
	Summary string                   `json:"summary"`
	Tone    string                   `json:"tone,omitempty"`
	Items   []PerolaIntelligenceItem `json:"items"`
}

type PerolaIntelligenceItem struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
	Tone   string `json:"tone,omitempty"`
}

type PerolaTableColumn struct {
	ID             string `json:"id"`
	Label          string `json:"label"`
	Width          string `json:"width,omitempty"`
	Align          string `json:"align,omitempty"`
	Locked         bool   `json:"locked,omitempty"`
	DefaultVisible bool   `json:"defaultVisible"`
	Description    string `json:"description,omitempty"`
}

type PerolaTableFilter struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Placeholder string   `json:"placeholder,omitempty"`
	Options     []string `json:"options"`
}

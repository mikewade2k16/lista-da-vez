package finance

import (
	"encoding/json"
	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/llm"
	"math/big"
	"net/url"
	"regexp"
	"time"
)

type ExplanationConfig struct {
	Enabled         bool                `json:"enabled"`
	CredentialID    string              `json:"credentialId"`
	Model           string              `json:"model"`
	TaskClass       string              `json:"taskClass"`
	MaxInputBytes   int                 `json:"maxInputBytes"`
	MaxOutputTokens int                 `json:"maxOutputTokens"`
	TimeoutMS       int                 `json:"timeoutMs"`
	DailyRequests   int                 `json:"dailyRequests"`
	MaxRunCostUSD   string              `json:"maxRunCostUsd"`
	Pricing         *ExplanationPricing `json:"pricing"`
}
type ExplanationPricing struct {
	Version             string `json:"version"`
	Source              string `json:"source"`
	ValidFrom           string `json:"validFrom"`
	ValidTo             string `json:"validTo"`
	InputUSDPerMillion  string `json:"inputUsdPerMillion"`
	OutputUSDPerMillion string `json:"outputUsdPerMillion"`
}
type PlusEvidence struct {
	DocumentID string          `json:"documentId"`
	Version    int             `json:"version"`
	ChunkID    string          `json:"chunkId"`
	Title      string          `json:"title"`
	Excerpt    string          `json:"excerpt"`
	SourceRef  string          `json:"sourceRef"`
	Applicable bool            `json:"applicable"`
	Location   json.RawMessage `json:"location"`
}
type GroundedAnswer struct {
	Answer      string   `json:"answer"`
	EvidenceIDs []string `json:"evidenceIds"`
	Assumptions []string `json:"assumptions"`
	MissingData []string `json:"missingData"`
	Coverage    string   `json:"coverage"`
}
type PlusRunUsage struct {
	RunID     string  `json:"runId"`
	Model     string  `json:"model"`
	Status    string  `json:"status"`
	Tokens    *int    `json:"tokens"`
	CostUSD   *string `json:"costUsd"`
	LatencyMS int     `json:"latencyMs"`
}

var usdPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,6})(\.[0-9]{1,6})?$`)

func usd(v string) (*big.Rat, bool) {
	if !usdPattern.MatchString(v) {
		return nil, false
	}
	r, ok := new(big.Rat).SetString(v)
	return r, ok
}
func validateExplanation(c ExplanationConfig) error {
	if !c.Enabled {
		return nil
	}
	if !isUUID(c.CredentialID) || c.Model == "" || len(c.Model) > 120 || c.TaskClass != "source_summary" || c.MaxInputBytes < 1000 || c.MaxInputBytes > 16000 || c.MaxOutputTokens < 128 || c.MaxOutputTokens > 2048 || c.TimeoutMS < 1000 || c.TimeoutMS > 15000 || c.DailyRequests < 1 || c.DailyRequests > 1000 {
		return invalidFiscal("Configure modelo, tarefa e limites das explicações Plus.")
	}
	if c.MaxRunCostUSD != "" {
		if _, ok := usd(c.MaxRunCostUSD); !ok {
			return invalidFiscal("Limite de custo inválido.")
		}
	}
	if p := c.Pricing; p != nil {
		source, sourceError := url.Parse(p.Source)
		if sourceError != nil || source.User != nil || source.Host == "" || source.RawQuery != "" || source.Fragment != "" || (source.Scheme != "https" && source.Scheme != "manual") {
			return invalidFiscal("Informe origem verificável da tabela de preços.")
		}
		from, e1 := time.Parse("2006-01-02", p.ValidFrom)
		to, e2 := time.Parse("2006-01-02", p.ValidTo)
		_, inOK := usd(p.InputUSDPerMillion)
		_, outOK := usd(p.OutputUSDPerMillion)
		if e1 != nil || e2 != nil || !from.Before(to) || !inOK || !outOK || p.Version == "" || len(p.Version) > 100 || len(p.Source) > 1000 {
			return invalidFiscal("Tabela de preço inválida: informe versão, fonte, valores e vigência.")
		}
	}
	return nil
}
func priceUSD(p *ExplanationPricing, input, output int, now time.Time) *string {
	if p == nil || input < 0 || output < 0 || now.UTC().Format("2006-01-02") < p.ValidFrom || now.UTC().Format("2006-01-02") >= p.ValidTo {
		return nil
	}
	a, ok := usd(p.InputUSDPerMillion)
	if !ok {
		return nil
	}
	b, ok := usd(p.OutputUSDPerMillion)
	if !ok {
		return nil
	}
	a.Mul(a, new(big.Rat).SetInt64(int64(input)))
	b.Mul(b, new(big.Rat).SetInt64(int64(output)))
	a.Add(a, b)
	// a is the number of micro-USD. Round up to avoid claiming a lower cost.
	units := new(big.Int).Quo(a.Num(), a.Denom())
	if new(big.Int).Mod(a.Num(), a.Denom()).Sign() > 0 {
		units.Add(units, big.NewInt(1))
	}
	v := new(big.Rat).SetFrac(units, big.NewInt(1000000)).FloatString(6)
	return &v
}
func explanationSchema() *llm.Schema {
	return &llm.Schema{Name: "finance_plus_source_summary", Version: 1, Definition: json.RawMessage(`{"type":"object","additionalProperties":false,"required":["answer","evidenceIds","assumptions","missingData","coverage"],"properties":{"answer":{"type":"string","minLength":1,"maxLength":3000},"evidenceIds":{"type":"array","maxItems":5,"items":{"type":"string","maxLength":36}},"assumptions":{"type":"array","maxItems":5,"items":{"type":"string","maxLength":300}},"missingData":{"type":"array","maxItems":8,"items":{"type":"string","maxLength":300}},"coverage":{"type":"string","enum":["informational","abstained"]}}}`)}
}

func defaultExplanationConfig() ExplanationConfig {
	return ExplanationConfig{TaskClass: "source_summary", MaxInputBytes: 8000, MaxOutputTokens: 512, TimeoutMS: 10000, DailyRequests: 20}
}

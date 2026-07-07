package finance

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// uuidPattern valida ids no formato uuid antes do cast ::uuid no SQL. Id invalido
// vira ” (o SQL entao gera um novo via gen_random_uuid()).
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// isUUID reporta se s ja e um uuid canonico (minusculo).
func isUUID(s string) bool { return uuidPattern.MatchString(s) }

// SheetStore abstrai a persistencia de planilhas.
type SheetStore interface {
	List(ctx context.Context, accountID string, f ListFilter) ([]SheetListItem, int, error)
	Get(ctx context.Context, accountID, id string) (SheetDetail, error)
	Create(ctx context.Context, accountID string, d SheetDetail) (SheetDetail, error)
	Update(ctx context.Context, accountID string, d SheetDetail) (SheetDetail, error)
	Delete(ctx context.Context, accountID, id string) error
	PatchLine(ctx context.Context, accountID, sheetID, lineID string, effective bool, effectiveDate string) (Line, string, error)
}

// ConfigStore abstrai a persistencia de config e o read model de recorrencias.
type ConfigStore interface {
	GetConfig(ctx context.Context, accountID, coreTenantID string) (ConfigData, error)
	SaveConfig(ctx context.Context, accountID string, d ConfigData) (ConfigData, error)
	ListRecurringClients(ctx context.Context) ([]RecurringClient, error)
}

// Service concentra as regras de negocio portadas do financeMockStore.ts.
type Service struct {
	sheets SheetStore
	config ConfigStore
}

// NewService injeta os stores.
func NewService(sheets SheetStore, config ConfigStore) *Service {
	return &Service{sheets: sheets, config: config}
}

// ============================================================================
// Normalizacao (espelha financeMockStore.ts: text/num/normalizeLine)
// ============================================================================

// normText colapsa whitespace, faz trim e trunca em max runes.
func normText(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > max {
		s = string([]rune(s)[:max])
	}
	return s
}

// round2 arredonda para 2 casas.
func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}

// num arredonda e, quando allowNegative=false, piso em 0.
func num(v float64, allowNegative bool) float64 {
	r := round2(v)
	if !allowNegative && r < 0 {
		return 0
	}
	return r
}

// normalizeID valida o id recebido; invalido vira ” (gera novo no store).
func normalizeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if isUUID(id) {
		return id
	}
	return ""
}

// normalizeDate garante YYYY-MM-DD ou ”.
func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) != 10 {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return ""
	}
	return s
}

// normalizeLine aplica os limites e regras do mock (adjustmentAmount = soma dos
// adjustments quando existirem; senao o valor recebido, negativo permitido).
func normalizeLine(l Line) Line {
	adjustments := make([]Adjustment, 0, len(l.Adjustments))
	var sum float64
	for _, a := range l.Adjustments {
		amt := num(a.Amount, true)
		adjustments = append(adjustments, Adjustment{
			ID:     normalizeID(a.ID),
			Amount: amt,
			Note:   normText(a.Note, 240),
			Date:   normalizeDate(a.Date),
		})
		sum += amt
	}
	adjustmentAmount := num(l.AdjustmentAmount, true)
	if len(adjustments) > 0 {
		adjustmentAmount = round2(sum)
	}
	return Line{
		ID:               normalizeID(l.ID),
		Description:      normText(l.Description, 260),
		Category:         normText(l.Category, 120),
		Effective:        l.Effective,
		EffectiveDate:    normalizeDate(l.EffectiveDate),
		Amount:           num(l.Amount, false),
		AdjustmentAmount: adjustmentAmount,
		Adjustments:      adjustments,
		FixedAccountID:   normText(l.FixedAccountID, 90),
		Details:          normText(l.Details, 600),
	}
}

// normalizeLines aplica normalizeLine e carimba kind + position.
func normalizeLines(lines []Line, kind string) []Line {
	out := make([]Line, 0, len(lines))
	for _, l := range lines {
		nl := normalizeLine(l)
		nl.Kind = kind
		out = append(out, nl)
	}
	return out
}

// ============================================================================
// Summary / Preview (espelha computeSummary/computePreview do mock)
// ============================================================================

func lineTotal(l Line) float64 { return l.Amount + l.AdjustmentAmount }

// computeSummary soma expected (tudo) e effective (so effective=true).
func computeSummary(entradas, saidas []Line) Summary {
	var expectedIn, effectiveIn, expectedOut, effectiveOut float64
	for _, l := range entradas {
		expectedIn += lineTotal(l)
		if l.Effective {
			effectiveIn += lineTotal(l)
		}
	}
	for _, l := range saidas {
		expectedOut += lineTotal(l)
		if l.Effective {
			effectiveOut += lineTotal(l)
		}
	}
	return Summary{
		ExpectedIn:       round2(expectedIn),
		EffectiveIn:      round2(effectiveIn),
		ExpectedOut:      round2(expectedOut),
		EffectiveOut:     round2(effectiveOut),
		ExpectedBalance:  round2(expectedIn - expectedOut),
		EffectiveBalance: round2(effectiveIn - effectiveOut),
	}
}

// formatAmount replica a interpolacao JS (numero sem casas fixas: 8000, 12.5).
func formatAmount(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// computePreview replica `Entradas x | Saidas y | Saldo z` do mock.
func computePreview(s Summary) string {
	return fmt.Sprintf("Entradas %s | Saidas %s | Saldo %s",
		formatAmount(s.EffectiveIn), formatAmount(s.EffectiveOut), formatAmount(s.EffectiveBalance))
}

// scopeKey normaliza coreTenantId (trim + lower).
func scopeKey(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// currentPeriod devolve o mes corrente YYYY-MM.
func currentPeriod() string { return time.Now().Format("2006-01") }

// normalizePeriod aceita apenas YYYY-MM.
func normalizePeriod(s string) string {
	s = strings.TrimSpace(s)
	if _, err := time.Parse("2006-01", s); err != nil {
		return ""
	}
	return s
}

// ============================================================================
// Sheets
// ============================================================================

// ListSheets lista planilhas da account com filtro e paginacao.
func (s *Service) ListSheets(ctx context.Context, accountID string, f ListFilter) ([]SheetListItem, ListMeta, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 {
		f.Limit = 240
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	f.CoreTenantID = scopeKey(f.CoreTenantID)
	f.Period = normalizePeriod(f.Period)
	f.Q = normText(f.Q, 120)

	items, total, err := s.sheets.List(ctx, accountID, f)
	if err != nil {
		return nil, ListMeta{}, err
	}
	totalPages := 0
	if f.Limit > 0 {
		totalPages = (total + f.Limit - 1) / f.Limit
	}
	meta := ListMeta{
		Page:       f.Page,
		Limit:      f.Limit,
		Total:      total,
		TotalPages: totalPages,
		HasMore:    f.Page < totalPages,
	}
	return items, meta, nil
}

// GetSheet devolve o detalhe (404 se fora da account).
func (s *Service) GetSheet(ctx context.Context, accountID, id string) (SheetDetail, error) {
	sid := normalizeID(id)
	if sid == "" {
		return SheetDetail{}, ErrSheetNotFound
	}
	return s.sheets.Get(ctx, accountID, sid)
}

// CreateSheet cria uma planilha com defaults do mock (title/period/status).
func (s *Service) CreateSheet(ctx context.Context, accountID string, in SheetInput) (SheetDetail, error) {
	period := normalizePeriod(strVal(in.Period))
	if period == "" {
		period = currentPeriod()
	}
	title := normText(strVal(in.Title), 180)
	if title == "" {
		title = "Finance " + period
	}
	status := normText(strVal(in.Status), 120)
	if status == "" {
		status = "aberta"
	}
	d := SheetDetail{
		SheetListItem: SheetListItem{
			Title:        title,
			Period:       period,
			Status:       status,
			Notes:        normText(strVal(in.Notes), 12000),
			CoreTenantID: scopeKey(strVal(in.CoreTenantID)),
		},
		Entradas: normalizeLines(in.Entradas, "entrada"),
		Saidas:   normalizeLines(in.Saidas, "saida"),
	}
	return s.sheets.Create(ctx, accountID, d)
}

// UpdateSheet faz o full-replace da planilha (404 se fora da account).
func (s *Service) UpdateSheet(ctx context.Context, accountID, id string, in SheetInput) (SheetDetail, error) {
	sid := normalizeID(id)
	if sid == "" {
		return SheetDetail{}, ErrSheetNotFound
	}
	period := normalizePeriod(strVal(in.Period))
	d := SheetDetail{
		SheetListItem: SheetListItem{
			ID:           sid,
			Title:        normText(strVal(in.Title), 180),
			Period:       period, // '' preserva o atual (resolvido no store)
			Status:       normText(strVal(in.Status), 120),
			Notes:        normText(strVal(in.Notes), 12000),
			CoreTenantID: scopeKey(strVal(in.CoreTenantID)),
		},
		Entradas: normalizeLines(in.Entradas, "entrada"),
		Saidas:   normalizeLines(in.Saidas, "saida"),
	}
	return s.sheets.Update(ctx, accountID, d)
}

// DeleteSheet remove a planilha (404 se fora da account).
func (s *Service) DeleteSheet(ctx context.Context, accountID, id string) error {
	sid := normalizeID(id)
	if sid == "" {
		return ErrSheetNotFound
	}
	return s.sheets.Delete(ctx, accountID, sid)
}

// PatchLine efetiva/des-efetiva uma linha e devolve linha + summary + preview
// recalculados. Des-efetivar limpa effectiveDate (regra do mock).
func (s *Service) PatchLine(ctx context.Context, accountID, sheetID, lineID string, in LinePatchInput) (LineMutationData, error) {
	sid := normalizeID(sheetID)
	if sid == "" {
		return LineMutationData{}, ErrSheetNotFound
	}
	// Le o estado atual para computar effective/effectiveDate finais.
	detail, err := s.sheets.Get(ctx, accountID, sid)
	if err != nil {
		return LineMutationData{}, err
	}
	target := findLine(detail, lineID)
	if target == nil {
		return LineMutationData{}, ErrLineNotFound
	}
	effective := target.Effective
	effectiveDate := target.EffectiveDate
	if in.Effective != nil {
		effective = *in.Effective
		if !effective {
			effectiveDate = ""
		}
	}
	if in.EffectiveDate != nil {
		effectiveDate = normalizeDate(*in.EffectiveDate)
	}
	if !effective {
		effectiveDate = ""
	}

	line, updatedAt, err := s.sheets.PatchLine(ctx, accountID, sid, target.ID, effective, effectiveDate)
	if err != nil {
		return LineMutationData{}, err
	}
	// Recalcula summary sobre o detalhe com a linha ja atualizada.
	applyLine(&detail, line)
	summary := computeSummary(detail.Entradas, detail.Saidas)
	return LineMutationData{
		SheetID:   sid,
		LineID:    line.ID,
		Line:      line,
		Summary:   summary,
		Preview:   computePreview(summary),
		UpdatedAt: updatedAt,
	}, nil
}

// findLine procura uma linha em entradas/saidas por id.
func findLine(d SheetDetail, lineID string) *Line {
	lid := strings.ToLower(strings.TrimSpace(lineID))
	for i := range d.Entradas {
		if d.Entradas[i].ID == lid {
			return &d.Entradas[i]
		}
	}
	for i := range d.Saidas {
		if d.Saidas[i].ID == lid {
			return &d.Saidas[i]
		}
	}
	return nil
}

// applyLine substitui a linha correspondente no detalhe (para recomputar summary).
func applyLine(d *SheetDetail, line Line) {
	for i := range d.Entradas {
		if d.Entradas[i].ID == line.ID {
			d.Entradas[i] = line
			return
		}
	}
	for i := range d.Saidas {
		if d.Saidas[i].ID == line.ID {
			d.Saidas[i] = line
			return
		}
	}
}

// strVal desreferencia um *string opcional para ” quando nil.
func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

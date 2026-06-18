package reports

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/auth"
	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/stores"
)

const (
	pauseReasonFallback = "Sem motivo"
	pauseKindPause      = "pause"
	pauseKindAssignment = "assignment"
)

// Pauses agrega as pausas (queue.operation_status_sessions, status='paused')
// das lojas acessiveis no escopo: contagem, duracao, motivos e distribuicao por
// hora do dia. Espelha o escopo/permissao dos demais relatorios.
func (service *Service) Pauses(ctx context.Context, principal auth.Principal, filters Filters) (PausesResponse, error) {
	if !canViewReports(principal) {
		return PausesResponse{}, stores.ErrForbidden
	}

	normalized, repositoryInput, err := normalizeFilters(filters)
	if err != nil {
		return PausesResponse{}, err
	}

	scopeStoreID, storeIDs, resolvedTenantID, err := service.resolvePauseScope(ctx, principal, normalized)
	if err != nil {
		return PausesResponse{}, err
	}
	normalized.TenantID = resolvedTenantID

	sessions, err := service.repository.ListPauseSessions(
		ctx,
		storeIDs,
		repositoryInput.FinishedAtFrom,
		repositoryInput.FinishedAtTo,
		normalized.ConsultantIDs,
	)
	if err != nil {
		return PausesResponse{}, err
	}

	response := buildPausesResponse(sessions)
	response.StoreID = scopeStoreID
	response.Filters = normalized
	return response, nil
}

func (service *Service) resolvePauseScope(
	ctx context.Context,
	principal auth.Principal,
	normalized Filters,
) (string, []string, string, error) {
	if normalized.StoreID != "" {
		store, err := service.storeFinder.FindAccessible(ctx, principal, normalized.StoreID)
		if err != nil {
			return "", nil, "", err
		}
		return store.ID, []string{store.ID}, store.TenantID, nil
	}

	resolvedTenantID := firstNonEmpty(normalized.TenantID, principal.TenantID)
	if resolvedTenantID == "" {
		return "", nil, "", ErrStoreRequired
	}

	storeRows, err := service.storeFinder.ListAccessible(ctx, principal, stores.ListInput{TenantID: resolvedTenantID})
	if err != nil {
		return "", nil, "", err
	}

	storeIDs := make([]string, 0, len(storeRows))
	for _, store := range storeRows {
		storeIDs = append(storeIDs, store.ID)
	}

	return "", storeIDs, resolvedTenantID, nil
}

type pauseConsultantAgg struct {
	id        string
	name      string
	count     int
	totalMs   int64
	byReason  map[string]*PauseReasonRow
	reasonSeq []string
}

func buildPausesResponse(sessions []PauseSessionRow) PausesResponse {
	rows := make([]PauseRow, 0, len(sessions))
	summary := PauseSummary{}
	distinctConsultants := map[string]struct{}{}

	consultants := map[string]*pauseConsultantAgg{}
	consultantSeq := make([]string, 0)
	reasonTotals := map[string]*PauseReasonRow{}
	reasonSeq := make([]string, 0)
	hourTotals := map[string]*PauseHourRow{}

	for _, session := range sessions {
		reason := normalizePauseReasonLabel(session.Reason)
		duration := maxInt64(session.DurationMs, 0)

		rows = append(rows, PauseRow{
			ConsultantID:   session.ConsultantID,
			ConsultantName: session.ConsultantName,
			Reason:         reason,
			Kind:           normalizePauseKindLabel(session.Kind),
			StartedAt:      session.StartedAt,
			EndedAt:        session.EndedAt,
			DurationMs:     duration,
		})

		summary.TotalPauses++
		summary.TotalDurationMs += duration
		distinctConsultants[session.ConsultantID] = struct{}{}

		accumulateConsultant(consultants, &consultantSeq, session, reason, duration)
		accumulateReason(reasonTotals, &reasonSeq, reason, duration)
		accumulateHour(hourTotals, session.StartedAt, duration)
	}

	summary.DistinctConsultants = len(distinctConsultants)
	if summary.TotalPauses > 0 {
		summary.AverageDurationMs = summary.TotalDurationMs / int64(summary.TotalPauses)
	}

	return PausesResponse{
		Summary:      summary,
		ByConsultant: buildConsultantRows(consultants, consultantSeq),
		ByReason:     buildReasonRows(reasonTotals, reasonSeq),
		ByHour:       buildHourRows(hourTotals),
		Rows:         rows,
	}
}

func accumulateConsultant(
	consultants map[string]*pauseConsultantAgg,
	consultantSeq *[]string,
	session PauseSessionRow,
	reason string,
	duration int64,
) {
	agg, ok := consultants[session.ConsultantID]
	if !ok {
		agg = &pauseConsultantAgg{
			id:       session.ConsultantID,
			name:     session.ConsultantName,
			byReason: map[string]*PauseReasonRow{},
		}
		consultants[session.ConsultantID] = agg
		*consultantSeq = append(*consultantSeq, session.ConsultantID)
	}
	if agg.name == "" {
		agg.name = session.ConsultantName
	}
	agg.count++
	agg.totalMs += duration

	reasonRow, ok := agg.byReason[reason]
	if !ok {
		reasonRow = &PauseReasonRow{Reason: reason}
		agg.byReason[reason] = reasonRow
		agg.reasonSeq = append(agg.reasonSeq, reason)
	}
	reasonRow.Count++
	reasonRow.TotalDurationMs += duration
}

func accumulateReason(reasonTotals map[string]*PauseReasonRow, reasonSeq *[]string, reason string, duration int64) {
	row, ok := reasonTotals[reason]
	if !ok {
		row = &PauseReasonRow{Reason: reason}
		reasonTotals[reason] = row
		*reasonSeq = append(*reasonSeq, reason)
	}
	row.Count++
	row.TotalDurationMs += duration
}

func accumulateHour(hourTotals map[string]*PauseHourRow, startedAt int64, duration int64) {
	hour := pauseHourLabel(startedAt)
	row, ok := hourTotals[hour]
	if !ok {
		row = &PauseHourRow{Hour: hour}
		hourTotals[hour] = row
	}
	row.Count++
	row.TotalDurationMs += duration
}

func buildConsultantRows(consultants map[string]*pauseConsultantAgg, consultantSeq []string) []PauseConsultantRow {
	result := make([]PauseConsultantRow, 0, len(consultantSeq))
	for _, id := range consultantSeq {
		agg := consultants[id]
		row := PauseConsultantRow{
			ConsultantID:    agg.id,
			ConsultantName:  agg.name,
			PauseCount:      agg.count,
			TotalDurationMs: agg.totalMs,
			ByReason:        make([]PauseReasonRow, 0, len(agg.reasonSeq)),
		}
		if agg.count > 0 {
			row.AverageDurationMs = agg.totalMs / int64(agg.count)
		}
		for _, reason := range agg.reasonSeq {
			row.ByReason = append(row.ByReason, *agg.byReason[reason])
		}
		sort.SliceStable(row.ByReason, func(i, j int) bool {
			return row.ByReason[i].Count > row.ByReason[j].Count
		})
		result = append(result, row)
	}

	sort.SliceStable(result, func(i, j int) bool {
		if result[i].TotalDurationMs == result[j].TotalDurationMs {
			return result[i].ConsultantName < result[j].ConsultantName
		}
		return result[i].TotalDurationMs > result[j].TotalDurationMs
	})
	return result
}

func buildReasonRows(reasonTotals map[string]*PauseReasonRow, reasonSeq []string) []PauseReasonRow {
	result := make([]PauseReasonRow, 0, len(reasonSeq))
	for _, reason := range reasonSeq {
		result = append(result, *reasonTotals[reason])
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Count == result[j].Count {
			return result[i].Reason < result[j].Reason
		}
		return result[i].Count > result[j].Count
	})
	return result
}

func buildHourRows(hourTotals map[string]*PauseHourRow) []PauseHourRow {
	result := make([]PauseHourRow, 0, len(hourTotals))
	for _, row := range hourTotals {
		result = append(result, *row)
	}
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Hour < result[j].Hour
	})
	return result
}

func normalizePauseReasonLabel(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return pauseReasonFallback
	}
	return trimmed
}

func normalizePauseKindLabel(kind string) string {
	if strings.TrimSpace(kind) == pauseKindAssignment {
		return pauseKindAssignment
	}
	return pauseKindPause
}

// pauseHourLabel devolve a hora do dia (UTC, "00".."23") do inicio da pausa.
// A tabela `rows` carrega os timestamps crus para o front formatar em hora local.
func pauseHourLabel(startedAt int64) string {
	return fmt.Sprintf("%02d", time.UnixMilli(startedAt).UTC().Hour())
}

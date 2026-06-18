package erp

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/queue/commission"
)

// crmGoalSet e o conjunto de metas (mensal/ticket/PA) de um consultor ou de uma loja.
type crmGoalSet struct {
	monthlyReais float64
	ticketCents  int64
	pa           float64
}

// crmPayoutInputs sao os insumos extras (carregados 1x por request, tenant-scoped)
// para o calculo de comissao embutido em GET /v1/erp/crm.
type crmPayoutInputs struct {
	policy          commission.Policy
	consultantGoals map[string]crmGoalSet // queue.consultants.id -> meta do consultor
	storeGoals      map[string]crmGoalSet // queue.stores.id -> meta de LOJA (consultant_id null)
}

// loadCRMPayoutInputs carrega a politica (settings) e as metas do mes-alvo
// (queue.operation_goal_targets). Tudo tenant-scoped (where tenant_id = $1) e em
// poucas queries — sem N+1.
func (repository *PostgresRepository) loadCRMPayoutInputs(ctx context.Context, store StoreScope, query CRMOverviewQuery) (crmPayoutInputs, error) {
	policy, err := repository.loadCRMGoalPayoutPolicy(ctx, store.TenantID)
	if err != nil {
		return crmPayoutInputs{}, err
	}

	consultantGoals, storeGoals, err := repository.loadCRMGoalTargets(ctx, store.TenantID, crmPayoutTargetMonth(query))
	if err != nil {
		return crmPayoutInputs{}, err
	}

	return crmPayoutInputs{
		policy:          policy,
		consultantGoals: consultantGoals,
		storeGoals:      storeGoals,
	}, nil
}

func (repository *PostgresRepository) loadCRMGoalPayoutPolicy(ctx context.Context, tenantID string) (commission.Policy, error) {
	var raw *string
	err := repository.pool.QueryRow(ctx, `
		select crm_goal_payout_policy::text
		from tenant_operation_core_settings
		where tenant_id = $1::uuid
		limit 1;
	`, tenantID).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return commission.DefaultPolicy(), nil
		}
		return commission.Policy{}, err
	}

	if raw == nil || strings.TrimSpace(*raw) == "" {
		return commission.DefaultPolicy(), nil
	}

	policy, err := commission.ParsePolicyJSON([]byte(*raw))
	if err != nil {
		return commission.DefaultPolicy(), nil
	}
	return policy, nil
}

// loadCRMGoalTargets devolve as metas do mes-alvo a partir de queue.operation_goal_targets
// (FONTE UNICA das metas), separando metas POR CONSULTOR (consultant_id preenchido) das
// metas de LOJA (consultant_id null, herdadas pelos consultores). Batch por tenant_id (sem N+1).
func (repository *PostgresRepository) loadCRMGoalTargets(ctx context.Context, tenantID string, targetMonth time.Time) (map[string]crmGoalSet, map[string]crmGoalSet, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			consultant_id::text,
			store_id::text,
			coalesce(monthly_goal, 0)::float8,
			coalesce(avg_ticket_goal, 0)::float8,
			coalesce(pa_goal, 0)::float8
		from queue.operation_goal_targets
		where tenant_id = $1::uuid
		  and target_month = $2::date;
	`, tenantID, targetMonth)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	consultantGoals := make(map[string]crmGoalSet)
	storeGoals := make(map[string]crmGoalSet)
	for rows.Next() {
		var consultantID *string
		var storeID string
		var monthly, ticket, pa float64
		if err := rows.Scan(&consultantID, &storeID, &monthly, &ticket, &pa); err != nil {
			return nil, nil, err
		}
		goal := crmGoalSet{monthlyReais: monthly, ticketCents: reaisToCents(ticket), pa: pa}
		switch {
		case consultantID != nil && strings.TrimSpace(*consultantID) != "":
			consultantGoals[strings.TrimSpace(*consultantID)] = goal
		case strings.TrimSpace(storeID) != "":
			storeGoals[strings.TrimSpace(storeID)] = goal
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return consultantGoals, storeGoals, nil
}

// applyCRMPayouts anexa o payout de gerente/caixa por loja e o payout por
// consultor ao response, usando commission.Calculate. Mutates response in-place.
//
// Gate da loja: o storeProgress usado aqui ESPELHA a tela
// (useConsultantIntegratedRows): meta da loja = monthly_goal cadastrado da loja
// OU, na falta, a SOMA das metas dos consultores. Sem esse fallback, lojas sem
// meta propria cadastrada ficavam em 0% e o gate derrubava consultor E gerente.
func applyCRMPayouts(response *CRMOverviewResponse, inputs crmPayoutInputs) {
	// slug <-> storeId interno (UUID) + contagem de consultores por loja (para dividir
	// a meta de loja igualmente quando o consultor nao tem meta propria).
	storeIDBySlug := make(map[string]string)
	consultantCountByStore := make(map[string]int)
	for index := range response.Consultants {
		consultant := &response.Consultants[index]
		storeID := strings.TrimSpace(consultant.ProfileStoreID)
		if storeID == "" {
			continue
		}
		storeIDBySlug[consultant.StoreSlug] = storeID
		consultantCountByStore[storeID]++
	}

	// Soma EFETIVA das metas mensais por loja (meta propria do consultor OU a fracao
	// da meta de loja) — usada como meta da loja no gate quando nao ha meta de loja.
	consultantMonthlyByStore := make(map[string]float64)
	for index := range response.Consultants {
		consultant := &response.Consultants[index]
		storeID := strings.TrimSpace(consultant.ProfileStoreID)
		consultantMonthlyByStore[storeID] += resolveConsultantMonthlyGoal(
			consultant, inputs, consultantCountByStore[storeID],
		)
	}

	storeSoldBySlug := make(map[string]float64, len(response.Stores))
	storeProgressBySlug := make(map[string]float64, len(response.Stores))
	storeBySlug := make(map[string]*CRMStoreMetric, len(response.Stores))
	for index := range response.Stores {
		store := &response.Stores[index]
		store.StoreType = normalizeStoreType(store.StoreType)
		storeID := storeIDBySlug[store.StoreSlug]

		storeSold := centsToReais(store.SalesCents)
		sg := inputs.storeGoals[storeID]
		storeGoal := sg.monthlyReais
		if storeGoal <= 0 {
			storeGoal = consultantMonthlyByStore[storeID]
		}
		storeProgress := 0.0
		if storeGoal > 0 {
			storeProgress = storeSold / storeGoal * 100
		}

		storeSoldBySlug[store.StoreSlug] = storeSold
		storeProgressBySlug[store.StoreSlug] = storeProgress
		storeBySlug[store.StoreSlug] = store

		// Expoe o progresso EFETIVO (mesmo do gate) para o front exibir o mesmo numero.
		store.StoreSold = storeSold
		store.StoreGoal = storeGoal
		store.StoreProgress = storeProgress

		// Flags de gap: de onde veio a meta + quais configs faltam (sem recalcular).
		store.MissingStoreGoal = sg.monthlyReais <= 0
		switch {
		case sg.monthlyReais > 0:
			store.StoreGoalSource = "own"
		case consultantMonthlyByStore[storeID] > 0:
			store.StoreGoalSource = "consultant-sum"
		default:
			store.StoreGoalSource = "none"
		}
		store.MissingTicketGoal = sg.ticketCents <= 0
		store.MissingPaGoal = sg.pa <= 0
		store.SplitConsultantCount = consultantCountByStore[storeID]

		applyStorePayouts(store, inputs.policy, storeSold, storeProgress)
	}

	for index := range response.Consultants {
		consultant := &response.Consultants[index]
		storeID := strings.TrimSpace(consultant.ProfileStoreID)
		applyConsultantPayout(
			consultant,
			storeBySlug[consultant.StoreSlug],
			storeSoldBySlug[consultant.StoreSlug],
			storeProgressBySlug[consultant.StoreSlug],
			consultantCountByStore[storeID],
			inputs,
		)
	}
}

// resolveConsultantMonthlyGoal: meta mensal do consultor = a propria meta cadastrada
// OU, na falta, a meta de LOJA dividida igualmente entre os consultores da loja.
func resolveConsultantMonthlyGoal(consultant *CRMConsultantMetric, inputs crmPayoutInputs, countInStore int) float64 {
	if cg := inputs.consultantGoals[strings.TrimSpace(consultant.ProfileConsultantID)]; cg.monthlyReais > 0 {
		return cg.monthlyReais
	}
	sg := inputs.storeGoals[strings.TrimSpace(consultant.ProfileStoreID)]
	if sg.monthlyReais > 0 && countInStore > 0 {
		return sg.monthlyReais / float64(countInStore)
	}
	return 0
}

func applyStorePayouts(store *CRMStoreMetric, policy commission.Policy, storeSold, storeProgress float64) {
	if !store.Mapped {
		return
	}

	managerResult := commission.Calculate(commission.Input{
		Role:          "gerente",
		StoreType:     store.StoreType,
		StoreSold:     storeSold,
		StoreProgress: storeProgress,
		Policy:        policy,
	})
	store.ManagerPayout = toPayoutStore(managerResult)

	supportResult := commission.Calculate(commission.Input{
		Role:          "caixa",
		StoreType:     store.StoreType,
		StoreSold:     storeSold,
		StoreProgress: storeProgress,
		Policy:        policy,
	})
	store.SupportPayout = toPayoutStore(supportResult)
}

func applyConsultantPayout(
	consultant *CRMConsultantMetric,
	store *CRMStoreMetric,
	storeSold float64,
	storeProgress float64,
	countInStore int,
	inputs crmPayoutInputs,
) {
	storeType := ""
	if store != nil {
		storeType = store.StoreType
	}

	consultantSold := centsToReais(consultant.SalesCents)

	cg := inputs.consultantGoals[strings.TrimSpace(consultant.ProfileConsultantID)]
	sg := inputs.storeGoals[strings.TrimSpace(consultant.ProfileStoreID)]

	// HERANCA: meta mensal = propria OU fracao da loja; ticket/PA = proprios OU os da LOJA.
	monthlyGoal := resolveConsultantMonthlyGoal(consultant, inputs, countInStore)
	ticketGoalCents := cg.ticketCents
	if ticketGoalCents <= 0 {
		ticketGoalCents = sg.ticketCents
	}
	paGoal := cg.pa
	if paGoal <= 0 {
		paGoal = sg.pa
	}

	consultant.MonthlyGoalCents = reaisToCents(monthlyGoal)
	consultant.AvgTicketGoalCents = ticketGoalCents
	consultant.PAGoal = paGoal
	if monthlyGoal > 0 {
		consultant.GoalProgress = consultantSold / monthlyGoal * 100
	}

	// Flags de gap: de onde veio a meta mensal + quais metas faltam (sem recalcular).
	own := inputs.consultantGoals[strings.TrimSpace(consultant.ProfileConsultantID)].monthlyReais > 0
	switch {
	case own:
		consultant.GoalSource = "own"
	case monthlyGoal > 0:
		consultant.GoalSource = "store-split"
	default:
		consultant.GoalSource = "none"
	}
	consultant.MissingMonthlyGoal = !own
	consultant.MissingTicketGoal = ticketGoalCents <= 0
	consultant.MissingPaGoal = paGoal <= 0

	result := commission.Calculate(commission.Input{
		Role:               firstNonEmpty(consultant.ProfileConsultantName, consultant.ConsultantName),
		StoreType:          storeType,
		StoreSold:          storeSold,
		StoreProgress:      storeProgress,
		ConsultantSold:     consultantSold,
		ConsultantProgress: consultant.GoalProgress,
		HitPa:              crmHitMetric(paGoal, consultant.PAScore),
		HitTicket:          crmHitMetric(centsToReais(ticketGoalCents), centsToReais(consultant.TicketAverageCents)),
		Policy:             inputs.policy,
	})

	consultant.Payout = &PayoutConsultant{
		Amount:         result.Amount,
		RatePercent:    result.RatePercent,
		Base:           result.Base,
		Group:          string(result.Group),
		RuleLabel:      result.RuleLabel,
		PenaltyApplied: result.PenaltyApplied,
	}
}

func toPayoutStore(result commission.Result) *PayoutStore {
	return &PayoutStore{
		Amount:      result.Amount,
		RatePercent: result.RatePercent,
		RuleLabel:   result.RuleLabel,
	}
}

// crmHitMetric: meta>0 ? score>=meta : true (sem meta cadastrada, considera atingida).
func crmHitMetric(goal float64, score float64) bool {
	if goal > 0 {
		return score >= goal
	}
	return true
}

func crmPayoutTargetMonth(query CRMOverviewQuery) time.Time {
	reference := time.Now().UTC()
	if !query.DateFrom.IsZero() {
		reference = query.DateFrom.UTC()
	}
	return time.Date(reference.Year(), reference.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func normalizeStoreType(storeType string) string {
	if strings.EqualFold(strings.TrimSpace(storeType), "shopping") {
		return "shopping"
	}
	return "bairro"
}

func centsToReais(cents int64) float64 {
	return float64(cents) / 100
}

func reaisToCents(reais float64) int64 {
	return int64(reais*100 + 0.5)
}

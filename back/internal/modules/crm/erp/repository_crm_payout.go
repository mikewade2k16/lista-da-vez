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

	consultantGoals, storeGoals, err := repository.loadCRMGoalTargets(ctx, store.TenantID, crmPayoutTargetMonth(query), crmPayoutTargetWeek(query))
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

// loadCRMGoalTargets devolve as metas EFETIVAS do periodo-alvo a partir de
// queue.operation_goal_targets (FONTE UNICA das metas), separando metas POR CONSULTOR
// (consultant_id preenchido) das metas de LOJA (consultant_id null). Batch por
// tenant_id (sem N+1): carrega o mes (week=0) + as 4 semanas.
//
// Regra 1 (mes <-> semanas), aplicada em resolveEffectiveGoals:
//   - view MENSAL (week=0): se ALGUMA semana tem meta (>0) => meta do periodo = SOMA
//     das 4 semanas; senao => a meta mensal cadastrada.
//   - view SEMANA (week=N): meta da semana N se cadastrada (>0); senao => a mensal
//     DIVIDIDA IGUALMENTE por 4.
func (repository *PostgresRepository) loadCRMGoalTargets(ctx context.Context, tenantID string, targetMonth time.Time, week int) (map[string]crmGoalSet, map[string]crmGoalSet, error) {
	monthConsultant, monthStore, err := repository.loadGoalTargetsForWeek(ctx, tenantID, targetMonth, 0)
	if err != nil {
		return nil, nil, err
	}

	weekConsultant := make(map[int]map[string]crmGoalSet, 4)
	weekStore := make(map[int]map[string]crmGoalSet, 4)
	for w := 1; w <= 4; w++ {
		consultant, store, err := repository.loadGoalTargetsForWeek(ctx, tenantID, targetMonth, w)
		if err != nil {
			return nil, nil, err
		}
		weekConsultant[w] = consultant
		weekStore[w] = store
	}

	return resolveEffectiveGoals(monthConsultant, weekConsultant, week),
		resolveEffectiveGoals(monthStore, weekStore, week),
		nil
}

func (repository *PostgresRepository) loadGoalTargetsForWeek(ctx context.Context, tenantID string, targetMonth time.Time, week int) (map[string]crmGoalSet, map[string]crmGoalSet, error) {
	rows, err := repository.pool.Query(ctx, `
		select
			consultant_id::text,
			store_id::text,
			coalesce(monthly_goal, 0)::float8,
			coalesce(avg_ticket_goal, 0)::float8,
			coalesce(pa_goal, 0)::float8
		from queue.operation_goal_targets
		where tenant_id = $1::uuid
		  and target_month = $2::date
		  and week = $3;
	`, tenantID, targetMonth, week)
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

// resolveEffectiveGoals aplica a Regra 1 (mes <-> semanas) por chave (loja ou
// consultor). Ticket/PA sao medias por pedido — NAO somam/dividem; usam o valor mais
// especifico disponivel (semana N, senao mensal, senao qualquer semana com valor).
func resolveEffectiveGoals(monthMap map[string]crmGoalSet, weekMaps map[int]map[string]crmGoalSet, targetWeek int) map[string]crmGoalSet {
	keys := make(map[string]struct{}, len(monthMap))
	for key := range monthMap {
		keys[key] = struct{}{}
	}
	for w := 1; w <= 4; w++ {
		for key := range weekMaps[w] {
			keys[key] = struct{}{}
		}
	}

	out := make(map[string]crmGoalSet, len(keys))
	for key := range keys {
		monthGoal := monthMap[key]
		sumWeeks := 0.0
		anyWeek := false
		for w := 1; w <= 4; w++ {
			if wg := weekMaps[w][key]; wg.monthlyReais > 0 {
				anyWeek = true
				sumWeeks += wg.monthlyReais
			}
		}

		eff := crmGoalSet{}
		if targetWeek <= 0 {
			// Mensal: soma das semanas quando cadastradas; senao a mensal.
			if anyWeek {
				eff.monthlyReais = sumWeeks
			} else {
				eff.monthlyReais = monthGoal.monthlyReais
			}
			eff.ticketCents = monthGoal.ticketCents
			eff.pa = monthGoal.pa
			// Sem ticket/PA mensal cadastrado, herda o de alguma semana.
			for w := 1; w <= 4 && (eff.ticketCents <= 0 || eff.pa <= 0); w++ {
				wg := weekMaps[w][key]
				if eff.ticketCents <= 0 && wg.ticketCents > 0 {
					eff.ticketCents = wg.ticketCents
				}
				if eff.pa <= 0 && wg.pa > 0 {
					eff.pa = wg.pa
				}
			}
		} else {
			// Semana N: a da semana se cadastrada; senao a mensal dividida por 4.
			wg := weekMaps[targetWeek][key]
			if wg.monthlyReais > 0 {
				eff.monthlyReais = wg.monthlyReais
			} else {
				eff.monthlyReais = monthGoal.monthlyReais / 4
			}
			eff.ticketCents = firstPositiveCents(wg.ticketCents, monthGoal.ticketCents)
			eff.pa = firstPositive(wg.pa, monthGoal.pa)
		}
		out[key] = eff
	}
	return out
}

func firstPositive(values ...float64) float64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func firstPositiveCents(values ...int64) int64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
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
	// Regra 2: distribuicao da meta da loja entre consultores. Pre-computa por loja a
	// meta da loja + soma das metas explicitas + qtd de consultores sem meta propria.
	storeDist := make(map[string]storeConsultantDist)
	for storeID, storeGoal := range inputs.storeGoals {
		dist := storeDist[storeID]
		dist.storeGoal = storeGoal.monthlyReais
		storeDist[storeID] = dist
	}
	for index := range response.Consultants {
		consultant := &response.Consultants[index]
		storeID := strings.TrimSpace(consultant.ProfileStoreID)
		if storeID == "" {
			continue
		}
		dist := storeDist[storeID]
		if own := inputs.consultantGoals[strings.TrimSpace(consultant.ProfileConsultantID)].monthlyReais; own > 0 {
			dist.sumExplicit += own
		} else {
			dist.countWithoutOwn++
		}
		storeDist[storeID] = dist
	}

	consultantMonthlyByStore := make(map[string]float64)
	for index := range response.Consultants {
		consultant := &response.Consultants[index]
		storeID := strings.TrimSpace(consultant.ProfileStoreID)
		consultantMonthlyByStore[storeID] += resolveConsultantMonthlyGoal(
			consultant, inputs, storeDist[storeID],
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

		// Fonte unica: espelha a meta EFETIVA (com fallback consultant-sum) nos campos
		// que o painel consome (monthlyGoalCents/goalProgress/remaining), para o front
		// nao precisar re-mesclar /v1/operations/goals por fora — merge que zerava a meta
		// quando ela vinha do fallback consultant-sum.
		store.MonthlyGoalCents = reaisToCents(storeGoal)
		store.GoalProgress = storeProgress
		store.RemainingToGoalCents = maxCRMRemaining(store.MonthlyGoalCents, store.SalesCents)

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

	// Consolida a meta do summary a partir das metas EFETIVAS de loja (mapeadas), agora
	// que cada loja carrega monthlyGoalCents. Mesmo criterio do calculo original (soma so
	// mapeadas) e recomputa progresso/faltante — fonte unica, sem merge no front.
	var summaryGoalCents int64
	for index := range response.Stores {
		if response.Stores[index].Mapped {
			summaryGoalCents += response.Stores[index].MonthlyGoalCents
		}
	}
	response.Summary.MonthlyGoalCents = summaryGoalCents
	if summaryGoalCents > 0 {
		response.Summary.GoalProgress = float64(response.Summary.SalesCents) / float64(summaryGoalCents) * 100
		response.Summary.RemainingToGoalCents = maxCRMRemaining(summaryGoalCents, response.Summary.SalesCents)
	} else {
		response.Summary.GoalProgress = 0
		response.Summary.RemainingToGoalCents = 0
	}

	for index := range response.Consultants {
		consultant := &response.Consultants[index]
		storeID := strings.TrimSpace(consultant.ProfileStoreID)
		applyConsultantPayout(
			consultant,
			storeBySlug[consultant.StoreSlug],
			storeSoldBySlug[consultant.StoreSlug],
			storeProgressBySlug[consultant.StoreSlug],
			storeDist[storeID],
			inputs,
		)
	}
}

// storeConsultantDist sao os insumos da Regra 2 (distribuicao da meta da loja entre
// consultores), pre-computados por loja: meta da loja, soma das metas EXPLICITAS dos
// consultores e quantos consultores estao SEM meta propria.
type storeConsultantDist struct {
	storeGoal       float64
	sumExplicit     float64
	countWithoutOwn int
}

// resolveConsultantMonthlyGoal (Regra 2): meta mensal do consultor = a propria meta
// cadastrada; na falta, divide IGUALMENTE o que SOBRA da meta da loja (meta da loja -
// soma das metas explicitas dos consultores) entre os consultores SEM meta propria.
// Clamp em 0 (o restante nunca fica negativo).
func resolveConsultantMonthlyGoal(consultant *CRMConsultantMetric, inputs crmPayoutInputs, dist storeConsultantDist) float64 {
	if cg := inputs.consultantGoals[strings.TrimSpace(consultant.ProfileConsultantID)]; cg.monthlyReais > 0 {
		return cg.monthlyReais
	}
	if dist.storeGoal > 0 && dist.countWithoutOwn > 0 {
		remaining := dist.storeGoal - dist.sumExplicit
		if remaining < 0 {
			remaining = 0
		}
		return remaining / float64(dist.countWithoutOwn)
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
	dist storeConsultantDist,
	inputs crmPayoutInputs,
) {
	storeType := ""
	if store != nil {
		storeType = store.StoreType
	}

	consultantSold := centsToReais(consultant.SalesCents)

	cg := inputs.consultantGoals[strings.TrimSpace(consultant.ProfileConsultantID)]
	sg := inputs.storeGoals[strings.TrimSpace(consultant.ProfileStoreID)]

	// Meta mensal = propria OU resto rateado da loja (Regra 2); ticket/PA = proprios OU os da LOJA.
	monthlyGoal := resolveConsultantMonthlyGoal(consultant, inputs, dist)
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

// crmPayoutTargetWeek deriva a semana (0=mes inteiro; 1..4) a partir do range da
// query, casando com as fatias fixas do mes (S1=1-7, S2=8-14, S3=15-21, S4=22-fim).
// Range que nao bate exatamente uma fatia (mes inteiro, custom, ou cruzando meses)
// => 0 (mensal). Espelha buildMonthWeekRange do front.
func crmPayoutTargetWeek(query CRMOverviewQuery) int {
	if query.DateFrom.IsZero() || query.DateTo.IsZero() {
		return 0
	}
	from := query.DateFrom.UTC()
	to := query.DateTo.UTC()
	if from.Year() != to.Year() || from.Month() != to.Month() {
		return 0
	}
	lastDay := time.Date(from.Year(), from.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
	for week := 1; week <= 4; week++ {
		startDay := (week-1)*7 + 1
		endDay := week * 7
		if week == 4 {
			endDay = lastDay
		}
		if from.Day() == startDay && to.Day() == endDay {
			return week
		}
	}
	return 0
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

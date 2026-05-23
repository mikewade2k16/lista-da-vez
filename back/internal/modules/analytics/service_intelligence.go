package analytics

import (
	"fmt"
	"strings"

	"github.com/mikewade2k16/lista-da-vez/back/internal/modules/operations"
)

func buildOperationalIntelligence(data bundle) IntelligenceResponse {
	return buildOperationalIntelligenceSummary(data.storeID, data.tenantID, data.history, buildTimeIntelligence(data))
}

func buildOperationalIntelligenceSummary(storeID string, tenantID string, history []operations.ServiceHistoryEntry, timeIntelligence TimeIntelligence) IntelligenceResponse {
	totalAttendances := len(history)
	conversions := 0
	soldValue := 0.0
	noSales := 0
	for _, entry := range history {
		if isSaleOutcome(entry.FinishOutcome) {
			conversions++
			soldValue += maxFloat(entry.SaleAmount, 0)
		} else if strings.TrimSpace(entry.FinishOutcome) == "nao-compra" {
			noSales++
		}
	}

	conversionRate := 0.0
	if totalAttendances > 0 {
		conversionRate = (float64(conversions) / float64(totalAttendances)) * 100
	}

	ticketAverage := 0.0
	if conversions > 0 {
		ticketAverage = soldValue / float64(conversions)
	}

	quickNoSaleRate := 0.0
	longNoSaleRate := 0.0
	quickCloseRate := 0.0
	longLowSaleRate := 0.0
	queueToServiceRatio := 0.0
	idleVsServiceRatio := 0.0

	if noSales > 0 {
		quickNoSaleRate = (float64(timeIntelligence.QuickNoSaleCount) / float64(noSales)) * 100
		longNoSaleRate = (float64(timeIntelligence.LongNoSaleCount) / float64(noSales)) * 100
	}
	if conversions > 0 {
		quickCloseRate = (float64(timeIntelligence.QuickHighPotentialCount) / float64(conversions)) * 100
		longLowSaleRate = (float64(timeIntelligence.LongLowSaleCount) / float64(conversions)) * 100
	}
	if timeIntelligence.ConsultantsInServiceMs > 0 {
		queueToServiceRatio = float64(timeIntelligence.ConsultantsInQueueMs) / float64(timeIntelligence.ConsultantsInServiceMs)
	}
	if timeIntelligence.TotalsByStatus.Service > 0 {
		idleVsServiceRatio = float64(timeIntelligence.TotalsByStatus.Available) / float64(timeIntelligence.TotalsByStatus.Service)
	}

	diagnosis := make([]IntelligenceDiagnosis, 0, 6)
	appendDiagnosis := func(item IntelligenceDiagnosis) {
		diagnosis = append(diagnosis, item)
	}

	if totalAttendances < 6 {
		appendDiagnosis(IntelligenceDiagnosis{
			ID:         "sample-size",
			Level:      "attention",
			Title:      "Base ainda pequena para conclusoes fortes",
			Reading:    fmt.Sprintf("%d atendimentos registrados ate agora.", totalAttendances),
			Hypothesis: "A amostra ainda pode distorcer as leituras de tempo e conversao.",
			Action:     "Coletar mais atendimentos antes de tomar decisoes estruturais.",
		})
	}

	switch {
	case timeIntelligence.NotUsingQueueRate >= 25:
		appendDiagnosis(IntelligenceDiagnosis{"queue-discipline", "critical", "Uso da fila comprometido", fmt.Sprintf("%s dos atendimentos foram fora da vez.", formatPercent(timeIntelligence.NotUsingQueueRate)), "A regra da fila pode estar sendo ignorada com frequencia.", "Reforcar criterio para furar fila e auditar motivos por consultor diariamente."})
	case timeIntelligence.NotUsingQueueRate >= 12:
		appendDiagnosis(IntelligenceDiagnosis{"queue-discipline", "attention", "Uso da fila acima do ideal", fmt.Sprintf("%s dos atendimentos foram fora da vez.", formatPercent(timeIntelligence.NotUsingQueueRate)), "Pode haver excesso de excecoes no fluxo da loja.", "Acompanhar quem mais fura fila e validar se os motivos fazem sentido."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"queue-discipline", "healthy", "Disciplina de fila estavel", fmt.Sprintf("Atendimento fora da vez em %s.", formatPercent(timeIntelligence.NotUsingQueueRate)), "As excecoes estao sob controle operacional.", "Manter monitoramento dos motivos para manter consistencia."})
	}

	switch {
	case queueToServiceRatio >= 1.2 && timeIntelligence.ConsultantsInQueueMs >= 20*60000:
		appendDiagnosis(IntelligenceDiagnosis{"live-backlog", "critical", "Backlog atual de fila elevado", fmt.Sprintf("Fila atual acumulada %s vs atendimento atual %s.", formatDurationMinutes(timeIntelligence.ConsultantsInQueueMs), formatDurationMinutes(timeIntelligence.ConsultantsInServiceMs)), "Equipe pode estar sem tracao de inicio de atendimento no momento.", "Acionar lider para redistribuir entrada em atendimento nos proximos minutos."})
	case queueToServiceRatio >= 0.7 && timeIntelligence.ConsultantsInQueueMs >= 10*60000:
		appendDiagnosis(IntelligenceDiagnosis{"live-backlog", "attention", "Fila atual crescendo", fmt.Sprintf("Fila atual acumulada %s.", formatDurationMinutes(timeIntelligence.ConsultantsInQueueMs)), "A demanda atual pode estar maior que a capacidade ativa.", "Priorizar chamadas da fila e reduzir pausas nao essenciais."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"live-backlog", "healthy", "Ritmo atual equilibrado", fmt.Sprintf("Fila atual em %s.", formatDurationMinutes(timeIntelligence.ConsultantsInQueueMs)), "Fluxo atual entre espera e atendimento esta proporcional.", "Manter ritmo de chamada e monitorar picos por horario."})
	}

	switch {
	case quickNoSaleRate >= 45:
		appendDiagnosis(IntelligenceDiagnosis{"quick-no-sale", "critical", "Nao compra muito rapida", fmt.Sprintf("%s dos nao fechamentos encerram muito rapido.", formatPercent(quickNoSaleRate)), "Abordagem inicial pode estar curta, sem exploracao de oportunidade.", "Testar script de descoberta de motivo e sugestao de 2a opcao antes de encerrar."})
	case quickNoSaleRate >= 25:
		appendDiagnosis(IntelligenceDiagnosis{"quick-no-sale", "attention", "Nao compra rapida em alta", fmt.Sprintf("%s dos nao fechamentos foram rapidos.", formatPercent(quickNoSaleRate)), "Pode haver descarte precoce de atendimento.", "Acompanhar atendimentos curtos e criar checklist minimo antes de encerrar."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"quick-no-sale", "healthy", "Tempo minimo de exploracao razoavel", fmt.Sprintf("%s dos nao fechamentos foram rapidos.", formatPercent(quickNoSaleRate)), "A equipe tende a investigar melhor antes de encerrar sem venda.", "Manter rotina de registro do motivo de nao compra."})
	}

	switch {
	case longLowSaleRate >= 30 || longNoSaleRate >= 35:
		appendDiagnosis(IntelligenceDiagnosis{"long-service-low-return", "critical", "Atendimento longo com retorno baixo", fmt.Sprintf("%s de vendas longas com ticket baixo e %s de nao compra longa.", formatPercent(longLowSaleRate), formatPercent(longNoSaleRate)), "Tempo alto sem progresso pode indicar baixa objetividade na conducao.", "Criar checkpoints de 5 em 5 minutos para avancar proposta, upsell ou encerramento."})
	case longLowSaleRate >= 18 || longNoSaleRate >= 20:
		appendDiagnosis(IntelligenceDiagnosis{"long-service-low-return", "attention", "Parte dos atendimentos longos sem retorno", fmt.Sprintf("%s de vendas longas com ticket baixo.", formatPercent(longLowSaleRate)), "Existe espaco para melhorar conducao de atendimento demorado.", "Revisar casos de maior duracao e mapear pontos de trava."})
	default:
		appendDiagnosis(IntelligenceDiagnosis{"long-service-low-return", "healthy", "Duracao e retorno em equilibrio", "Nao houve excesso relevante de atendimento longo com baixo retorno.", "O tempo investido esta proporcional ao resultado comercial.", "Continuar monitorando para manter estabilidade."})
	}

	if quickCloseRate >= 45 {
		appendDiagnosis(IntelligenceDiagnosis{"quick-close", "attention", "Fechamento rapido em excesso", fmt.Sprintf("%s das conversoes encerram muito rapido.", formatPercent(quickCloseRate)), "Pode existir perda de oportunidade de relacionamento ou venda complementar.", "Adicionar passo obrigatorio de relacionamento antes de fechar atendimento."})
	} else {
		appendDiagnosis(IntelligenceDiagnosis{"quick-close", "healthy", "Tempo de fechamento sob controle", fmt.Sprintf("%s das conversoes sao muito rapidas.", formatPercent(quickCloseRate)), "Fechamento sem sinal forte de pressa excessiva.", "Manter foco em coletar dados do cliente no encerramento."})
	}

	if idleVsServiceRatio >= 1 && totalAttendances >= 8 {
		appendDiagnosis(IntelligenceDiagnosis{"idle-capacity", "attention", "Tempo ocioso acima do tempo atendendo", fmt.Sprintf("Historico ocioso %s vs atendendo %s.", formatDurationMinutes(timeIntelligence.TotalsByStatus.Available), formatDurationMinutes(timeIntelligence.TotalsByStatus.Service)), "Pode haver capacidade parada ou falha de uso da lista em horarios de baixa.", "Criar rotina de ativacao em horarios ociosos (vitrine, WhatsApp, base de leads)."})
	} else {
		appendDiagnosis(IntelligenceDiagnosis{"idle-capacity", "healthy", "Uso de capacidade dentro do esperado", fmt.Sprintf("Historico atendendo %s.", formatDurationMinutes(timeIntelligence.TotalsByStatus.Service)), "Nao ha sinal forte de ociosidade acima do esperado.", "Manter acompanhamento por turno para ajustar escala."})
	}

	severityCounts := SeverityCounts{}
	recommendedActions := make([]string, 0, 4)
	for _, item := range diagnosis {
		switch item.Level {
		case "critical":
			severityCounts.Critical++
		case "attention":
			severityCounts.Attention++
		default:
			severityCounts.Healthy++
		}

		if item.Level != "healthy" && len(recommendedActions) < 4 {
			recommendedActions = append(recommendedActions, item.Action)
		}
	}

	healthScore := 100 - float64(severityCounts.Critical*18) - float64(severityCounts.Attention*8)
	if healthScore < 0 {
		healthScore = 0
	}

	return IntelligenceResponse{
		StoreID:            storeID,
		TenantID:           tenantID,
		TotalAttendances:   totalAttendances,
		ConversionRate:     conversionRate,
		TicketAverage:      ticketAverage,
		HealthScore:        healthScore,
		SeverityCounts:     severityCounts,
		Diagnosis:          diagnosis,
		RecommendedActions: recommendedActions,
		Time:               timeIntelligence,
	}
}

package operations

import (
	"context"
	"strings"
)

// processAutoClose avalia o auto-encerramento (2h) de UMA loja num tick do sweep
// (ProcessTimedAlerts). E' AUTORITATIVO no servidor: fecha atendimentos esquecidos
// mesmo sem ninguem olhando a tela. Idempotente pelo estado no banco (uma vez
// fechado, o servico sai de operation_active_services). A barra de 1 min no front
// e' so display: quem decide e fecha e' este sweep.
//
// Maquina de estados por atendimento aberto (gate shouldMonitorLongOpenAlert):
//   - elapsed < autoClose            => sem countdown (limpa grace pendente).
//   - elapsed >= autoClose, snoozed  => sem countdown enquanto o "Continuar" vale.
//   - elapsed >= autoClose, grace==0 => abre o countdown (grace_deadline=now+grace).
//     Tambem reabre apos o snooze vencer = re-pergunta.
//   - now >= grace_deadline          => AUTO_CLOSE (um por tick, ver nota de sessao).
//   - grace em andamento             => aguarda o proximo tick.
func (service *Service) processAutoClose(ctx context.Context, storeID string, roster []ConsultantProfile, snapshotState SnapshotState, now int64) error {
	if service.alertCoordinator == nil {
		return nil
	}

	rules, err := service.alertCoordinator.LoadOperationalRules(ctx, storeID)
	if err != nil {
		return err
	}
	if !rules.AutoCloseEnabled || rules.AutoCloseMinutes < 1 {
		return nil
	}

	autoCloseMs := int64(rules.AutoCloseMinutes) * 60_000
	graceMs := int64(maxInt(rules.AutoCloseGraceSeconds, 1)) * 1_000

	graceChanged := false
	var closeTarget *ActiveServiceState

	for index := range snapshotState.ActiveServices {
		item := &snapshotState.ActiveServices[index]
		if !shouldMonitorLongOpenAlert(*item) {
			continue
		}

		elapsed := now - item.ServiceStartedAt
		if elapsed < autoCloseMs {
			if item.GraceDeadline != 0 {
				item.GraceDeadline = 0
				graceChanged = true
			}
			continue
		}

		if item.SnoozedUntil > now {
			if item.GraceDeadline != 0 {
				item.GraceDeadline = 0
				graceChanged = true
			}
			continue
		}

		if item.GraceDeadline == 0 {
			item.GraceDeadline = now + graceMs
			graceChanged = true
			continue
		}

		if now >= item.GraceDeadline && closeTarget == nil {
			// Um auto-close por tick para respeitar o append de sessao unica do
			// persistAndAck; os demais fecham nos ticks seguintes.
			target := *item
			closeTarget = &target
		}
	}

	if closeTarget != nil {
		return service.autoCloseService(ctx, storeID, roster, snapshotState, *closeTarget, now)
	}
	if graceChanged {
		return service.persistGraceState(ctx, storeID, snapshotState)
	}
	return nil
}

// autoCloseService encerra UM atendimento autoritativamente (sem AccessContext:
// e' um job de sistema, nao uma rota HTTP). Grava o historico como PENDENTE de
// validacao (close_reason='auto', validation_status='pending', outcome='auto'),
// devolve o consultor a fila (decisao de produto: nao trava o consultor) e resolve
// o alerta long_open_service. Reusa os mesmos blocos do fluxo manual de Finish.
func (service *Service) autoCloseService(ctx context.Context, storeID string, roster []ConsultantProfile, snapshotState SnapshotState, target ActiveServiceState, now int64) error {
	serviceID := strings.TrimSpace(target.ServiceID)
	if serviceID == "" {
		return nil
	}

	rosterByID := mapRosterByID(roster)
	person, ok := rosterByID[target.ConsultantID]
	if !ok {
		return ErrConsultantNotFound
	}

	// duration = fechamento - inicio. effectiveFinishedAt = instante do fechamento
	// (truncado no proximo servico do grupo paralelo, como no Finish). NAO fixado no
	// limite: um atendimento continuado via snooze ate 3h+ conta o tempo real.
	effectiveFinishedAt := deriveSequentialServiceEndAt(target, snapshotState.ActiveServices, snapshotState.ServiceHistory, now)
	queuePositionAtStart := deriveQueuePositionAtStart(target, snapshotState.ActiveServices, snapshotState.ServiceHistory)
	snapshotState.ActiveServices = filterActiveServicesByServiceID(snapshotState.ActiveServices, serviceID)

	isLastService := countActiveServicesForConsultant(snapshotState.ActiveServices, person.ID) == 0

	historyEntry := normalizeHistoryEntry(ServiceHistoryEntry{
		ServiceID:            target.ServiceID,
		StoreID:              storeID,
		PersonID:             person.ID,
		PersonName:           person.Name,
		StartedAt:            target.ServiceStartedAt,
		FinishedAt:           effectiveFinishedAt,
		DurationMs:           maxInt64(0, effectiveFinishedAt-target.ServiceStartedAt),
		FinishOutcome:        outcomeAuto,
		StartMode:            target.StartMode,
		QueuePositionAtStart: queuePositionAtStart,
		QueueWaitMs:          target.QueueWaitMs,
		SkippedPeople:        cloneSkippedPeople(target.SkippedPeople),
		SkippedCount:         len(target.SkippedPeople),
		ParallelGroupID:      target.ParallelGroupID,
		ParallelStartIndex:   target.ParallelStartIndex,
		SiblingServiceIDs:    cloneStringSlice(target.SiblingServiceIDs),
		StartOffsetMs:        target.StartOffsetMs,
		CloseReason:          closeReasonAuto,
		ValidationStatus:     validationStatusPending,
		SnoozeCount:          target.SnoozeCount,
	})

	snapshotState.ServiceHistory = append(snapshotState.ServiceHistory, historyEntry)

	if isLastService {
		snapshotState.WaitingList = append(snapshotState.WaitingList, QueueStateItem{
			ConsultantID:  person.ID,
			QueueJoinedAt: now,
		})
		snapshotState.ConsultantActivitySessions, snapshotState.ConsultantCurrentStatus = applyStatusTransitions(
			snapshotState.ConsultantActivitySessions,
			snapshotState.ConsultantCurrentStatus,
			[]transition{{personID: person.ID, nextStatus: statusQueue}},
			now,
		)
	} else {
		// Sem transicao de status: nao reinserir a ultima sessao (append-only) no persist.
		snapshotState.ConsultantActivitySessions = nil
	}

	_, err := service.persistAndAck(ctx, storeID, actionAutoClose, person.ID, snapshotState, []ServiceHistoryEntry{historyEntry}, nil)
	return err
}

// persistGraceState grava so a mudanca de countdown (grace_deadline) no atendimento
// corrente e emite o evento realtime para o front mostrar/atualizar a barra. Nao ha
// transicao de status, entao zera as sessoes para o persistAndAck nao reinserir a
// ultima sessao (a tabela e' append-only, sem chave unica).
func (service *Service) persistGraceState(ctx context.Context, storeID string, snapshotState SnapshotState) error {
	snapshotState.ConsultantActivitySessions = nil
	_, err := service.persistAndAck(ctx, storeID, actionAutoCloseGrace, "", snapshotState, nil, nil)
	return err
}

// KeepOpen e o "Continuar atendimento" do operador: adia o auto-encerramento por
// mais uma janela (snooze) e apaga o countdown corrente. E' mutacao real e durável
// (grava snoozed_until no banco, sobrevive a restart), nao um dismiss local. O sweep,
// ao vencer o snooze com o atendimento ainda aberto, reabre o countdown (re-pergunta).
func (service *Service) KeepOpen(ctx context.Context, access AccessContext, input KeepOpenCommandInput) (MutationAck, error) {
	resolvedStoreID, err := service.resolveStoreID(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	serviceID := strings.TrimSpace(input.ServiceID)
	if serviceID == "" {
		return MutationAck{}, ErrValidation
	}

	_, snapshotState, err := service.loadSnapshotState(ctx, resolvedStoreID)
	if err != nil {
		return MutationAck{}, err
	}

	index := indexOfActiveServiceByServiceID(snapshotState.ActiveServices, serviceID)
	if index < 0 {
		return MutationAck{}, ErrValidation
	}

	snoozeMinutes := fallbackSnoozeRepromptMinutes
	if service.alertCoordinator != nil {
		if rules, rulesErr := service.alertCoordinator.LoadOperationalRules(ctx, resolvedStoreID); rulesErr == nil && rules.SnoozeRepromptMinutes > 0 {
			snoozeMinutes = rules.SnoozeRepromptMinutes
		}
	}

	now := nowUnixMilli()
	snapshotState.ActiveServices[index].SnoozedUntil = now + int64(snoozeMinutes)*60_000
	snapshotState.ActiveServices[index].GraceDeadline = 0
	snapshotState.ActiveServices[index].SnoozeCount++
	personID := snapshotState.ActiveServices[index].ConsultantID

	// Sem transicao de status: zera as sessoes para nao reinserir a ultima (append-only).
	snapshotState.ConsultantActivitySessions = nil
	return service.persistAndAck(ctx, resolvedStoreID, actionKeepOpen, personID, snapshotState, nil, nil)
}

// ValidateAutoClose promove uma pendencia a validada: o gerente grava o desfecho
// real (via o mesmo modal de fechamento) e os dados preenchidos, preservando a
// metrica de tempo original. UPDATE no historico (nao re-insert).
func (service *Service) ValidateAutoClose(ctx context.Context, access AccessContext, input FinishCommandInput) (MutationAck, error) {
	// allowArchived: valida-se pendencia mesmo se a loja foi arquivada depois.
	resolvedStoreID, err := service.resolveStoreIDAllowArchived(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	serviceID := strings.TrimSpace(input.ServiceID)
	if serviceID == "" {
		return MutationAck{}, ErrValidation
	}
	if _, ok := finishOutcomes[strings.TrimSpace(input.Outcome)]; !ok {
		return MutationAck{}, ErrValidation
	}
	// Justificativa OBRIGATORIA: por que o consultor nao encerrou na hora. E o dado
	// que alimenta as metricas de cobranca (por consultor/gerente/loja).
	validationReason := strings.TrimSpace(input.ValidationReason)
	if validationReason == "" {
		return MutationAck{}, ErrValidation
	}

	entry := normalizeHistoryEntry(ServiceHistoryEntry{
		ServiceID:                  serviceID,
		ValidationReason:           validationReason,
		FinishOutcome:              strings.TrimSpace(input.Outcome),
		IsWindowService:            input.IsWindowService,
		IsGift:                     input.IsGift,
		ProductSeen:                input.ProductSeen,
		ProductClosed:              input.ProductClosed,
		PurchaseCode:               input.PurchaseCode,
		ProductDetails:             input.ProductDetails,
		ProductsSeen:               cloneProducts(input.ProductsSeen),
		ProductsClosed:             cloneProducts(input.ProductsClosed),
		ProductsNotFound:           cloneProducts(input.ProductsNotFound),
		ProductsSeenNone:           input.ProductsSeenNone,
		VisitReasonsNotInformed:    input.VisitReasonsNotInformed,
		CustomerSourcesNotInformed: input.CustomerSourcesNotInformed,
		CustomerName:               input.CustomerName,
		CustomerPhone:              input.CustomerPhone,
		CustomerEmail:              input.CustomerEmail,
		IsExistingCustomer:         input.IsExistingCustomer,
		VisitReasons:               normalizeStringSlice(input.VisitReasons),
		VisitReasonDetails:         normalizeStringMap(input.VisitReasonDetails),
		CustomerSources:            normalizeStringSlice(input.CustomerSources),
		CustomerSourceDetails:      normalizeStringMap(input.CustomerSourceDetails),
		LossReasons:                normalizeStringSlice(input.LossReasons),
		LossReasonDetails:          normalizeStringMap(input.LossReasonDetails),
		LossReasonID:               input.LossReasonID,
		LossReason:                 input.LossReason,
		SaleAmount:                 maxFloat(input.SaleAmount, 0),
		CustomerProfession:         input.CustomerProfession,
		QueueJumpReason:            input.QueueJumpReason,
		Notes:                      input.Notes,
		CampaignMatches:            normalizeCampaignMatches(input.CampaignMatches),
		CampaignBonusTotal:         maxFloat(input.CampaignBonusTotal, 0),
	})

	// Mesma limpeza condicional do Finish: campos so fazem sentido no desfecho certo.
	if entry.FinishOutcome != "nao-compra" {
		entry.LossReasons = nil
		entry.LossReasonDetails = map[string]string{}
		entry.LossReasonID = ""
		entry.LossReason = ""
	}
	if entry.FinishOutcome != "compra" {
		entry.PurchaseCode = ""
	}

	validatedAt := nowUnixMilli()
	if err := service.repository.ValidateAutoClose(ctx, resolvedStoreID, entry, access.UserID, validatedAt); err != nil {
		return MutationAck{}, err
	}

	return service.buildAutoCloseAck(ctx, resolvedStoreID, actionValidate, serviceID), nil
}

// CancelAutoClose descarta a metrica de uma pendencia (fora dos relatorios, mas
// preservada para auditoria) com motivo obrigatorio. UPDATE no historico.
func (service *Service) CancelAutoClose(ctx context.Context, access AccessContext, input CancelMetricCommandInput) (MutationAck, error) {
	resolvedStoreID, err := service.resolveStoreIDAllowArchived(ctx, access, input.StoreID)
	if err != nil {
		return MutationAck{}, err
	}

	serviceID := strings.TrimSpace(input.ServiceID)
	reason := strings.TrimSpace(input.Reason)
	if serviceID == "" || reason == "" {
		return MutationAck{}, ErrValidation
	}

	validatedAt := nowUnixMilli()
	if err := service.repository.CancelAutoClose(ctx, resolvedStoreID, serviceID, reason, access.UserID, validatedAt); err != nil {
		return MutationAck{}, err
	}

	return service.buildAutoCloseAck(ctx, resolvedStoreID, actionCancelMetric, serviceID), nil
}

// buildAutoCloseAck monta o ack de validar/cancelar e publica o evento realtime
// (para o front revalidar a caixa de Pendencias). Nao ha personId associado.
func (service *Service) buildAutoCloseAck(ctx context.Context, storeID string, action string, serviceID string) MutationAck {
	ack := service.buildAck(storeID, action, "")
	ack.ServiceID = serviceID
	service.publisher.PublishOperationEvent(ctx, PublishedEvent{
		StoreID: ack.StoreID,
		Action:  ack.Action,
		SavedAt: ack.SavedAt,
	})
	return ack
}

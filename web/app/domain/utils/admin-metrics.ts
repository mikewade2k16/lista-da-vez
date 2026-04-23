function getMonthStamp(timestamp) {
  const date = new Date(timestamp);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}`;
}

function getDayStamp(timestamp) {
  const date = new Date(timestamp);
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}`;
}

function byCountDescending(a, b) {
  if (b.count !== a.count) {
    return b.count - a.count;
  }

  return a.label.localeCompare(b.label);
}

function groupLabels(entries) {
  const counter = new Map();

  entries.forEach((label) => {
    const normalized = String(label || "").trim();

    if (!normalized) {
      return;
    }

    const key = normalized.toLowerCase();
    const current = counter.get(key) || { label: normalized, count: 0 };

    current.count += 1;
    counter.set(key, current);
  });

  return [...counter.values()].sort(byCountDescending);
}

function extractClosedProductLabels(entry) {
  if (Array.isArray(entry?.productsClosed) && entry.productsClosed.length) {
    return entry.productsClosed
      .map((item) => String(item?.name || item?.code || "").trim())
      .filter(Boolean);
  }

  const fallback = String(entry?.productClosed || entry?.productSeen || entry?.productDetails || "").trim();
  return fallback ? [fallback] : [];
}

function buildLabelMap(options) {
  return new Map((options || []).map((item) => [item.id, item.label]));
}

function resolveLiveStatusSnapshot({
  consultantId,
  now,
  waitingList,
  activeServices,
  pausedEmployees,
  consultantCurrentStatus
}) {
  const service = activeServices.find((item) => item.id === consultantId);

  if (service) {
    return {
      status: "service",
      startedAt: Number(service.serviceStartedAt || now)
    };
  }

  const waitingItem = waitingList.find((item) => item.id === consultantId);

  if (waitingItem) {
    return {
      status: "queue",
      startedAt: Number(waitingItem.queueJoinedAt || now)
    };
  }

  const pausedItem = pausedEmployees.find((item) => item.personId === consultantId);

  if (pausedItem) {
    return {
      status: "paused",
      startedAt: Number(pausedItem.startedAt || now)
    };
  }

  const currentStatus = consultantCurrentStatus?.[consultantId];

  return {
    status: "available",
    startedAt:
      currentStatus?.status === "available"
        ? Number(currentStatus.startedAt || now)
        : now
  };
}

export function buildConsultantStats({ history, consultantId, monthlyGoal = 0, commissionRate = 0, conversionGoal = 0, avgTicketGoal = 0, paGoal = 0 }) {
  const now = Date.now();
  const currentMonth = getMonthStamp(now);
  const monthEntries = history.filter(
    (entry) => entry.personId === consultantId && getMonthStamp(entry.finishedAt) === currentMonth
  );
  const convertedEntries = monthEntries.filter(
    (entry) => entry.finishOutcome === "compra" || entry.finishOutcome === "reserva"
  );
  const soldValue = convertedEntries.reduce((sum, entry) => sum + Number(entry.saleAmount || 0), 0);
  const conversionRate = monthEntries.length ? (convertedEntries.length / monthEntries.length) * 100 : 0;
  const averageDurationMs = monthEntries.length
    ? monthEntries.reduce((sum, entry) => sum + Number(entry.durationMs || 0), 0) / monthEntries.length
    : 0;
  const ticketAverage = convertedEntries.length ? soldValue / convertedEntries.length : 0;
  const totalPieces = monthEntries.reduce((sum, entry) => {
    const closed = Array.isArray(entry.productsClosed) ? entry.productsClosed.length : 0;
    return sum + closed;
  }, 0);
  const paScore = monthEntries.length ? totalPieces / monthEntries.length : 0;

  return {
    monthEntries,
    soldValue,
    conversions: convertedEntries.length,
    nonConversions: monthEntries.length - convertedEntries.length,
    conversionRate,
    ticketAverage,
    paScore,
    nonClientConversions: convertedEntries.filter((entry) => !entry.isExistingCustomer).length,
    queueJumpServices: monthEntries.filter((entry) => entry.startMode === "queue-jump").length,
    averageDurationMs,
    remainingToGoal: Math.max(0, monthlyGoal - soldValue),
    monthlyGoal,
    commissionRate,
    estimatedCommission: soldValue * commissionRate,
    conversionGoal,
    avgTicketGoal,
    paGoal
  };
}

export function buildRankingRows({ history, roster, scope = "month" }) {
  const now = Date.now();
  const currentMonth = getMonthStamp(now);
  const currentDay = getDayStamp(now);
  const scopedHistory = history.filter((entry) => {
    if (scope === "today") {
      return getDayStamp(entry.finishedAt) === currentDay;
    }

    return getMonthStamp(entry.finishedAt) === currentMonth;
  });

  return roster
    .map((consultant) => {
      const entries = scopedHistory.filter((entry) => entry.personId === consultant.id);
      const converted = entries.filter((entry) => entry.finishOutcome === "compra" || entry.finishOutcome === "reserva");
      const soldValue = converted.reduce((sum, entry) => sum + Number(entry.saleAmount || 0), 0);

      const ticketAverage = converted.length ? soldValue / converted.length : 0;
      const totalPieces = entries.reduce((sum, entry) => {
        const closed = Array.isArray(entry.productsClosed) ? entry.productsClosed.length : 0;
        return sum + closed;
      }, 0);
      const paScore = entries.length ? totalPieces / entries.length : 0;

      const completeEntries = entries.filter((entry) => {
        const hasCustomer = Boolean(String(entry.customerName || "").trim()) || Boolean(String(entry.customerPhone || "").trim());
        const hasProduct = (Array.isArray(entry.productsSeen) && entry.productsSeen.length > 0) || Boolean(String(entry.productSeen || "").trim()) || Boolean(entry.productsSeenNone);
        const hasReason = (Array.isArray(entry.visitReasons) && entry.visitReasons.length > 0) || Boolean(entry.visitReasonsNotInformed);
        const hasSource = (Array.isArray(entry.customerSources) && entry.customerSources.length > 0) || Boolean(entry.customerSourcesNotInformed);
        return hasCustomer && hasProduct && hasReason && hasSource;
      });
      const qualityScore = entries.length ? (completeEntries.length / entries.length) * 100 : 0;
      const avgDurationMs = entries.length ? entries.reduce((sum, e) => sum + Number(e.durationMs || 0), 0) / entries.length : 0;
      const queueJumpCount = entries.filter((entry) => entry.startMode === "queue-jump").length;

      return {
        consultantId: consultant.id,
        consultantName: consultant.name,
        soldValue,
        attendances: entries.length,
        conversions: converted.length,
        nonConversions: entries.length - converted.length,
        conversionRate: entries.length ? (converted.length / entries.length) * 100 : 0,
        ticketAverage,
        paScore,
        qualityScore,
        avgDurationMs,
        nonClientConversions: converted.filter((entry) => !entry.isExistingCustomer).length,
        queueJumpServices: queueJumpCount,
        queueJumpRate: entries.length ? (queueJumpCount / entries.length) * 100 : 0
      };
    })
    .sort((a, b) => {
      if (b.soldValue !== a.soldValue) {
        return b.soldValue - a.soldValue;
      }

      if (b.conversions !== a.conversions) {
        return b.conversions - a.conversions;
      }

      return b.conversionRate - a.conversionRate;
    });
}

export function buildConsultantAlerts({ roster, history, settings }) {
  const alerts = [];
  const now = Date.now();
  const currentMonth = getMonthStamp(now);

  const minConversion = Number(settings?.alertMinConversionRate || 0);
  const maxQueueJump = Number(settings?.alertMaxQueueJumpRate || 0);
  const minPa = Number(settings?.alertMinPaScore || 0);
  const minTicket = Number(settings?.alertMinTicketAverage || 0);

  if (!minConversion && !maxQueueJump && !minPa && !minTicket) return alerts;

  (roster || []).forEach((consultant) => {
    const monthEntries = (history || []).filter(
      (entry) => entry.personId === consultant.id && getMonthStamp(entry.finishedAt) === currentMonth
    );

    if (!monthEntries.length) return;

    const converted = monthEntries.filter((e) => e.finishOutcome === "compra" || e.finishOutcome === "reserva");
    const conversionRate = (converted.length / monthEntries.length) * 100;
    const queueJumps = monthEntries.filter((e) => e.startMode === "queue-jump").length;
    const queueJumpRate = (queueJumps / monthEntries.length) * 100;
    const soldValue = converted.reduce((sum, e) => sum + Number(e.saleAmount || 0), 0);
    const ticketAverage = converted.length ? soldValue / converted.length : 0;
    const totalPieces = monthEntries.reduce((sum, e) => sum + (Array.isArray(e.productsClosed) ? e.productsClosed.length : 0), 0);
    const paScore = totalPieces / monthEntries.length;

    if (minConversion > 0 && conversionRate < minConversion) {
      alerts.push({ consultantId: consultant.id, consultantName: consultant.name, type: "conversion", value: conversionRate, threshold: minConversion });
    }
    if (maxQueueJump > 0 && queueJumpRate > maxQueueJump) {
      alerts.push({ consultantId: consultant.id, consultantName: consultant.name, type: "queueJump", value: queueJumpRate, threshold: maxQueueJump });
    }
    if (minPa > 0 && paScore < minPa) {
      alerts.push({ consultantId: consultant.id, consultantName: consultant.name, type: "pa", value: paScore, threshold: minPa });
    }
    if (minTicket > 0 && ticketAverage < minTicket) {
      alerts.push({ consultantId: consultant.id, consultantName: consultant.name, type: "ticket", value: ticketAverage, threshold: minTicket });
    }
  });

  return alerts;
}

export function buildInsights({ history, visitReasonOptions = [], customerSourceOptions = [] }) {
  const visitReasonMap = buildLabelMap(visitReasonOptions);
  const customerSourceMap = buildLabelMap(customerSourceOptions);
  const convertedEntries = history.filter((entry) => entry.finishOutcome === "compra" || entry.finishOutcome === "reserva");
  const soldProducts = groupLabels(convertedEntries.flatMap((entry) => extractClosedProductLabels(entry)));
  const requestedProducts = groupLabels(history.map((entry) => entry.productSeen || entry.productDetails));
  const visitReasons = groupLabels(
    history.flatMap((entry) => (entry.visitReasons || []).map((id) => visitReasonMap.get(id) || id))
  );
  const customerSources = groupLabels(
    history.flatMap((entry) => (entry.customerSources || []).map((id) => customerSourceMap.get(id) || id))
  );
  const professions = groupLabels(history.map((entry) => entry.customerProfession));
  const outcomeSummary = groupLabels(
    history.map((entry) => {
      if (entry.finishOutcome === "compra") {
        return "Compra";
      }

      if (entry.finishOutcome === "reserva") {
        return "Reserva";
      }

      return "Nao compra";
    })
  );
  const hourlySales = new Map();

  convertedEntries.forEach((entry) => {
    const hour = String(new Date(entry.finishedAt).getHours()).padStart(2, "0");
    const current = hourlySales.get(hour) || { label: `${hour}h`, count: 0, value: 0 };

    current.count += 1;
    current.value += Number(entry.saleAmount || 0);
    hourlySales.set(hour, current);
  });

  return {
    soldProducts: soldProducts.slice(0, 8),
    requestedProducts: requestedProducts.slice(0, 8),
    visitReasons: visitReasons.slice(0, 8),
    customerSources: customerSources.slice(0, 8),
    professions: professions.slice(0, 6),
    outcomeSummary,
    hourlySales: [...hourlySales.values()].sort((a, b) => b.value - a.value).slice(0, 8)
  };
}

export function buildTimeIntelligence({
  history,
  roster,
  waitingList,
  activeServices,
  pausedEmployees,
  consultantCurrentStatus,
  consultantActivitySessions,
  settings
}) {
  const now = Date.now();
  const fastThresholdMs = Number(settings.timingFastCloseMinutes || 5) * 60000;
  const longThresholdMs = Number(settings.timingLongServiceMinutes || 25) * 60000;
  const lowSaleThreshold = Number(settings.timingLowSaleAmount || 1200);
  const converted = history.filter((entry) => entry.finishOutcome === "compra" || entry.finishOutcome === "reserva");
  const queueJumpServices = history.filter((entry) => entry.startMode === "queue-jump");
  const quickHighPotential = converted.filter((entry) => Number(entry.durationMs || 0) <= fastThresholdMs);
  const longLowSale = converted.filter(
    (entry) => Number(entry.durationMs || 0) >= longThresholdMs && Number(entry.saleAmount || 0) <= lowSaleThreshold
  );
  const longNoSale = history.filter(
    (entry) => entry.finishOutcome === "nao-compra" && Number(entry.durationMs || 0) >= longThresholdMs
  );
  const quickNoSale = history.filter(
    (entry) => entry.finishOutcome === "nao-compra" && Number(entry.durationMs || 0) <= fastThresholdMs
  );
  const avgQueueWaitMs = history.length
    ? history.reduce((sum, entry) => sum + Number(entry.queueWaitMs || 0), 0) / history.length
    : 0;
  const completedSessions = Array.isArray(consultantActivitySessions) ? consultantActivitySessions : [];
  const openSessions = roster.map((consultant) => {
    const snapshot = resolveLiveStatusSnapshot({
      consultantId: consultant.id,
      now,
      waitingList,
      activeServices,
      pausedEmployees,
      consultantCurrentStatus
    });

    return {
      personId: consultant.id,
      status: snapshot.status,
      startedAt: snapshot.startedAt,
      endedAt: now,
      durationMs: Math.max(0, now - snapshot.startedAt)
    };
  });
  const allSessions = [...completedSessions, ...openSessions];
  const totalsByStatus = {
    available: 0,
    queue: 0,
    service: 0,
    paused: 0
  };

  allSessions.forEach((session) => {
    if (session.status in totalsByStatus) {
      totalsByStatus[session.status] += Number(session.durationMs || 0);
    }
  });

  const consultantsInQueueMs = waitingList.reduce(
    (sum, item) => sum + Math.max(0, now - Number(item.queueJoinedAt || now)),
    0
  );
  const consultantsPausedMs = pausedEmployees.reduce(
    (sum, item) => sum + Math.max(0, now - Number(item.startedAt || now)),
    0
  );
  const consultantsInServiceMs = activeServices.reduce(
    (sum, item) => sum + Math.max(0, now - Number(item.serviceStartedAt || now)),
    0
  );
  const notUsingQueueRate = history.length ? (queueJumpServices.length / history.length) * 100 : 0;

  return {
    quickHighPotentialCount: quickHighPotential.length,
    longLowSaleCount: longLowSale.length,
    longNoSaleCount: longNoSale.length,
    quickNoSaleCount: quickNoSale.length,
    avgQueueWaitMs,
    totalsByStatus,
    consultantsInQueueMs,
    consultantsPausedMs,
    consultantsInServiceMs,
    notUsingQueueRate
  };
}

export function buildOperationalIntelligence({
  history,
  visitReasonOptions = [],
  customerSourceOptions = [],
  roster,
  waitingList,
  activeServices,
  pausedEmployees,
  consultantCurrentStatus,
  consultantActivitySessions,
  settings
}) {
  const totalAttendances = history.length;
  const convertedEntries = history.filter((entry) => entry.finishOutcome === "compra" || entry.finishOutcome === "reserva");
  const noSaleEntries = history.filter((entry) => entry.finishOutcome === "nao-compra");
  const soldValue = convertedEntries.reduce((sum, entry) => sum + Number(entry.saleAmount || 0), 0);
  const conversionRate = totalAttendances ? (convertedEntries.length / totalAttendances) * 100 : 0;
  const ticketAverage = convertedEntries.length ? soldValue / convertedEntries.length : 0;
  const time = buildTimeIntelligence({
    history,
    roster,
    waitingList,
    activeServices,
    pausedEmployees,
    consultantCurrentStatus,
    consultantActivitySessions,
    settings
  });
  const quickNoSaleRate = noSaleEntries.length ? (time.quickNoSaleCount / noSaleEntries.length) * 100 : 0;
  const longNoSaleRate = noSaleEntries.length ? (time.longNoSaleCount / noSaleEntries.length) * 100 : 0;
  const quickCloseRate = convertedEntries.length ? (time.quickHighPotentialCount / convertedEntries.length) * 100 : 0;
  const longLowSaleRate = convertedEntries.length ? (time.longLowSaleCount / convertedEntries.length) * 100 : 0;
  const queueToServiceRatio = time.consultantsInServiceMs > 0 ? time.consultantsInQueueMs / time.consultantsInServiceMs : 0;
  const idleVsServiceRatio = time.totalsByStatus.service > 0 ? time.totalsByStatus.available / time.totalsByStatus.service : 0;
  const diagnosis = [];

  if (totalAttendances < 6) {
    diagnosis.push({
      id: "sample-size",
      level: "attention",
      title: "Base ainda pequena para conclusoes fortes",
      reading: `${totalAttendances} atendimentos registrados ate agora.`,
      hypothesis: "A amostra ainda pode distorcer as leituras de tempo e conversao.",
      action: "Coletar mais atendimentos antes de tomar decisoes estruturais."
    });
  }

  if (time.notUsingQueueRate >= 25) {
    diagnosis.push({
      id: "queue-discipline",
      level: "critical",
      title: "Uso da fila comprometido",
      reading: `${formatPercent(time.notUsingQueueRate)} dos atendimentos foram fora da vez.`,
      hypothesis: "A regra da fila pode estar sendo ignorada com frequencia.",
      action: "Reforcar criterio para furar fila e auditar motivos por consultor diariamente."
    });
  } else if (time.notUsingQueueRate >= 12) {
    diagnosis.push({
      id: "queue-discipline",
      level: "attention",
      title: "Uso da fila acima do ideal",
      reading: `${formatPercent(time.notUsingQueueRate)} dos atendimentos foram fora da vez.`,
      hypothesis: "Pode haver excesso de excecoes no fluxo da loja.",
      action: "Acompanhar quem mais fura fila e validar se os motivos fazem sentido."
    });
  } else {
    diagnosis.push({
      id: "queue-discipline",
      level: "healthy",
      title: "Disciplina de fila estavel",
      reading: `Atendimento fora da vez em ${formatPercent(time.notUsingQueueRate)}.`,
      hypothesis: "As excecoes estao sob controle operacional.",
      action: "Manter monitoramento dos motivos para manter consistencia."
    });
  }

  if (queueToServiceRatio >= 1.2 && time.consultantsInQueueMs >= 20 * 60000) {
    diagnosis.push({
      id: "live-backlog",
      level: "critical",
      title: "Backlog atual de fila elevado",
      reading: `Fila atual acumulada ${formatDurationMinutes(time.consultantsInQueueMs)} vs atendimento atual ${formatDurationMinutes(
        time.consultantsInServiceMs
      )}.`,
      hypothesis: "Equipe pode estar sem tracao de inicio de atendimento no momento.",
      action: "Acionar lider para redistribuir entrada em atendimento nos proximos minutos."
    });
  } else if (queueToServiceRatio >= 0.7 && time.consultantsInQueueMs >= 10 * 60000) {
    diagnosis.push({
      id: "live-backlog",
      level: "attention",
      title: "Fila atual crescendo",
      reading: `Fila atual acumulada ${formatDurationMinutes(time.consultantsInQueueMs)}.`,
      hypothesis: "A demanda atual pode estar maior que a capacidade ativa.",
      action: "Priorizar chamadas da fila e reduzir pausas nao essenciais."
    });
  } else {
    diagnosis.push({
      id: "live-backlog",
      level: "healthy",
      title: "Ritmo atual equilibrado",
      reading: `Fila atual em ${formatDurationMinutes(time.consultantsInQueueMs)}.`,
      hypothesis: "Fluxo atual entre espera e atendimento esta proporcional.",
      action: "Manter ritmo de chamada e monitorar picos por horario."
    });
  }

  if (quickNoSaleRate >= 45) {
    diagnosis.push({
      id: "quick-no-sale",
      level: "critical",
      title: "Nao compra muito rapida",
      reading: `${formatPercent(quickNoSaleRate)} dos nao fechamentos encerram muito rapido.`,
      hypothesis: "Abordagem inicial pode estar curta, sem exploracao de oportunidade.",
      action: "Testar script de descoberta de motivo e sugestao de 2a opcao antes de encerrar."
    });
  } else if (quickNoSaleRate >= 25) {
    diagnosis.push({
      id: "quick-no-sale",
      level: "attention",
      title: "Nao compra rapida em alta",
      reading: `${formatPercent(quickNoSaleRate)} dos nao fechamentos foram rapidos.`,
      hypothesis: "Pode haver descarte precoce de atendimento.",
      action: "Acompanhar atendimentos curtos e criar checklist minimo antes de encerrar."
    });
  } else {
    diagnosis.push({
      id: "quick-no-sale",
      level: "healthy",
      title: "Tempo minimo de exploracao razoavel",
      reading: `${formatPercent(quickNoSaleRate)} dos nao fechamentos foram rapidos.`,
      hypothesis: "A equipe tende a investigar melhor antes de encerrar sem venda.",
      action: "Manter rotina de registro do motivo de nao compra."
    });
  }

  if (longLowSaleRate >= 30 || longNoSaleRate >= 35) {
    diagnosis.push({
      id: "long-service-low-return",
      level: "critical",
      title: "Atendimento longo com retorno baixo",
      reading: `${formatPercent(longLowSaleRate)} de vendas longas com ticket baixo e ${formatPercent(
        longNoSaleRate
      )} de nao compra longa.`,
      hypothesis: "Tempo alto sem progresso pode indicar baixa objetividade na conducao.",
      action: "Criar checkpoints de 5 em 5 minutos para avancar proposta, upsell ou encerramento."
    });
  } else if (longLowSaleRate >= 18 || longNoSaleRate >= 20) {
    diagnosis.push({
      id: "long-service-low-return",
      level: "attention",
      title: "Parte dos atendimentos longos sem retorno",
      reading: `${formatPercent(longLowSaleRate)} de vendas longas com ticket baixo.`,
      hypothesis: "Existe espaco para melhorar conducao de atendimento demorado.",
      action: "Revisar casos de maior duracao e mapear pontos de trava."
    });
  } else {
    diagnosis.push({
      id: "long-service-low-return",
      level: "healthy",
      title: "Duracao e retorno em equilibrio",
      reading: "Nao houve excesso relevante de atendimento longo com baixo retorno.",
      hypothesis: "O tempo investido esta proporcional ao resultado comercial.",
      action: "Continuar monitorando para manter estabilidade."
    });
  }

  if (quickCloseRate >= 45) {
    diagnosis.push({
      id: "quick-close",
      level: "attention",
      title: "Fechamento rapido em excesso",
      reading: `${formatPercent(quickCloseRate)} das conversoes encerram muito rapido.`,
      hypothesis: "Pode existir perda de oportunidade de relacionamento ou venda complementar.",
      action: "Adicionar passo obrigatorio de relacionamento antes de fechar atendimento."
    });
  } else {
    diagnosis.push({
      id: "quick-close",
      level: "healthy",
      title: "Tempo de fechamento sob controle",
      reading: `${formatPercent(quickCloseRate)} das conversoes sao muito rapidas.`,
      hypothesis: "Fechamento sem sinal forte de pressa excessiva.",
      action: "Manter foco em coletar dados do cliente no encerramento."
    });
  }

  if (idleVsServiceRatio >= 1 && totalAttendances >= 8) {
    diagnosis.push({
      id: "idle-capacity",
      level: "attention",
      title: "Tempo ocioso acima do tempo atendendo",
      reading: `Historico ocioso ${formatDurationMinutes(time.totalsByStatus.available)} vs atendendo ${formatDurationMinutes(
        time.totalsByStatus.service
      )}.`,
      hypothesis: "Pode haver capacidade parada ou falha de uso da lista em horarios de baixa.",
      action: "Criar rotina de ativacao em horarios ociosos (vitrine, WhatsApp, base de leads)."
    });
  } else {
    diagnosis.push({
      id: "idle-capacity",
      level: "healthy",
      title: "Uso de capacidade dentro do esperado",
      reading: `Historico atendendo ${formatDurationMinutes(time.totalsByStatus.service)}.`,
      hypothesis: "Nao ha sinal forte de ociosidade acima do esperado.",
      action: "Manter acompanhamento por turno para ajustar escala."
    });
  }

  const severityCounts = diagnosis.reduce(
    (acc, item) => {
      if (item.level === "critical") {
        acc.critical += 1;
      } else if (item.level === "attention") {
        acc.attention += 1;
      } else {
        acc.healthy += 1;
      }

      return acc;
    },
    { critical: 0, attention: 0, healthy: 0 }
  );
  const healthScore = Math.max(0, 100 - severityCounts.critical * 18 - severityCounts.attention * 8);
  const recommendedActions = diagnosis
    .filter((item) => item.level !== "healthy")
    .slice(0, 4)
    .map((item) => item.action);

  return {
    totalAttendances,
    conversionRate,
    ticketAverage,
    diagnosis,
    time,
    severityCounts,
    healthScore,
    recommendedActions
  };
}

export function formatCurrencyBRL(value) {
  return new Intl.NumberFormat("pt-BR", { style: "currency", currency: "BRL" }).format(Number(value || 0));
}

export function formatPercent(value) {
  return `${Number(value || 0).toFixed(1)}%`;
}

export function formatDurationMinutes(valueMs) {
  const minutes = Math.round(Number(valueMs || 0) / 60000);
  return `${minutes} min`;
}

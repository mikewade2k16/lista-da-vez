const OUTCOME_LABELS = {
  compra: "Compra",
  reserva: "Reserva",
  "nao-compra": "Nao compra"
};

const START_MODE_LABELS = {
  queue: "Na vez",
  "queue-jump": "Fora da vez"
};

const COMPLETION_LEVEL_LABELS = {
  excellent: "Completo + observacao",
  complete: "Completo",
  incomplete: "Incompleto"
};

export const DEFAULT_REPORT_FILTERS = {
  dateFrom: "",
  dateTo: "",
  consultantIds: [],
  outcomes: [],
  sourceIds: [],
  visitReasonIds: [],
  startModes: [],
  existingCustomerModes: [],
  completionLevels: [],
  campaignIds: [],
  minSaleAmount: "",
  maxSaleAmount: "",
  search: ""
};

function toDayStartMs(dateValue) {
  if (!dateValue) {
    return null;
  }

  const date = new Date(`${dateValue}T00:00:00`);
  const timestamp = date.getTime();

  return Number.isFinite(timestamp) ? timestamp : null;
}

function toDayEndMs(dateValue) {
  if (!dateValue) {
    return null;
  }

  const date = new Date(`${dateValue}T23:59:59.999`);
  const timestamp = date.getTime();

  return Number.isFinite(timestamp) ? timestamp : null;
}

function toComparableText(value) {
  return String(value || "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();
}

function buildLabelMap(options) {
  return new Map((options || []).map((item) => [item.id, item.label]));
}

function normalizeFilterList(primaryValue, legacyValue) {
  const source =
    primaryValue !== undefined
      ? primaryValue
      : legacyValue !== undefined
        ? legacyValue
        : [];

  if (!source || source === "all") {
    return [];
  }

  const normalized = Array.isArray(source) ? source : [source];

  return [...new Set(normalized.map((item) => String(item || "").trim()).filter((item) => item && item !== "all"))];
}

function hasText(value) {
  return String(value || "").trim().length > 0;
}

function intersectsAny(values, requiredValues) {
  if (!Array.isArray(requiredValues) || requiredValues.length === 0) {
    return true;
  }

  return Array.isArray(values) && values.some((value) => requiredValues.includes(value));
}

function matchesSelectedValue(value, selectedValues) {
  if (!Array.isArray(selectedValues) || selectedValues.length === 0) {
    return true;
  }

  return selectedValues.includes(value);
}

function evaluateEntryCompletion(entry) {
  const checks = {
    customerName: hasText(entry.customerName),
    customerPhone: hasText(entry.customerPhone),
    product:
      hasText(entry.productClosed) ||
      hasText(entry.productSeen) ||
      hasText(entry.productDetails) ||
      (Array.isArray(entry.productsSeen) && entry.productsSeen.length > 0) ||
      Boolean(entry.productsSeenNone),
    visitReasons:
      (Array.isArray(entry.visitReasons) && entry.visitReasons.length > 0) ||
      Boolean(entry.visitReasonsNotInformed),
    customerSources:
      (Array.isArray(entry.customerSources) && entry.customerSources.length > 0) ||
      Boolean(entry.customerSourcesNotInformed)
  };
  const coreTotal = Object.keys(checks).length;
  const coreFilledCount = Object.values(checks).filter(Boolean).length;
  const hasNotes = hasText(entry.notes);
  const isCoreComplete = coreFilledCount === coreTotal;
  const level = isCoreComplete ? (hasNotes ? "excellent" : "complete") : "incomplete";

  return {
    coreFilledCount,
    coreTotal,
    coreFillRate: coreTotal ? coreFilledCount / coreTotal : 0,
    hasNotes,
    isCoreComplete,
    level,
    levelLabel: COMPLETION_LEVEL_LABELS[level]
  };
}

function matchesReportFilters(entry, filters, consultantName) {
  const finishedAt = Number(entry.finishedAt || 0);
  const startAt = toDayStartMs(filters.dateFrom);
  const endAt = toDayEndMs(filters.dateTo);
  const minSaleAmount = Number(filters.minSaleAmount || 0);
  const maxSaleAmount = Number(filters.maxSaleAmount || 0);
  const saleAmount = Number(entry.saleAmount || 0);
  const completion = evaluateEntryCompletion(entry);

  if (startAt !== null && finishedAt < startAt) {
    return false;
  }

  if (endAt !== null && finishedAt > endAt) {
    return false;
  }

  if (!matchesSelectedValue(entry.personId, filters.consultantIds)) {
    return false;
  }

  if (!matchesSelectedValue(entry.finishOutcome || "nao-compra", filters.outcomes)) {
    return false;
  }

  if (!matchesSelectedValue(entry.startMode || "queue", filters.startModes)) {
    return false;
  }

  if (!matchesSelectedValue(entry.isExistingCustomer ? "yes" : "no", filters.existingCustomerModes)) {
    return false;
  }

  if (!matchesSelectedValue(completion.level, filters.completionLevels)) {
    return false;
  }

  if (Number.isFinite(minSaleAmount) && minSaleAmount > 0 && saleAmount < minSaleAmount) {
    return false;
  }

  if (Number.isFinite(maxSaleAmount) && maxSaleAmount > 0 && saleAmount > maxSaleAmount) {
    return false;
  }

  if (!intersectsAny(entry.customerSources, filters.sourceIds)) {
    return false;
  }

  if (!intersectsAny(entry.visitReasons, filters.visitReasonIds)) {
    return false;
  }

  if (Array.isArray(filters.campaignIds) && filters.campaignIds.length > 0) {
    const entryIds = Array.isArray(entry.campaignMatches)
      ? entry.campaignMatches.map((m) => m.campaignId)
      : [];

    if (!filters.campaignIds.some((id) => entryIds.includes(id))) {
      return false;
    }
  }

  const query = toComparableText(filters.search);

  if (!query) {
    return true;
  }

  const searchable = [
    entry.storeName,
    entry.serviceId,
    entry.personName,
    consultantName,
    entry.customerName,
    entry.customerPhone,
    entry.customerEmail,
    entry.customerProfession,
    entry.productSeen,
    entry.productClosed,
    entry.productDetails,
    entry.notes
  ];

  return searchable.some((value) => toComparableText(value).includes(query));
}

function toLocaleDateTime(timestamp) {
  if (!timestamp) {
    return "-";
  }

  return new Intl.DateTimeFormat("pt-BR", {
    dateStyle: "short",
    timeStyle: "short"
  }).format(new Date(timestamp));
}

function formatDurationMinutes(durationMs) {
  const minutes = Math.round(Number(durationMs || 0) / 60000);
  return `${minutes} min`;
}

function formatCurrency(value) {
  return new Intl.NumberFormat("pt-BR", {
    style: "currency",
    currency: "BRL"
  }).format(Number(value || 0));
}

function buildReportRows(rows = [], visitReasonOptions = [], customerSourceOptions = []) {
  const visitReasonMap = buildLabelMap(visitReasonOptions);
  const customerSourceMap = buildLabelMap(customerSourceOptions);

  return (Array.isArray(rows) ? rows : []).map((row) => {
    const visitReasons = (row.visitReasons || []).map((item) => visitReasonMap.get(item) || item).filter(Boolean);
    const customerSources = (row.customerSources || []).map((item) => customerSourceMap.get(item) || item).filter(Boolean);
    const completionLevel = row.completionLevel || "incomplete";

    return {
      serviceId: row.serviceId || "",
      storeId: row.storeId || "",
      storeName: row.storeName || "-",
      consultantId: row.consultantId || "",
      finishedAt: Number(row.finishedAt || 0),
      finishedAtLabel: toLocaleDateTime(row.finishedAt),
      consultantName: row.consultantName || "-",
      outcome: row.outcome || "nao-compra",
      outcomeLabel: OUTCOME_LABELS[row.outcome] || "Nao compra",
      saleAmount: Number(row.saleAmount || 0),
      saleAmountLabel: formatCurrency(row.saleAmount || 0),
      durationMs: Number(row.durationMs || 0),
      durationLabel: formatDurationMinutes(row.durationMs || 0),
      queueWaitMs: Number(row.queueWaitMs || 0),
      queueWaitLabel: formatDurationMinutes(row.queueWaitMs || 0),
      startMode: row.startMode || "queue",
      startModeLabel: START_MODE_LABELS[row.startMode] || "Na vez",
      isWindowService: Boolean(row.isWindowService),
      isGift: Boolean(row.isGift),
      isExistingCustomer: Boolean(row.isExistingCustomer),
      customerName: row.customerName || "-",
      customerPhone: row.customerPhone || "-",
      customerEmail: row.customerEmail || "-",
      customerProfession: row.customerProfession || "-",
      productSeen: row.productSeen || "-",
      productClosed: row.productClosed || "-",
      visitReasonsLabel: visitReasons.length ? visitReasons.join(", ") : "-",
      customerSourcesLabel: customerSources.length ? customerSources.join(", ") : "-",
      queueJumpReason: row.queueJumpReason || "-",
      notes: row.notes || "",
      hasNotes: Boolean(row.hasNotes),
      completionLevel,
      completionLabel: COMPLETION_LEVEL_LABELS[completionLevel] || COMPLETION_LEVEL_LABELS.incomplete,
      completionRate: Number(row.completionRate || 0),
      campaignNamesLabel: Array.isArray(row.campaignNames) && row.campaignNames.length
        ? row.campaignNames.join(", ")
        : "-",
      campaignBonusTotal: Number(row.campaignBonusTotal || 0),
      campaignBonusTotalLabel: formatCurrency(row.campaignBonusTotal || 0)
    };
  });
}

export function buildReportRowsFromApi(rows = [], visitReasonOptions = [], customerSourceOptions = []) {
  return buildReportRows(rows, visitReasonOptions, customerSourceOptions);
}

function extractClosedProductLabels(entry) {
  if (Array.isArray(entry?.productsClosed) && entry.productsClosed.length) {
    return entry.productsClosed
      .map((item) => String(item?.name || item?.code || "").trim())
      .filter(Boolean);
  }

  const fallback = String(entry?.productClosed || "").trim();
  return fallback ? [fallback] : [];
}

function resolveConsultantQualityLevel(completeRate, excellentRate) {
  if (completeRate >= 85 && excellentRate >= 35) {
    return {
      key: "highlight",
      label: "Destaque"
    };
  }

  if (completeRate >= 70) {
    return {
      key: "consistent",
      label: "Consistente"
    };
  }

  return {
    key: "attention",
    label: "Precisa melhorar"
  };
}

export function normalizeReportFilters(filters = {}) {
  return {
    dateFrom: String(filters.dateFrom || ""),
    dateTo: String(filters.dateTo || ""),
    consultantIds: normalizeFilterList(filters.consultantIds, filters.consultantId),
    outcomes: normalizeFilterList(filters.outcomes, filters.outcome),
    sourceIds: normalizeFilterList(filters.sourceIds, filters.sourceId),
    visitReasonIds: normalizeFilterList(filters.visitReasonIds, filters.visitReasonId),
    startModes: normalizeFilterList(filters.startModes, filters.startMode),
    existingCustomerModes: normalizeFilterList(filters.existingCustomerModes, filters.existingCustomer),
    completionLevels: normalizeFilterList(filters.completionLevels, filters.completionLevel),
    campaignIds: normalizeFilterList(filters.campaignIds, []),
    minSaleAmount: String(filters.minSaleAmount ?? ""),
    maxSaleAmount: String(filters.maxSaleAmount ?? ""),
    search: String(filters.search || "")
  };
}

export function buildReportData({
  history = [],
  roster = [],
  visitReasonOptions = [],
  customerSourceOptions = [],
  filters = DEFAULT_REPORT_FILTERS
}) {
  const normalizedFilters = normalizeReportFilters(filters);
  const consultantMap = new Map((roster || []).map((consultant) => [consultant.id, consultant.name]));
  const visitReasonMap = buildLabelMap(visitReasonOptions);
  const customerSourceMap = buildLabelMap(customerSourceOptions);
  const filteredHistory = history
    .filter((entry) => matchesReportFilters(entry, normalizedFilters, consultantMap.get(entry.personId) || entry.personName))
    .sort((a, b) => Number(b.finishedAt || 0) - Number(a.finishedAt || 0));
  const rows = filteredHistory.map((entry) => {
    const visitReasons = (entry.visitReasons || []).map((item) => visitReasonMap.get(item) || item).filter(Boolean);
    const customerSources = (entry.customerSources || []).map((item) => customerSourceMap.get(item) || item).filter(Boolean);
    const campaignMatches = Array.isArray(entry.campaignMatches) ? entry.campaignMatches : [];
    const completion = evaluateEntryCompletion(entry);

    return {
      serviceId: entry.serviceId || "",
      storeId: entry.storeId || "",
      storeName: entry.storeName || "-",
      consultantId: entry.personId || "",
      finishedAt: Number(entry.finishedAt || 0),
      finishedAtLabel: toLocaleDateTime(entry.finishedAt),
      consultantName: consultantMap.get(entry.personId) || entry.personName || "-",
      outcome: entry.finishOutcome || "nao-compra",
      outcomeLabel: OUTCOME_LABELS[entry.finishOutcome] || "Nao compra",
      saleAmount: Number(entry.saleAmount || 0),
      saleAmountLabel: formatCurrency(entry.saleAmount || 0),
      durationMs: Number(entry.durationMs || 0),
      durationLabel: formatDurationMinutes(entry.durationMs || 0),
      queueWaitMs: Number(entry.queueWaitMs || 0),
      queueWaitLabel: formatDurationMinutes(entry.queueWaitMs || 0),
      startMode: entry.startMode || "queue",
      startModeLabel: START_MODE_LABELS[entry.startMode] || "Na vez",
      isWindowService: Boolean(entry.isWindowService),
      isGift: Boolean(entry.isGift),
      isExistingCustomer: Boolean(entry.isExistingCustomer),
      customerName: entry.customerName || "-",
      customerPhone: entry.customerPhone || "-",
      customerEmail: entry.customerEmail || "-",
      customerProfession: entry.customerProfession || "-",
      productSeen: entry.productSeen || "-",
      productClosed: entry.productClosed || "-",
      visitReasonsLabel: visitReasons.length ? visitReasons.join(", ") : "-",
      customerSourcesLabel: customerSources.length ? customerSources.join(", ") : "-",
      queueJumpReason: entry.queueJumpReason || "-",
      notes: entry.notes || "",
      hasNotes: completion.hasNotes,
      completionLevel: completion.level,
      completionLabel: completion.levelLabel,
      completionRate: completion.coreFillRate,
      campaignNamesLabel: campaignMatches.length
        ? campaignMatches.map((match) => match.campaignName || match.campaignId).join(", ")
        : "-",
      campaignBonusTotal: Number(entry.campaignBonusTotal || 0),
      campaignBonusTotalLabel: formatCurrency(entry.campaignBonusTotal || 0)
    };
  });

  const conversions = rows.filter((row) => row.outcome === "compra" || row.outcome === "reserva");
  const soldValue = conversions.reduce((sum, row) => sum + row.saleAmount, 0);
  const totalDurationMs = rows.reduce((sum, row) => sum + row.durationMs, 0);
  const totalQueueWaitMs = rows.reduce((sum, row) => sum + row.queueWaitMs, 0);
  const queueJumpCount = rows.filter((row) => row.startMode === "queue-jump").length;
  const campaignBonusTotal = rows.reduce((sum, row) => sum + row.campaignBonusTotal, 0);
  const excellentCount = rows.filter((row) => row.completionLevel === "excellent").length;
  const completeCount = rows.filter((row) => row.completionLevel === "complete" || row.completionLevel === "excellent").length;
  const incompleteCount = rows.filter((row) => row.completionLevel === "incomplete").length;
  const notesCount = rows.filter((row) => row.hasNotes).length;

  // Chart data computed from filtered history
  const outcomeCounts = { compra: 0, reserva: 0, "nao-compra": 0 };
  const hourlyMap = new Map();
  const consultantChartMap = new Map();
  const visitReasonChartMap = new Map();
  const customerSourceChartMap = new Map();
  const productClosedMap = new Map();

  filteredHistory.forEach((entry) => {
    const outcome = entry.finishOutcome || "nao-compra";
    const isConversion = outcome === "compra" || outcome === "reserva";
    const amount = Number(entry.saleAmount || 0);

    outcomeCounts[outcome] = (outcomeCounts[outcome] || 0) + 1;

    const hourKey = String(new Date(entry.finishedAt || 0).getHours()).padStart(2, "0");
    const hb = hourlyMap.get(hourKey) || { hour: hourKey, label: `${hourKey}h`, attendances: 0, conversions: 0, saleAmount: 0 };
    hb.attendances += 1;
    if (isConversion) { hb.conversions += 1; hb.saleAmount += amount; }
    hourlyMap.set(hourKey, hb);

    const cId = entry.personId || "";
    const cb = consultantChartMap.get(cId) || { consultantId: cId, consultantName: consultantMap.get(cId) || entry.personName || "-", attendances: 0, conversions: 0, saleAmount: 0 };
    cb.attendances += 1;
    if (isConversion) { cb.conversions += 1; cb.saleAmount += amount; }
    consultantChartMap.set(cId, cb);

    (entry.visitReasons || []).forEach((id) => {
      const label = visitReasonMap.get(id) || id;
      const vb = visitReasonChartMap.get(id) || { label, count: 0 };
      vb.count += 1;
      visitReasonChartMap.set(id, vb);
    });

    (entry.customerSources || []).forEach((id) => {
      const label = customerSourceMap.get(id) || id;
      const sb = customerSourceChartMap.get(id) || { label, count: 0 };
      sb.count += 1;
      customerSourceChartMap.set(id, sb);
    });

    extractClosedProductLabels(entry).forEach((product) => {
      const pb = productClosedMap.get(product) || { label: product, count: 0 };
      pb.count += 1;
      productClosedMap.set(product, pb);
    });
  });

  const byCountDesc = (a, b) => b.count - a.count;
  const chartData = {
    outcomeCounts,
    hourlyData: [...hourlyMap.values()].sort((a, b) => a.hour.localeCompare(b.hour)),
    consultantAgg: [...consultantChartMap.values()].sort((a, b) => b.saleAmount - a.saleAmount),
    topVisitReasons: [...visitReasonChartMap.values()].sort(byCountDesc).slice(0, 8),
    topCustomerSources: [...customerSourceChartMap.values()].sort(byCountDesc).slice(0, 8),
    topProductsClosed: [...productClosedMap.values()].sort(byCountDesc).slice(0, 8)
  };

  const consultantQualityMap = new Map();

  rows.forEach((row) => {
    const bucket = consultantQualityMap.get(row.consultantId) || {
      consultantId: row.consultantId,
      consultantName: row.consultantName,
      totalAttendances: 0,
      completeCount: 0,
      excellentCount: 0,
      incompleteCount: 0,
      notesCount: 0
    };

    bucket.totalAttendances += 1;

    if (row.completionLevel === "excellent") {
      bucket.completeCount += 1;
      bucket.excellentCount += 1;
    } else if (row.completionLevel === "complete") {
      bucket.completeCount += 1;
    } else {
      bucket.incompleteCount += 1;
    }

    if (row.hasNotes) {
      bucket.notesCount += 1;
    }

    consultantQualityMap.set(row.consultantId, bucket);
  });

  const consultantQuality = [...consultantQualityMap.values()]
    .map((item) => {
      const completeRate = item.totalAttendances ? (item.completeCount / item.totalAttendances) * 100 : 0;
      const excellentRate = item.totalAttendances ? (item.excellentCount / item.totalAttendances) * 100 : 0;
      const notesRate = item.totalAttendances ? (item.notesCount / item.totalAttendances) * 100 : 0;
      const incompleteRate = item.totalAttendances ? (item.incompleteCount / item.totalAttendances) * 100 : 0;
      const qualityLevel = resolveConsultantQualityLevel(completeRate, excellentRate);

      return {
        ...item,
        completeRate,
        excellentRate,
        notesRate,
        incompleteRate,
        qualityLevelKey: qualityLevel.key,
        qualityLevelLabel: qualityLevel.label
      };
    })
    .sort((a, b) => {
      if (b.excellentRate !== a.excellentRate) {
        return b.excellentRate - a.excellentRate;
      }

      if (b.completeRate !== a.completeRate) {
        return b.completeRate - a.completeRate;
      }

      return b.totalAttendances - a.totalAttendances;
    });

  return {
    filters: normalizedFilters,
    rows,
    chartData,
    quality: {
      completeCount,
      excellentCount,
      incompleteCount,
      notesCount,
      completeRate: rows.length ? (completeCount / rows.length) * 100 : 0,
      excellentRate: rows.length ? (excellentCount / rows.length) * 100 : 0,
      incompleteRate: rows.length ? (incompleteCount / rows.length) * 100 : 0,
      notesRate: rows.length ? (notesCount / rows.length) * 100 : 0,
      byConsultant: consultantQuality
    },
    metrics: {
      totalAttendances: rows.length,
      conversions: conversions.length,
      conversionRate: rows.length ? (conversions.length / rows.length) * 100 : 0,
      soldValue,
      soldValueLabel: formatCurrency(soldValue),
      averageTicket: conversions.length ? soldValue / conversions.length : 0,
      averageTicketLabel: formatCurrency(conversions.length ? soldValue / conversions.length : 0),
      averageDurationMs: rows.length ? totalDurationMs / rows.length : 0,
      averageDurationLabel: formatDurationMinutes(rows.length ? totalDurationMs / rows.length : 0),
      averageQueueWaitMs: rows.length ? totalQueueWaitMs / rows.length : 0,
      averageQueueWaitLabel: formatDurationMinutes(rows.length ? totalQueueWaitMs / rows.length : 0),
      queueJumpRate: rows.length ? (queueJumpCount / rows.length) * 100 : 0,
      campaignBonusTotal,
      campaignBonusTotalLabel: formatCurrency(campaignBonusTotal)
    }
  };
}

export function buildReportDataFromApi({
  overview = null,
  results = null,
  visitReasonOptions = [],
  customerSourceOptions = []
}) {
  const normalizedFilters = normalizeReportFilters(results?.filters || overview?.filters || DEFAULT_REPORT_FILTERS);
  const rows = buildReportRows(results?.rows || [], visitReasonOptions, customerSourceOptions);
  const metrics = overview?.metrics || {};
  const quality = overview?.quality || {};

  return {
    filters: normalizedFilters,
    rows,
    totalRows: Number(results?.total || rows.length),
    chartData: {
      outcomeCounts: {
        compra: Number(overview?.chartData?.outcomeCounts?.compra || 0),
        reserva: Number(overview?.chartData?.outcomeCounts?.reserva || 0),
        "nao-compra": Number(overview?.chartData?.outcomeCounts?.["nao-compra"] || 0)
      },
      hourlyData: Array.isArray(overview?.chartData?.hourlyData) ? overview.chartData.hourlyData : [],
      consultantAgg: Array.isArray(overview?.chartData?.consultantAgg) ? overview.chartData.consultantAgg : [],
      topVisitReasons: Array.isArray(overview?.chartData?.topVisitReasons) ? overview.chartData.topVisitReasons : [],
      topCustomerSources: Array.isArray(overview?.chartData?.topCustomerSources) ? overview.chartData.topCustomerSources : [],
      topProductsClosed: Array.isArray(overview?.chartData?.topProductsClosed) ? overview.chartData.topProductsClosed : []
    },
    quality: {
      completeCount: Number(quality.completeCount || 0),
      excellentCount: Number(quality.excellentCount || 0),
      incompleteCount: Number(quality.incompleteCount || 0),
      notesCount: Number(quality.notesCount || 0),
      completeRate: Number(quality.completeRate || 0),
      excellentRate: Number(quality.excellentRate || 0),
      incompleteRate: Number(quality.incompleteRate || 0),
      notesRate: Number(quality.notesRate || 0),
      byConsultant: Array.isArray(quality.byConsultant) ? quality.byConsultant : []
    },
    metrics: {
      totalAttendances: Number(metrics.totalAttendances || 0),
      conversions: Number(metrics.conversions || 0),
      conversionRate: Number(metrics.conversionRate || 0),
      soldValue: Number(metrics.soldValue || 0),
      soldValueLabel: formatCurrency(metrics.soldValue || 0),
      averageTicket: Number(metrics.averageTicket || 0),
      averageTicketLabel: formatCurrency(metrics.averageTicket || 0),
      averageDurationMs: Number(metrics.averageDurationMs || 0),
      averageDurationLabel: formatDurationMinutes(metrics.averageDurationMs || 0),
      averageQueueWaitMs: Number(metrics.averageQueueWaitMs || 0),
      averageQueueWaitLabel: formatDurationMinutes(metrics.averageQueueWaitMs || 0),
      queueJumpRate: Number(metrics.queueJumpRate || 0),
      campaignBonusTotal: Number(metrics.campaignBonusTotal || 0),
      campaignBonusTotalLabel: formatCurrency(metrics.campaignBonusTotal || 0)
    }
  };
}

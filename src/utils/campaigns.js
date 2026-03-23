const DEFAULT_CAMPAIGN = {
  id: "",
  name: "",
  description: "",
  isActive: true,
  startsAt: "",
  endsAt: "",
  targetOutcome: "compra-reserva",
  minSaleAmount: 0,
  maxServiceMinutes: 0,
  sourceIds: [],
  reasonIds: [],
  queueJumpOnly: false,
  existingCustomerFilter: "all",
  bonusFixed: 0,
  bonusRate: 0
};

const VALID_OUTCOMES = new Set(["qualquer", "compra", "reserva", "nao-compra", "compra-reserva"]);
const VALID_EXISTING_FILTERS = new Set(["all", "yes", "no"]);

function uniqueList(values) {
  return [...new Set((values || []).map((value) => String(value).trim()).filter(Boolean))];
}

function toNonNegativeNumber(value) {
  return Math.max(0, Number(value) || 0);
}

function toStartOfDayMs(dateValue) {
  if (!dateValue) {
    return null;
  }

  const date = new Date(`${dateValue}T00:00:00`);
  const timestamp = date.getTime();
  return Number.isFinite(timestamp) ? timestamp : null;
}

function toEndOfDayMs(dateValue) {
  if (!dateValue) {
    return null;
  }

  const date = new Date(`${dateValue}T23:59:59.999`);
  const timestamp = date.getTime();
  return Number.isFinite(timestamp) ? timestamp : null;
}

function matchesOutcome(targetOutcome, finishOutcome) {
  if (targetOutcome === "qualquer") {
    return true;
  }

  if (targetOutcome === "compra-reserva") {
    return finishOutcome === "compra" || finishOutcome === "reserva";
  }

  return targetOutcome === finishOutcome;
}

function matchesExistingCustomer(filter, isExistingCustomer) {
  if (filter === "all") {
    return true;
  }

  if (filter === "yes") {
    return Boolean(isExistingCustomer);
  }

  if (filter === "no") {
    return !Boolean(isExistingCustomer);
  }

  return true;
}

function intersects(selectedValues, entryValues) {
  if (!selectedValues.length) {
    return true;
  }

  return selectedValues.some((value) => entryValues.includes(value));
}

function calculateCampaignBonus(campaign, historyEntry) {
  const saleAmount = Number(historyEntry.saleAmount || 0);
  const totalBonus = toNonNegativeNumber(campaign.bonusFixed) + saleAmount * toNonNegativeNumber(campaign.bonusRate);
  return Number(totalBonus.toFixed(2));
}

export function normalizeCampaign(rawCampaign = {}) {
  const merged = {
    ...DEFAULT_CAMPAIGN,
    ...rawCampaign
  };
  const targetOutcome = VALID_OUTCOMES.has(merged.targetOutcome) ? merged.targetOutcome : DEFAULT_CAMPAIGN.targetOutcome;
  const existingCustomerFilter = VALID_EXISTING_FILTERS.has(merged.existingCustomerFilter)
    ? merged.existingCustomerFilter
    : DEFAULT_CAMPAIGN.existingCustomerFilter;

  return {
    ...merged,
    id: String(merged.id || "").trim(),
    name: String(merged.name || "").trim(),
    description: String(merged.description || "").trim(),
    startsAt: String(merged.startsAt || "").trim(),
    endsAt: String(merged.endsAt || "").trim(),
    isActive: Boolean(merged.isActive),
    targetOutcome,
    minSaleAmount: toNonNegativeNumber(merged.minSaleAmount),
    maxServiceMinutes: toNonNegativeNumber(merged.maxServiceMinutes),
    sourceIds: uniqueList(merged.sourceIds),
    reasonIds: uniqueList(merged.reasonIds),
    queueJumpOnly: Boolean(merged.queueJumpOnly),
    existingCustomerFilter,
    bonusFixed: toNonNegativeNumber(merged.bonusFixed),
    bonusRate: toNonNegativeNumber(merged.bonusRate)
  };
}

export function applyCampaignsToHistoryEntry(campaigns = [], historyEntry = {}) {
  const finishedAt = Number(historyEntry.finishedAt || Date.now());
  const durationMs = Number(historyEntry.durationMs || 0);
  const saleAmount = Number(historyEntry.saleAmount || 0);
  const customerSources = uniqueList(historyEntry.customerSources);
  const visitReasons = uniqueList(historyEntry.visitReasons);
  const matches = [];

  campaigns.forEach((rawCampaign) => {
    const campaign = normalizeCampaign(rawCampaign);

    if (!campaign.id || !campaign.name || !campaign.isActive) {
      return;
    }

    const startMs = toStartOfDayMs(campaign.startsAt);
    const endMs = toEndOfDayMs(campaign.endsAt);

    if (startMs !== null && finishedAt < startMs) {
      return;
    }

    if (endMs !== null && finishedAt > endMs) {
      return;
    }

    if (!matchesOutcome(campaign.targetOutcome, historyEntry.finishOutcome)) {
      return;
    }

    if (campaign.minSaleAmount > 0 && saleAmount < campaign.minSaleAmount) {
      return;
    }

    if (campaign.maxServiceMinutes > 0 && durationMs > campaign.maxServiceMinutes * 60000) {
      return;
    }

    if (!intersects(campaign.sourceIds, customerSources)) {
      return;
    }

    if (!intersects(campaign.reasonIds, visitReasons)) {
      return;
    }

    if (campaign.queueJumpOnly && historyEntry.startMode !== "queue-jump") {
      return;
    }

    if (!matchesExistingCustomer(campaign.existingCustomerFilter, historyEntry.isExistingCustomer)) {
      return;
    }

    matches.push({
      campaignId: campaign.id,
      campaignName: campaign.name,
      bonusValue: calculateCampaignBonus(campaign, historyEntry)
    });
  });

  return {
    matches,
    totalBonus: Number(matches.reduce((sum, item) => sum + Number(item.bonusValue || 0), 0).toFixed(2))
  };
}

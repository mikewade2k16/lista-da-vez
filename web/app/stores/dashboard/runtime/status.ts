export function getConsultantStatus(state, consultantId) {
  if (state.activeServices.some((item) => item.id === consultantId)) {
    return "service";
  }

  if (state.waitingList.some((item) => item.id === consultantId)) {
    return "queue";
  }

  if (state.pausedEmployees.some((item) => item.personId === consultantId)) {
    return "paused";
  }

  return "available";
}

export function getConsultantStatusStartedAt(state, consultantId, timestamp) {
  const now = Number(timestamp || Date.now());
  const activeService = state.activeServices.find((item) => item.id === consultantId);

  if (activeService) {
    return Number(activeService.serviceStartedAt || now);
  }

  const waitingItem = state.waitingList.find((item) => item.id === consultantId);

  if (waitingItem) {
    return Number(waitingItem.queueJoinedAt || now);
  }

  const pausedItem = state.pausedEmployees.find((item) => item.personId === consultantId);

  if (pausedItem) {
    return Number(pausedItem.startedAt || now);
  }

  return now;
}

export function initializeConsultantStatuses(state, timestamp) {
  const now = Number(timestamp || Date.now());
  const statusMap = {};

  state.roster.forEach((consultant) => {
    const status = getConsultantStatus(state, consultant.id);

    statusMap[consultant.id] = {
      status,
      startedAt: getConsultantStatusStartedAt(state, consultant.id, now)
    };
  });

  return statusMap;
}

export function reconcileConsultantStatuses(state, timestamp) {
  const now = Number(timestamp || Date.now());
  const currentStatus =
    state.consultantCurrentStatus && typeof state.consultantCurrentStatus === "object"
      ? state.consultantCurrentStatus
      : {};
  const normalized = {};

  state.roster.forEach((consultant) => {
    const consultantId = consultant.id;
    const derivedStatus = getConsultantStatus(state, consultantId);
    const expectedStartedAt = getConsultantStatusStartedAt(state, consultantId, now);
    const previous = currentStatus[consultantId];

    if (previous && previous.status === derivedStatus) {
      normalized[consultantId] = {
        status: derivedStatus,
        startedAt:
          derivedStatus === "available"
            ? Number(previous.startedAt || now)
            : expectedStartedAt
      };
      return;
    }

    normalized[consultantId] = {
      status: derivedStatus,
      startedAt: derivedStatus === "available" ? now : expectedStartedAt
    };
  });

  return normalized;
}

export function applyStatusTransitions(state, transitions, timestamp) {
  const now = Number(timestamp || Date.now());
  const currentStatus = { ...state.consultantCurrentStatus };
  const sessions = [...state.consultantActivitySessions];

  transitions.forEach(({ personId, nextStatus }) => {
    if (!personId || !nextStatus) {
      return;
    }

    const previous = currentStatus[personId] || { status: "available", startedAt: now };

    if (previous.status === nextStatus) {
      if (!currentStatus[personId]) {
        currentStatus[personId] = previous;
      }
      return;
    }

    sessions.push({
      personId,
      status: previous.status,
      startedAt: previous.startedAt,
      endedAt: now,
      durationMs: Math.max(0, now - previous.startedAt)
    });

    currentStatus[personId] = {
      status: nextStatus,
      startedAt: now
    };
  });

  return {
    consultantActivitySessions: sessions,
    consultantCurrentStatus: currentStatus
  };
}

import {
  buildConsultantColor,
  buildConsultantInitials,
  createOptionId
} from "~/stores/dashboard/runtime/shared";

export function createConsultantActions({ getState, updateState }) {
  return {
    createConsultantProfile({
      name,
      role,
      color,
      monthlyGoal,
      commissionRate,
      conversionGoal = 0,
      avgTicketGoal = 0,
      paGoal = 0
    }) {
      const state = getState();
      const normalizedName = String(name || "").trim();
      const normalizedRole = String(role || "Atendimento").trim() || "Atendimento";
      const goal = Math.max(0, Number(monthlyGoal) || 0);
      const commission = Math.max(0, Number(commissionRate) || 0);

      if (!normalizedName) {
        return { ok: false, message: "Nome do consultor e obrigatorio." };
      }

      const consultantId = createOptionId("consultor", normalizedName, state.roster);
      const consultant = {
        id: consultantId,
        name: normalizedName,
        role: normalizedRole,
        initials: buildConsultantInitials(normalizedName),
        color: color?.trim() || buildConsultantColor(state.roster),
        monthlyGoal: goal,
        commissionRate: commission,
        conversionGoal: Math.max(0, Math.min(100, Number(conversionGoal) || 0)),
        avgTicketGoal: Math.max(0, Number(avgTicketGoal) || 0),
        paGoal: Math.max(0, Number(paGoal) || 0)
      };
      const now = Date.now();
      const consultantCurrentStatus = {
        ...state.consultantCurrentStatus,
        [consultantId]: {
          status: "available",
          startedAt: now
        }
      };

      updateState({
        ...state,
        roster: [...state.roster, consultant],
        consultantCurrentStatus,
        selectedConsultantId: consultantId
      });

      return { ok: true };
    },

    updateConsultantProfile(consultantId, patch) {
      const state = getState();
      const existing = state.roster.find((consultant) => consultant.id === consultantId);

      if (!existing) {
        return { ok: false, message: "Consultor nao encontrado." };
      }

      const name = String((patch.name ?? existing.name) || "").trim();
      const role = String((patch.role ?? existing.role) || "").trim() || "Atendimento";
      const monthlyGoal = Math.max(0, Number(patch.monthlyGoal ?? existing.monthlyGoal) || 0);
      const commissionRate = Math.max(0, Number(patch.commissionRate ?? existing.commissionRate) || 0);
      const conversionGoal = Math.max(0, Math.min(100, Number(patch.conversionGoal ?? existing.conversionGoal) || 0));
      const avgTicketGoal = Math.max(0, Number(patch.avgTicketGoal ?? existing.avgTicketGoal) || 0);
      const paGoal = Math.max(0, Number(patch.paGoal ?? existing.paGoal) || 0);
      const color = String((patch.color ?? existing.color) || "").trim() || existing.color;
      const initials = buildConsultantInitials(name || existing.name);
      const nextConsultant = {
        ...existing,
        name: name || existing.name,
        role,
        initials,
        color,
        monthlyGoal,
        commissionRate,
        conversionGoal,
        avgTicketGoal,
        paGoal
      };

      updateState({
        ...state,
        roster: state.roster.map((consultant) => (consultant.id === consultantId ? nextConsultant : consultant)),
        waitingList: state.waitingList.map((item) => (item.id === consultantId ? { ...item, ...nextConsultant } : item)),
        activeServices: state.activeServices.map((item) => (item.id === consultantId ? { ...item, ...nextConsultant } : item))
      });

      return { ok: true };
    },

    archiveConsultantProfile(consultantId) {
      const state = getState();
      const consultant = state.roster.find((item) => item.id === consultantId);

      if (!consultant) {
        return { ok: false, message: "Consultor nao encontrado." };
      }

      const isInQueue = state.waitingList.some((item) => item.id === consultantId);
      const isInService = state.activeServices.some((item) => item.id === consultantId);
      const isPaused = state.pausedEmployees.some((item) => item.personId === consultantId);

      if (isInQueue || isInService || isPaused) {
        return {
          ok: false,
          message: "Retire o consultor de fila, atendimento ou pausa antes de arquivar."
        };
      }

      const nextCurrentStatus = { ...state.consultantCurrentStatus };
      delete nextCurrentStatus[consultantId];

      updateState({
        ...state,
        roster: state.roster.filter((item) => item.id !== consultantId),
        consultantCurrentStatus: nextCurrentStatus,
        selectedConsultantId:
          state.selectedConsultantId === consultantId
            ? state.roster.find((item) => item.id !== consultantId)?.id || null
            : state.selectedConsultantId
      });

      return { ok: true };
    },

    setSelectedConsultant(personId) {
      const state = getState();

      if (!state.roster.some((consultant) => consultant.id === personId)) {
        return;
      }

      updateState({
        ...state,
        selectedConsultantId: personId
      });
    },

    setConsultantSimulationAdditionalSales(amount) {
      const state = getState();

      updateState({
        ...state,
        consultantSimulationAdditionalSales: Math.max(0, Number(amount) || 0)
      });
    }
  };
}

import { cloneValue } from '~/domain/utils/object'

// Normalizadores puros do payload remoto (sem dependencia de store/fetch). Cada
// funcao defende contra shape parcial vindo do back e devolve um objeto estavel.
// Extraido de runtime-remote.ts para manter cada arquivo dentro do limite de
// linhas (ver principios de engenharia).

// Entrada de status de um consultor dentro do mapa consultantCurrentStatus.
interface ConsultantStatusEntry {
  status?: string
  startedAt?: number
}

// Payload parcial do snapshot de operacao vindo de /v1/operations/snapshot.
// Todos os campos sao opcionais: a funcao normalizeOperationSnapshot defende
// contra shape parcial (back stale, degraded, null) e devolve estado estavel.
// Os arrays usam Record<string, unknown> para que os normalizadores possam
// acessar propriedades via optional chaining sem abrir mao da seguranca de tipo.
interface OperationSnapshotPayload {
  waitingList?: Record<string, unknown>[]
  activeServices?: Record<string, unknown>[]
  pausedEmployees?: Record<string, unknown>[]
  consultantActivitySessions?: Record<string, unknown>[]
  consultantCurrentStatus?: Record<string, ConsultantStatusEntry>
  serviceHistory?: unknown[]
  pendingValidations?: Record<string, unknown>[]
  roster?: unknown[]
}

export function cloneOrFallback(value, fallback) {
  return cloneValue(value === undefined ? fallback : value)
}

export function normalizeOptions(options = []) {
  return (Array.isArray(options) ? options : [])
    .map((option) => ({
      id: String(option?.id || '').trim(),
      label: String(option?.label || '').trim(),
    }))
    .filter((option) => option.id && option.label)
}

export function normalizeProducts(products = []) {
  return (Array.isArray(products) ? products : [])
    .map((product) => ({
      id: String(product?.id || '').trim(),
      name: String(product?.name || '').trim(),
      code: String(product?.code || '')
        .trim()
        .toUpperCase(),
      category: String(product?.category || '').trim(),
      basePrice: Math.max(0, Number(product?.basePrice || 0) || 0),
    }))
    .filter((product) => product.id && product.name)
}

export function normalizeConsultants(consultants = []) {
  return (Array.isArray(consultants) ? consultants : [])
    .map((consultant) => ({
      id: String(consultant?.id || '').trim(),
      storeId: String(consultant?.storeId || '').trim(),
      name: String(consultant?.name || '').trim(),
      role: String(consultant?.role || '').trim() || 'Atendimento',
      initials: String(consultant?.initials || '').trim(),
      color: String(consultant?.color || '').trim() || '#168aad',
      monthlyGoal: Math.max(0, Number(consultant?.monthlyGoal || 0) || 0),
      commissionRate: Math.max(0, Number(consultant?.commissionRate || 0) || 0),
      conversionGoal: Math.max(0, Number(consultant?.conversionGoal || 0) || 0),
      avgTicketGoal: Math.max(0, Number(consultant?.avgTicketGoal || 0) || 0),
      paGoal: Math.max(0, Number(consultant?.paGoal || 0) || 0),
      active: Boolean(consultant?.active ?? true),
      access:
        consultant?.access && typeof consultant.access === 'object'
          ? {
              userId: String(consultant.access?.userId || '').trim(),
              email: String(consultant.access?.email || '')
                .trim()
                .toLowerCase(),
              active: Boolean(consultant.access?.active ?? false),
            }
          : null,
    }))
    .filter((consultant) => consultant.id && consultant.name)
}

// O roster da faixa de consultores vem de /v1/consultants (gestao, restrito a
// papeis com consultor.view/settings.view). Papeis operadores sem essa permissao
// (ex.: consultor) NAO buscam esse endpoint; nesse caso usamos o roster ENXUTO
// que o snapshot da operacao ja entrega (id/nome/iniciais/cor). Assim toda role
// que pode operar a fila recebe a faixa, sem vazar meta/comissao/e-mail. O roster
// completo (quando presente) sempre vence, para nao perder metas de quem ja tem
// a permissao de gestao.
export function resolveOperationRoster(consultants, operationSnapshot) {
  const managedRoster = normalizeConsultants(consultants)
  if (managedRoster.length) {
    return managedRoster
  }

  return normalizeConsultants(operationSnapshot?.roster)
}

export function normalizeOperationSnapshot(snapshot: OperationSnapshotPayload = {}) {
  return {
    waitingList: Array.isArray(snapshot?.waitingList)
      ? snapshot.waitingList.map((item) => ({
          ...item,
          queueJoinedAt: Math.max(0, Number(item?.queueJoinedAt || 0) || 0),
        }))
      : [],
    activeServices: Array.isArray(snapshot?.activeServices)
      ? snapshot.activeServices.map((item) => ({
          ...item,
          serviceStartedAt: Math.max(0, Number(item?.serviceStartedAt || 0) || 0),
          queueJoinedAt: Math.max(0, Number(item?.queueJoinedAt || 0) || 0),
          queueWaitMs: Math.max(0, Number(item?.queueWaitMs || 0) || 0),
          queuePositionAtStart: Math.max(1, Number(item?.queuePositionAtStart || 1) || 1),
          skippedPeople: Array.isArray(item?.skippedPeople) ? item.skippedPeople : [],
          stoppedAt: Math.max(0, Number(item?.stoppedAt || 0) || 0),
          effectiveFinishedAt: Math.max(0, Number(item?.effectiveFinishedAt || 0) || 0),
          stopReason: String(item?.stopReason || '').trim(),
          // Auto-encerramento (2h): epoch ms de servidor; a barra do card compara
          // graceDeadline contra adjustedNow (nunca Date.now).
          graceDeadline: Math.max(0, Number(item?.graceDeadline || 0) || 0),
          snoozedUntil: Math.max(0, Number(item?.snoozedUntil || 0) || 0),
          snoozeCount: Math.max(0, Number(item?.snoozeCount || 0) || 0),
        }))
      : [],
    pausedEmployees: Array.isArray(snapshot?.pausedEmployees)
      ? snapshot.pausedEmployees
          .map((item) => ({
            personId: String(item?.personId || '').trim(),
            reason: String(item?.reason || '').trim(),
            kind: String(item?.kind || 'pause').trim() || 'pause',
            startedAt: Math.max(0, Number(item?.startedAt || 0) || 0),
          }))
          .filter((item) => item.personId)
      : [],
    consultantActivitySessions: Array.isArray(snapshot?.consultantActivitySessions)
      ? snapshot.consultantActivitySessions
          .map((item) => ({
            personId: String(item?.personId || '').trim(),
            status: String(item?.status || '').trim(),
            startedAt: Math.max(0, Number(item?.startedAt || 0) || 0),
            endedAt: Math.max(0, Number(item?.endedAt || 0) || 0),
            durationMs: Math.max(0, Number(item?.durationMs || 0) || 0),
          }))
          .filter((item) => item.personId)
      : [],
    consultantCurrentStatus:
      snapshot?.consultantCurrentStatus && typeof snapshot.consultantCurrentStatus === 'object'
        ? Object.fromEntries(
            Object.entries(snapshot.consultantCurrentStatus)
              .map(([consultantId, value]) => [
                String(consultantId || '').trim(),
                {
                  status: String(value?.status || '').trim(),
                  startedAt: Math.max(0, Number(value?.startedAt || 0) || 0),
                },
              ])
              .filter(([consultantId]) => consultantId),
          )
        : {},
    serviceHistory: Array.isArray(snapshot?.serviceHistory) ? snapshot.serviceHistory : [],
    // Auto-encerramento (2h): atendimentos fechados pelo sweep aguardando o gerente
    // validar/cancelar. Array proprio (o servico ja saiu de activeServices).
    pendingValidations: Array.isArray(snapshot?.pendingValidations)
      ? snapshot.pendingValidations
          .map((item) => ({
            serviceId: String(item?.serviceId || '').trim(),
            storeId: String(item?.storeId || '').trim(),
            personId: String(item?.personId || '').trim(),
            personName: String(item?.personName || '').trim(),
            startedAt: Math.max(0, Number(item?.startedAt || 0) || 0),
            finishedAt: Math.max(0, Number(item?.finishedAt || 0) || 0),
            autoClosedAt: Math.max(0, Number(item?.autoClosedAt || 0) || 0),
            durationMs: Math.max(0, Number(item?.durationMs || 0) || 0),
            snoozeCount: Math.max(0, Number(item?.snoozeCount || 0) || 0),
          }))
          .filter((item) => item.serviceId)
      : [],
  }
}

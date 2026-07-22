import { ref } from "vue";
import type { Conversation } from "~/types";
import { useApi } from "~/composables/useApi";
import { getApiErrorMessage } from "~/utils/api-client";

export interface OmnichannelQueueView {
  id: string;
  departmentId: string;
  slug: string;
  name: string;
  isDefault: boolean;
  isActive: boolean;
}

export interface OmnichannelHandoffView {
  id: string;
  conversationId: string;
  reasonCode: string;
  summary: string;
  collectedFields: Record<string, unknown>;
  sourceState: string;
  targetQueueId: string | null;
  status: string;
  acceptedByUserId: string | null;
  requestedAt: string;
  queuedAt: string | null;
  acceptedAt: string | null;
  closedAt: string | null;
  createdAt: string;
  updatedAt: string;
}

export interface OmnichannelSLAEventView {
  id: string;
  conversationId: string;
  handoffId: string | null;
  eventType: string;
  idempotencyKey: string;
  occurredAt: string;
  metadata: Record<string, unknown>;
}

function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? value as T[] : [];
}

export function useOmnichannelHandoff() {
  const { apiFetch } = useApi();
  const queues = ref<OmnichannelQueueView[]>([]);
  const handoffs = ref<OmnichannelHandoffView[]>([]);
  const slaEvents = ref<OmnichannelSLAEventView[]>([]);
  const loadingQueues = ref(false);
  const loadingConversationOps = ref(false);
  const transferringQueue = ref(false);
  const handoffError = ref("");
  const queueError = ref("");
  let conversationLoadSequence = 0;

  async function loadQueues() {
    if (loadingQueues.value) {
      return;
    }

    loadingQueues.value = true;
    queueError.value = "";
    try {
      const result = await apiFetch<unknown>("/settings/queues");
      queues.value = asArray<OmnichannelQueueView>(result).filter((entry) => entry && entry.isActive);
    } catch (error) {
      queues.value = [];
      queueError.value = getApiErrorMessage(error, "Não foi possível carregar as filas.");
    } finally {
      loadingQueues.value = false;
    }
  }

  async function loadConversationOperations(conversationId: string | null) {
    const normalizedId = String(conversationId ?? "").trim();
    const sequence = ++conversationLoadSequence;
    handoffError.value = "";
    handoffs.value = [];
    slaEvents.value = [];

    if (!normalizedId) {
      loadingConversationOps.value = false;
      return;
    }

    loadingConversationOps.value = true;
    try {
      const [handoffResult, slaResult] = await Promise.all([
        apiFetch<unknown>(`/conversations/${normalizedId}/handoffs`),
        apiFetch<unknown>(`/conversations/${normalizedId}/sla`)
      ]);

      if (sequence !== conversationLoadSequence) {
        return;
      }

      handoffs.value = asArray<OmnichannelHandoffView>(handoffResult);
      slaEvents.value = asArray<OmnichannelSLAEventView>(slaResult);
    } catch (error) {
      if (sequence !== conversationLoadSequence) {
        return;
      }

      handoffError.value = getApiErrorMessage(error, "Não foi possível carregar o handoff e o SLA.");
    } finally {
      if (sequence === conversationLoadSequence) {
        loadingConversationOps.value = false;
      }
    }
  }

  async function transferConversation(conversationId: string, queueId: string): Promise<Conversation | null> {
    const normalizedConversationId = String(conversationId ?? "").trim();
    const normalizedQueueId = String(queueId ?? "").trim();
    if (!normalizedConversationId || !normalizedQueueId) {
      return null;
    }

    transferringQueue.value = true;
    handoffError.value = "";
    try {
      const updated = await apiFetch<Conversation>(`/conversations/${normalizedConversationId}/queue`, {
        method: "PATCH",
        body: { queueId: normalizedQueueId }
      });
      await loadConversationOperations(normalizedConversationId);
      return updated;
    } catch (error) {
      handoffError.value = getApiErrorMessage(error, "Não foi possível transferir a conversa.");
      return null;
    } finally {
      transferringQueue.value = false;
    }
  }

  return {
    queues,
    handoffs,
    slaEvents,
    loadingQueues,
    loadingConversationOps,
    transferringQueue,
    handoffError,
    queueError,
    loadQueues,
    loadConversationOperations,
    transferConversation
  };
}

<script setup lang="ts">
import {
  UAvatar,
  UBadge,
  UButton,
  UCard,
  UDashboardSidebar,
  UFormField,
  USelect,
  UTextarea
} from "#components";
import { computed, nextTick, onMounted, onUpdated, ref } from "vue";
import type { Conversation, ConversationStatus } from "~/types";
import type {
  OmnichannelHandoffView,
  OmnichannelSLAEventView
} from "~/composables/omnichannel/useOmnichannelHandoff";
import type { InboxSelectOption } from "./types";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";

const props = defineProps<{
  collapsed: boolean;
  activeConversation: Conversation | null;
  activeConversationLabel: string | null;
  isGroupConversation: boolean;
  savingContact: boolean;
  contactActionError: string;
  canSaveActiveContact: boolean;
  statusActionItems: InboxSelectOption[];
  assigneeItems: InboxSelectOption[];
  assigneeModel: string;
  updatingStatus: boolean;
  updatingAssignee: boolean;
  loadingUsers: boolean;
  internalNotes: string;
  canManageConversation: boolean;
  handoffItems: OmnichannelHandoffView[];
  slaEvents: OmnichannelSLAEventView[];
  queueItems: InboxSelectOption[];
  loadingHandoff: boolean;
  loadingQueues: boolean;
  transferringQueue: boolean;
  handoffError: string;
  queueError: string;
}>();

const emit = defineEmits<{
  (event: "update:collapsed", value: boolean): void;
  (event: "update:internalNotes", value: string): void;
  (event: "update:assigneeModel", value: string): void;
  (event: "save-contact"): void;
  (event: "update-status", value: ConversationStatus): void;
  (event: "update-assignee", value: string): void;
  (event: "transfer-queue", value: string): void;
}>();

const collapsedModel = computed({
  get: () => props.collapsed,
  set: (value: boolean) => emit("update:collapsed", value)
});

const internalNotesModel = computed({
  get: () => props.internalNotes,
  set: (value: string) => emit("update:internalNotes", value)
});

const selectedQueueModel = ref("");

const latestHandoff = computed(() => props.handoffItems[0] ?? null);

function onQueueChange(value: string | undefined) {
  selectedQueueModel.value = value ?? "";
}

function transferSelectedQueue() {
  const queueId = selectedQueueModel.value.trim();
  if (!queueId) {
    return;
  }

  emit("transfer-queue", queueId);
  selectedQueueModel.value = "";
}

function formatDate(value: string | null | undefined) {
  if (!value) {
    return "—";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }

  return date.toLocaleString("pt-BR", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  });
}

function handoffReasonLabel(value: string) {
  const labels: Record<string, string> = {
    requested: "Solicitado",
    low_confidence: "Baixa confiança",
    max_turns: "Limite de turnos",
    tool_failed: "Falha de ferramenta",
    policy: "Política",
    error: "Erro"
  };

  return labels[value] ?? value;
}

function handoffStatusLabel(value: string) {
  const labels: Record<string, string> = {
    requested: "Solicitado",
    queued: "Na fila",
    accepted: "Aceito",
    cancelled: "Cancelado",
    closed: "Encerrado"
  };

  return labels[value] ?? value;
}

function slaEventLabel(value: string) {
  const labels: Record<string, string> = {
    started: "SLA iniciado",
    warning: "SLA em risco",
    breached: "SLA violado",
    paused: "SLA pausado",
    resumed: "SLA retomado",
    satisfied: "SLA atendido"
  };

  return labels[value] ?? value;
}

function collectedFieldLabels(value: Record<string, unknown>) {
  return Object.keys(value).filter(Boolean).slice(0, 12);
}

function getInitials(value: string | null | undefined) {
  if (!value) {
    return "?";
  }

  const parts = value
    .trim()
    .split(/\s+/)
    .filter(Boolean);

  if (!parts.length) {
    return "?";
  }

  if (parts.length === 1) {
    return parts[0].slice(0, 1).toUpperCase();
  }

  return `${parts[0].slice(0, 1)}${parts[parts.length - 1].slice(0, 1)}`.toUpperCase();
}

function getChannelLabel(channelValue: Conversation["channel"]) {
  if (channelValue === "WHATSAPP") {
    return "WhatsApp";
  }

  if (channelValue === "INSTAGRAM") {
    return "Instagram";
  }

  return channelValue;
}

function getStatusLabel(statusValue: Conversation["status"]) {
  if (statusValue === "OPEN") {
    return "Aberto";
  }

  if (statusValue === "PENDING") {
    return "Pendente";
  }

  return "Encerrado";
}

function getStatusColor(statusValue: Conversation["status"]) {
  if (statusValue === "OPEN") {
    return "success";
  }

  if (statusValue === "PENDING") {
    return "warning";
  }

  return "neutral";
}

function onStatusChange(value: string | undefined) {
  if (!value) {
    return;
  }

  emit("update-status", value as ConversationStatus);
}

function onAssigneeChange(value: string | undefined) {
  if (!value) {
    return;
  }

  emit("update:assigneeModel", value);
  emit("update-assignee", value);
}

function stripSidebarMinHeightClass() {
  if (!import.meta.client) {
    return;
  }

  document
    .querySelectorAll<HTMLElement>(".chat-page__sidebar--right.min-h-svh")
    .forEach((sidebarElement) => {
      sidebarElement.classList.remove("min-h-svh");
    });
}

onMounted(() => {
  void nextTick(() => {
    stripSidebarMinHeightClass();
  });
});

onUpdated(() => {
  stripSidebarMinHeightClass();
});

function toggleDetailsVisibility() {
  collapsedModel.value = !collapsedModel.value;
}
</script>

<template>
  <UDashboardSidebar
    id="omni-inbox-right"
    side="right"
    resizable
    :min-size="16"
    :default-size="24"
    :max-size="34"
    :ui="{
      root: 'chat-page__sidebar chat-page__sidebar--right !min-h-0 !h-full',
      header: 'chat-page__panel-header chat-page__panel-header--sidebar',
      body: 'chat-page__sidebar-body'
    }"
  >
    <template #header>
      <div class="chat-page__details-head">
        <h2 class="chat-page__details-title">Detalhes</h2>
        <UButton
          color="neutral"
          variant="ghost"
          :icon="collapsedModel ? 'i-lucide-panel-right-open' : 'i-lucide-panel-right-close'"
          :aria-label="collapsedModel ? 'Expandir detalhes' : 'Minimizar detalhes'"
          @click="toggleDetailsVisibility"
        />
      </div>
    </template>

    <div class="chat-page__sidebar-content chat-page__details-content">
      <div v-if="collapsedModel" class="chat-page__details-minimized">
        <p class="chat-page__empty">Detalhes minimizados. Clique no icone para expandir novamente.</p>
      </div>

      <div v-else-if="activeConversation" class="chat-page__panel-body chat-page__details-body">
        <UCard>
          <template #header>
            <h3 class="details-card__title">Contato</h3>
          </template>

          <div class="details-card__contact">
            <UAvatar
              :src="resolveAvatarSource(activeConversation.contactAvatarUrl) || undefined"
              :alt="activeConversationLabel || 'Contato'"
              :text="getInitials(activeConversationLabel || 'Contato')"
              class="details-card__avatar"
            />
            <div class="details-card__contact-copy">
              <p class="details-card__text">{{ activeConversationLabel }}</p>
              <p v-if="activeConversation.contactPhone" class="details-card__subtext">
                {{ activeConversation.contactPhone }}
              </p>
            </div>
          </div>

          <div v-if="activeConversation.instanceId" class="details-card__contact-actions">
            <UBadge color="primary" variant="soft">
              {{ activeConversation.instanceDisplayName || activeConversation.instanceName }}
            </UBadge>
          </div>

          <div v-if="!isGroupConversation && canSaveActiveContact" class="details-card__contact-actions">
            <UButton
              size="sm"
              color="primary"
              variant="soft"
              :loading="savingContact"
              :disabled="savingContact"
              @click="emit('save-contact')"
            >
              Salvar contato
            </UButton>
          </div>

          <p v-if="contactActionError" class="details-card__error">
            {{ contactActionError }}
          </p>
        </UCard>

        <UCard>
          <template #header>
            <h3 class="details-card__title">Canal e status</h3>
          </template>

          <div class="details-card__tags">
            <UBadge color="neutral" variant="soft">{{ getChannelLabel(activeConversation.channel) }}</UBadge>
            <UBadge :color="getStatusColor(activeConversation.status)" variant="soft">
              {{ getStatusLabel(activeConversation.status) }}
            </UBadge>
          </div>
        </UCard>

        <UCard>
          <template #header>
            <h3 class="details-card__title">Handoff e SLA</h3>
          </template>

          <div v-if="loadingHandoff" class="details-card__muted">Carregando histórico operacional…</div>
          <div v-else-if="handoffError" class="details-card__error" role="alert">
            {{ handoffError }}
          </div>
          <template v-else>
            <div v-if="latestHandoff" class="details-card__handoff">
              <div class="details-card__tags">
                <UBadge color="primary" variant="soft">
                  {{ handoffStatusLabel(latestHandoff.status) }}
                </UBadge>
                <UBadge color="neutral" variant="soft">
                  {{ handoffReasonLabel(latestHandoff.reasonCode) }}
                </UBadge>
              </div>
              <p v-if="latestHandoff.summary" class="details-card__summary">
                {{ latestHandoff.summary }}
              </p>
              <div v-if="collectedFieldLabels(latestHandoff.collectedFields).length" class="details-card__field-list">
                <UBadge
                  v-for="field in collectedFieldLabels(latestHandoff.collectedFields)"
                  :key="field"
                  color="neutral"
                  variant="outline"
                  size="sm"
                >
                  {{ field }}
                </UBadge>
              </div>
              <p class="details-card__subtext">
                Atualizado em {{ formatDate(latestHandoff.updatedAt) }}
              </p>
            </div>
            <p v-else class="details-card__muted">Nenhum handoff registrado nesta conversa.</p>

            <div v-if="slaEvents.length" class="details-card__sla-list">
              <div v-for="event in slaEvents.slice(0, 4)" :key="event.id" class="details-card__sla-item">
                <span>{{ slaEventLabel(event.eventType) }}</span>
                <span class="details-card__subtext">{{ formatDate(event.occurredAt) }}</span>
              </div>
            </div>

            <div v-if="canManageConversation" class="details-card__transfer">
              <UFormField label="Transferir para fila" name="handoffQueue">
                <USelect
                  :model-value="selectedQueueModel"
                  :items="queueItems"
                  value-key="value"
                  placeholder="Selecione uma fila"
                  :loading="loadingQueues"
                  :disabled="loadingQueues || transferringQueue || queueItems.length === 0"
                  @update:model-value="onQueueChange"
                />
              </UFormField>
              <UButton
                size="sm"
                color="warning"
                variant="soft"
                icon="i-lucide-arrow-right-left"
                :loading="transferringQueue"
                :disabled="!selectedQueueModel || transferringQueue || queueItems.length === 0"
                @click="transferSelectedQueue"
              >
                Transferir
              </UButton>
              <p v-if="!loadingQueues && queueItems.length === 0" class="details-card__muted">
                Nenhuma fila ativa disponível para sua permissão.
              </p>
              <p v-if="queueError" class="details-card__error" role="alert">
                {{ queueError }}
              </p>
            </div>
          </template>
        </UCard>

        <UCard>
          <template #header>
            <h3 class="details-card__title">Acoes</h3>
          </template>

          <UFormField label="Status" name="conversationStatus">
            <USelect
              :model-value="activeConversation.status"
              :items="statusActionItems"
              value-key="value"
              :disabled="updatingStatus || !canManageConversation"
              @update:model-value="onStatusChange"
            />
          </UFormField>

          <UFormField label="Responsavel" name="conversationAssignee">
            <USelect
              :model-value="assigneeModel"
              :items="assigneeItems"
              value-key="value"
              :disabled="updatingAssignee || loadingUsers || !canManageConversation"
              @update:model-value="onAssigneeChange"
            />
          </UFormField>
        </UCard>

        <UCard>
          <template #header>
            <h3 class="details-card__title">Notas internas</h3>
          </template>

          <UTextarea
            v-model="internalNotesModel"
            :rows="4"
            autoresize
            placeholder="Adicione observacoes sobre esse contato"
          />
        </UCard>
      </div>

      <div v-else class="chat-page__empty">Selecione uma conversa para visualizar os detalhes.</div>
    </div>
  </UDashboardSidebar>
</template>

<style scoped>
.chat-page__sidebar {
  min-height: 0;
}

.chat-page__sidebar-body,
.chat-page__sidebar-content,
.chat-page__details-content {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
}

.chat-page__panel-header,
.chat-page__details-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.chat-page__details-title,
.details-card__title {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
}

.chat-page__panel-body,
.chat-page__details-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.chat-page__details-minimized {
  display: flex;
  align-items: flex-start;
  justify-content: flex-start;
  padding: 0.75rem 0.5rem;
}

.chat-page__details-body {
  display: grid;
  align-content: start;
  gap: 0.6rem;
}

.chat-page__empty {
  color: rgb(var(--muted));
  font-size: 0.85rem;
}

.details-card__contact {
  display: flex;
  align-items: center;
  gap: 0.55rem;
}

.details-card__contact-copy {
  min-width: 0;
}

.details-card__text {
  margin: 0;
  font-weight: 500;
}

.details-card__subtext {
  margin: 0.2rem 0 0;
  font-size: 0.8rem;
  color: rgb(var(--muted));
}

.details-card__contact-actions {
  margin-top: 0.75rem;
}

.details-card__error {
  margin: 0.5rem 0 0;
  font-size: 0.8rem;
  color: rgb(var(--error));
}

.details-card__tags {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.details-card__muted {
  margin: 0;
  color: rgb(var(--muted));
  font-size: 0.8rem;
}

.details-card__handoff {
  display: grid;
  gap: 0.45rem;
}

.details-card__summary {
  margin: 0;
  font-size: 0.82rem;
  line-height: 1.35;
  white-space: pre-wrap;
}

.details-card__field-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.details-card__sla-list {
  display: grid;
  gap: 0.35rem;
  margin-top: 0.7rem;
  border-top: 1px solid rgb(var(--border) / 0.6);
  padding-top: 0.6rem;
}

.details-card__sla-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  font-size: 0.8rem;
}

.details-card__transfer {
  display: grid;
  gap: 0.5rem;
  margin-top: 0.8rem;
  border-top: 1px solid rgb(var(--border) / 0.6);
  padding-top: 0.7rem;
}
</style>

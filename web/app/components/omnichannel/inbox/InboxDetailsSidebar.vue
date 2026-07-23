<script setup lang="ts">
import {
  UAvatar,
  UBadge,
  UButton,
  UDashboardSidebar,
  UFormField,
  UInput,
  USelect,
  USwitch,
  UTextarea
} from "#components";
import { computed, nextTick, onMounted, onUpdated, ref, watch } from "vue";
import type { Conversation, ConversationStatus } from "~/types";
import type {
  OmnichannelHandoffView,
  OmnichannelSLAEventView
} from "~/composables/omnichannel/useOmnichannelHandoff";
import type { InboxSelectOption } from "./types";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";
import InboxDetailsSection from "./InboxDetailsSection.vue";
import type {
  ContactAIRestriction,
  ContactAIRestrictionInput
} from "~/domain/omnichannel/privacy-api";

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
  canManagePrivacy: boolean;
  hidingContact: boolean;
  aiRestriction: ContactAIRestriction | null;
  loadingAiRestriction: boolean;
  updatingAiRestriction: boolean;
  aiRestrictionError: string;
}>();

const emit = defineEmits<{
  (event: "update:collapsed", value: boolean): void;
  (event: "update:internalNotes", value: string): void;
  (event: "update:assigneeModel", value: string): void;
  (event: "save-contact"): void;
  (event: "update-status", value: ConversationStatus): void;
  (event: "update-assignee", value: string): void;
  (event: "transfer-queue", value: string): void;
  (event: "hide-contact"): void;
  (event: "update-ai-restriction", value: ContactAIRestrictionInput): void;
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
const aiRestrictionPreset = ref("allow");
const aiRestrictionCustomUntil = ref("");
const aiRestrictionItems: InboxSelectOption[] = [
  { label: "Permitir atendimento pela IA", value: "allow" },
  { label: "Bloquear por 24 horas", value: "24h" },
  { label: "Bloquear por 7 dias", value: "7d" },
  { label: "Bloquear por 30 dias", value: "30d" },
  { label: "Bloquear até uma data", value: "custom" },
  { label: "Bloquear por tempo indeterminado", value: "indefinite" }
];

function toLocalDateTime(value: string | null | undefined) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "";
  const shifted = new Date(date.getTime() - date.getTimezoneOffset() * 60_000);
  return shifted.toISOString().slice(0, 16);
}

watch(
  () => props.aiRestriction,
  (restriction) => {
    if (!restriction || !restriction.blocked) {
      aiRestrictionPreset.value = "allow";
      aiRestrictionCustomUntil.value = "";
      return;
    }
    if (restriction.mode === "indefinite") {
      aiRestrictionPreset.value = "indefinite";
      aiRestrictionCustomUntil.value = "";
      return;
    }
    aiRestrictionPreset.value = "custom";
    aiRestrictionCustomUntil.value = toLocalDateTime(restriction.blockedUntil);
  },
  { immediate: true }
);

function saveAIRestriction() {
  const preset = aiRestrictionPreset.value;
  if (preset === "allow" || preset === "indefinite") {
    emit("update-ai-restriction", { mode: preset });
    return;
  }
  let blockedUntil: Date;
  if (preset === "custom") {
    blockedUntil = new Date(aiRestrictionCustomUntil.value);
  } else {
    const durations: Record<string, number> = {
      "24h": 24 * 60 * 60 * 1000,
      "7d": 7 * 24 * 60 * 60 * 1000,
      "30d": 30 * 24 * 60 * 60 * 1000
    };
    blockedUntil = new Date(Date.now() + (durations[preset] ?? 0));
  }
  if (Number.isNaN(blockedUntil.getTime()) || blockedUntil.getTime() <= Date.now()) {
    return;
  }
  emit("update-ai-restriction", { mode: "until", blockedUntil: blockedUntil.toISOString() });
}

const savedAIRestrictionPreset = computed(() => {
  if (!props.aiRestriction?.blocked) return "allow";
  return props.aiRestriction.mode === "indefinite" ? "indefinite" : "custom";
});

const savedAIRestrictionCustomUntil = computed(() =>
  savedAIRestrictionPreset.value === "custom"
    ? toLocalDateTime(props.aiRestriction?.blockedUntil)
    : ""
);

const isAIRestrictionDirty = computed(() => {
  if (aiRestrictionPreset.value !== savedAIRestrictionPreset.value) return true;
  if (aiRestrictionPreset.value !== "custom") return false;
  return aiRestrictionCustomUntil.value !== savedAIRestrictionCustomUntil.value;
});

const canSaveAIRestriction = computed(() => {
  const validUntil =
    aiRestrictionPreset.value !== "custom" ||
    (Boolean(aiRestrictionCustomUntil.value) && new Date(aiRestrictionCustomUntil.value).getTime() > Date.now());
  return validUntil && isAIRestrictionDirty.value;
});

const latestHandoff = computed(() => props.handoffItems[0] ?? null);

const aiIndividualEnabled = computed(
  () => Boolean(props.aiRestriction && !props.aiRestriction.blocked),
);
const aiIndividualReady = computed(
  () =>
    Boolean(props.activeConversation) &&
    !props.isGroupConversation &&
    props.canManagePrivacy &&
    !props.loadingAiRestriction &&
    Boolean(props.aiRestriction),
);

function toggleIndividualAI(enabled: boolean) {
  if (!aiIndividualReady.value || props.updatingAiRestriction) return;
  emit("update-ai-restriction", { mode: enabled ? "allow" : "indefinite" });
}

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
      </div>
    </template>

    <div class="chat-page__sidebar-content chat-page__details-content">
      <div v-if="collapsedModel" class="chat-page__details-minimized">
        <p class="chat-page__empty">Detalhes minimizados. Clique no icone para expandir novamente.</p>
      </div>

      <div v-else-if="activeConversation" class="chat-page__panel-body chat-page__details-body">
        <div
          v-if="canManagePrivacy"
          class="details-card__ai-quick-control"
          :class="{ 'details-card__ai-quick-control--group': isGroupConversation }"
        >
          <div class="details-card__ai-quick-copy">
            <p class="details-card__control-label">Atendimento por IA</p>
            <p class="details-card__muted">
              {{ isGroupConversation ? 'Desativado para grupos.' : aiIndividualEnabled ? 'Ativo nesta pessoa.' : 'Parado nesta pessoa.' }}
            </p>
          </div>
          <USwitch
            v-if="!isGroupConversation"
            :model-value="aiIndividualEnabled"
            :loading="loadingAiRestriction"
            :disabled="!aiIndividualReady || updatingAiRestriction"
            aria-label="Ativar ou parar a IA nesta pessoa"
            @update:model-value="toggleIndividualAI"
          />
          <UBadge v-else color="neutral" variant="soft">Grupo · IA desativada</UBadge>
        </div>

        <InboxDetailsSection title="Contato">

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
            <UBadge color="neutral" variant="soft">
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
        </InboxDetailsSection>

        <InboxDetailsSection
          v-if="canManagePrivacy && !isGroupConversation"
          title="Visibilidade e IA"
          default-open
        >

          <div class="details-card__ai-control">
            <UFormField label="Atendimento por IA" name="aiRestriction">
              <USelect
                v-model="aiRestrictionPreset"
                :items="aiRestrictionItems"
                value-key="value"
                :loading="loadingAiRestriction"
                :disabled="loadingAiRestriction || updatingAiRestriction"
              />
            </UFormField>
            <UFormField
              v-if="aiRestrictionPreset === 'custom'"
              label="Bloquear até"
              name="aiRestrictionUntil"
            >
              <UInput
                v-model="aiRestrictionCustomUntil"
                type="datetime-local"
                :disabled="updatingAiRestriction"
              />
            </UFormField>
            <UButton
              v-if="isAIRestrictionDirty"
              class="details-card__save-rule"
              size="sm"
              color="primary"
              variant="solid"
              icon="i-lucide-check"
              :loading="updatingAiRestriction"
              :disabled="loadingAiRestriction || updatingAiRestriction || !canSaveAIRestriction"
              @click="saveAIRestriction"
            >
              Salvar alteração
            </UButton>
            <p v-if="aiRestrictionError" class="details-card__error" role="alert">
              {{ aiRestrictionError }}
            </p>
          </div>

          <div class="details-card__visibility-control">
            <div class="details-card__visibility-copy">
              <p class="details-card__control-label">Ocultar das conversas</p>
              <p class="details-card__muted">
                Remove a pessoa do Omnichannel e da Automação até que você a restaure.
              </p>
            </div>
            <UButton
              class="details-card__privacy-action"
              size="sm"
              color="warning"
              variant="ghost"
              icon="i-lucide-eye-off"
              :loading="hidingContact"
              :disabled="hidingContact || !activeConversation.contactId"
              @click="emit('hide-contact')"
            >
              Ocultar
            </UButton>
          </div>
        </InboxDetailsSection>

        <InboxDetailsSection title="Canal e status">

          <div class="details-card__tags">
            <UBadge color="neutral" variant="soft">{{ getChannelLabel(activeConversation.channel) }}</UBadge>
            <UBadge :color="getStatusColor(activeConversation.status)" variant="soft">
              {{ getStatusLabel(activeConversation.status) }}
            </UBadge>
          </div>
        </InboxDetailsSection>

        <InboxDetailsSection title="Handoff e SLA">

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
        </InboxDetailsSection>

        <InboxDetailsSection title="Ações">

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
        </InboxDetailsSection>

        <InboxDetailsSection title="Notas internas">

          <UTextarea
            v-model="internalNotesModel"
            :rows="4"
            autoresize
            placeholder="Adicione observacoes sobre esse contato"
          />
        </InboxDetailsSection>
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
  height: auto;
  flex: 1 1 auto;
  overflow: hidden;
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
  font-size: 0.88rem;
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
  display: flex;
  flex-direction: column;
  flex: 1 1 auto;
  gap: 0;
  padding-bottom: 0.75rem;
  overscroll-behavior: contain;
}

.chat-page__details-body > * {
  flex: 0 0 auto;
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

.details-card__ai-quick-control {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.55rem;
  padding: 0.65rem 0.75rem;
  border: 1px solid rgb(var(--success) / 0.2);
  border-radius: 0.75rem;
  background: rgb(var(--success) / 0.07);
}

.details-card__ai-quick-control--group {
  border-color: rgb(var(--muted) / 0.24);
  background: rgb(var(--surface-2) / 0.55);
}

.details-card__ai-quick-copy {
  min-width: 0;
}

.details-card__privacy-action {
  flex: 0 0 auto;
  align-self: center;
}

.details-card__ai-control,
.details-card__visibility-control {
  display: grid;
  gap: 0.6rem;
}

.details-card__visibility-control {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.75rem;
  margin-top: 0.55rem;
  border-top: 1px solid rgb(var(--border) / 0.6);
  padding-top: 0.65rem;
}

.details-card__visibility-copy {
  display: grid;
  min-width: 0;
  gap: 0.2rem;
}

.details-card__save-rule {
  justify-self: end;
}

.details-card__control-label {
  margin: 0;
  font-size: 0.82rem;
  font-weight: 600;
}

@media (max-width: 1120px) {
  .details-card__visibility-control {
    display: grid;
  }

  .details-card__privacy-action {
    justify-self: start;
  }
}
</style>

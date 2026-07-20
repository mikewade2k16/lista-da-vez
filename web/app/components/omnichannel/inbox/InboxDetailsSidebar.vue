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
import { computed, nextTick, onMounted, onUpdated } from "vue";
import type { Conversation, ConversationStatus } from "~/types";
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
}>();

const emit = defineEmits<{
  (event: "update:collapsed", value: boolean): void;
  (event: "update:internalNotes", value: string): void;
  (event: "update:assigneeModel", value: string): void;
  (event: "save-contact"): void;
  (event: "update-status", value: ConversationStatus): void;
  (event: "update-assignee", value: string): void;
}>();

const collapsedModel = computed({
  get: () => props.collapsed,
  set: (value: boolean) => emit("update:collapsed", value)
});

const internalNotesModel = computed({
  get: () => props.internalNotes,
  set: (value: string) => emit("update:internalNotes", value)
});

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
</style>

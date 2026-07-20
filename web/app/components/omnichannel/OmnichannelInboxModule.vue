<script setup lang="ts">
import { UAlert, UButton, UDashboardGroup } from "#components";
import { computed, nextTick, onMounted, onUpdated, ref } from "vue";
import OmnichannelInboxLoading from "./OmnichannelInboxLoading.vue";
import InboxChatPanel from "./inbox/InboxChatPanel.vue";
import InboxConversationsSidebar from "./inbox/InboxConversationsSidebar.vue";
import InboxDetailsSidebar from "./inbox/InboxDetailsSidebar.vue";
import InboxSaveContactModal from "./inbox/InboxSaveContactModal.vue";
import OmnichannelWhatsAppSessionModal from "./inbox/OmnichannelWhatsAppSessionModal.vue";
import { useOmnichannelInbox } from "~/composables/omnichannel/useOmnichannelInbox";
import {
  buildCanonicalContactPhone,
  normalizePhoneDigits
} from "~/composables/omnichannel/useOmnichannelInboxShared";

const sessionSimulation = useSessionSimulationStore();
const canSwitchTenant = computed(() => sessionSimulation.canSimulate && sessionSimulation.clientOptions.length > 1);

function normalizeModuleCode(value: unknown) {
  return String(value ?? "").trim().toLowerCase().replace(/\s+/g, "_");
}

const activeClientHasAtendimento = computed(() => sessionSimulation.hasModule("atendimento"));

const tenantSwitchOptions = computed(() =>
  sessionSimulation.clientOptions.map((option) => {
    const hasAtendimento = (option.moduleCodes ?? []).some((code) => normalizeModuleCode(code) === "atendimento");
    return {
      label: hasAtendimento ? option.label : `${option.label} (sem atendimento)`,
      value: option.value,
      disabled: !hasAtendimento
    };
  })
);
const tenantSwitchItems = computed(() =>
  tenantSwitchOptions.value.map((option) => ({
    label: option.label,
    value: String(option.value),
    disabled: option.disabled
  }))
);
const activeTenantLabel = computed(() => {
  const found = sessionSimulation.clientOptions.find(
    (option) => option.value === sessionSimulation.effectiveClientId
  );
  return found?.label ?? tenantSlug.value ?? "Tenant";
});

const {
  user,
  tenantSlug,
  leftCollapsed,
  rightCollapsed,
  showFilters,
  sidebarView,
  loadingConversations,
  loadingContacts,
  loadingUsers,
  loadingWhatsAppStatus,
  pageBootstrapping,
  realtimeConnectionState,
  whatsappConnectionState,
  loadingMessages,
  loadingOlderMessages,
  loadingGroupParticipants,
  hasMoreMessages,
  showLoadOlderMessagesButton,
  showScrollToLatestButton,
  savingContact,
  creatingContact,
  importingContacts,
  sendingMessage,
  sendError,
  contactActionError,
  contactImportPreview,
  contactImportResult,
  whatsappBannerMessage,
  isWhatsAppConfigured,
  isWhatsAppConnected,
  updatingStatus,
  updatingAssignee,
  stickyDateLabel,
  showStickyDate,
  draft,
  hasAttachment,
  attachmentType,
  attachmentName,
  attachmentMimeType,
  attachmentSizeBytes,
  attachmentDurationSeconds,
  attachmentPreviewUrl,
  search,
  channel,
  status,
  instanceId,
  replyTarget,
  assigneeModel,
  channelFilterItems,
  statusFilterItems,
  instanceFilterItems,
  statusActionItems,
  assigneeItems,
  activeConversation,
  activeConversationId,
  activeConversationLabel,
  conversations,
  whatsappInstances,
  contacts,
  filteredContacts,
  isGroupConversation,
  canSaveActiveContact,
  canManageConversation,
  activeGroupParticipants,
  internalNotes,
  filteredConversations,
  unreadConversationIds,
  mentionConversationIds,
  mentionConversationCounts,
  messageRenderItems,
  showOutboundOperatorLabel,
  onChatBodyMounted,
  onChatScroll,
  scrollToLatest,
  requestOlderMessages,
  setReplyTarget,
  clearReplyTarget,
  updateDraft,
  updateAttachment,
  clearAttachment,
  updateSearch,
  updateChannel,
  updateStatus,
  updateInstanceId,
  updateSidebarView,
  updateShowFilters,
  updateLeftCollapsed,
  updateRightCollapsed,
  updateAssigneeModel,
  updateInternalNotes,
  updateShowOutboundOperatorLabel,
  selectConversation,
  openContactConversation,
  createContactAndOpenConversation,
  previewWhatsAppContactsImport,
  applyWhatsAppContactsImport,
  clearWhatsAppContactsImportPreview,
  saveContactFromMessageCard,
  saveActiveConversationContact,
  deleteMessagesForMe,
  deleteMessagesForAll,
  forwardMessagesToConversation,
  closeConversation,
  sendMessage,
  sendContactCard,
  reactToMessage,
  updateConversationStatus,
  updateConversationAssignee,
  refreshAfterConversationHistoryClear,
  openMentionConversation,
  switchTenant,
  switchingTenant,
  switchTenantError
} = useOmnichannelInbox();

type SaveContactDraft = {
  name: string;
  phone: string;
  avatarUrl?: string | null;
  contactId?: string | null;
};

const saveContactModalOpen = ref(false);
const saveContactDraft = ref<SaveContactDraft | null>(null);
const saveContactModalError = ref("");
const whatsappSessionModalOpen = ref(false);

const showWhatsAppConnectionAlert = computed(() => {
  if (pageBootstrapping.value) {
    return false;
  }

  if (loadingWhatsAppStatus.value) {
    return false;
  }

  return !isWhatsAppConnected.value;
});

const whatsappConnectionAlertTitle = computed(() => {
  if (!isWhatsAppConfigured.value) {
    return "Nenhum WhatsApp configurado para este cliente";
  }

  if (whatsappConnectionState.value === "connecting") {
    return "WhatsApp aguardando conexao (leitura do QR Code)";
  }

  return "WhatsApp desconectado";
});

const whatsappConnectionAlertColor = computed(() => {
  if (!isWhatsAppConfigured.value) {
    return "error";
  }

  return "warning";
});

const sidebarWhatsappBannerMessage = computed(() => {
  if (pageBootstrapping.value || showWhatsAppConnectionAlert.value) {
    return "";
  }

  return whatsappBannerMessage.value;
});

const existingContactForDraft = computed(() => {
  const payload = saveContactDraft.value;
  if (!payload) {
    return null;
  }

  const normalizedPhone = buildCanonicalContactPhone({
    phone: payload.phone
  });
  if (!normalizedPhone) {
    return null;
  }

  return (
    contacts.value.find((contactEntry) => {
      const contactPhone = normalizePhoneDigits(contactEntry.phone);
      return contactPhone === normalizedPhone;
    }) ?? null
  );
});

function stripMinHeightUtilityClass() {
  if (!import.meta.client) {
    return;
  }

  document
    .querySelectorAll<HTMLElement>(".chat-page__chat.min-h-svh, .chat-page__sidebar.min-h-svh")
    .forEach((element) => {
      element.classList.remove("min-h-svh");
    });
}

onMounted(() => {
  void nextTick(() => {
    stripMinHeightUtilityClass();
  });
});

onUpdated(() => {
  stripMinHeightUtilityClass();
});

async function handleOpenContactFromCard(payload: {
  name: string;
  phone: string;
  contactId?: string | null;
  avatarUrl?: string | null;
}) {
  await saveContactFromMessageCard({
    ...payload,
    openConversation: true
  });
}

function formatContactDisplayPhone(value: string | null | undefined) {
  const trimmed = value?.trim() ?? "";
  if (!trimmed) {
    return "";
  }

  const digits = trimmed.replace(/\D/g, "");
  if (digits.startsWith("55") && digits.length >= 12) {
    return digits.slice(2);
  }

  return trimmed;
}

function handleOpenSaveContactModal(payload: SaveContactDraft) {
  saveContactDraft.value = {
    ...payload
  };
  saveContactModalError.value = "";
  saveContactModalOpen.value = true;
}

function handleCloseSaveContactModal() {
  saveContactModalOpen.value = false;
  saveContactDraft.value = null;
  saveContactModalError.value = "";
}

async function handleSaveContactModal(payload: SaveContactDraft) {
  const normalizedPhone = buildCanonicalContactPhone({
    phone: payload.phone
  });

  if (!normalizedPhone) {
    saveContactModalError.value = "Informe um telefone valido para salvar o contato.";
    return;
  }

  saveContactModalError.value = "";
  const saved = await saveContactFromMessageCard({
    name: payload.name,
    phone: normalizedPhone,
    avatarUrl: payload.avatarUrl ?? null,
    contactId: payload.contactId ?? null
  });

  if (saved) {
    handleCloseSaveContactModal();
    return;
  }

  saveContactModalError.value = contactActionError.value || "Nao foi possivel salvar o contato.";
}

async function handleOpenExistingContact(contactId: string) {
  await openContactConversation(contactId);
  handleCloseSaveContactModal();
}

async function handleSwitchTenant(clientId: number) {
  if (clientId === sessionSimulation.effectiveClientId) {
    return;
  }

  const target = sessionSimulation.clientOptions.find((option) => option.value === clientId);
  const hasAtendimento = (target?.moduleCodes ?? []).some((code) => normalizeModuleCode(code) === "atendimento");
  if (!hasAtendimento) {
    return;
  }

  sessionSimulation.setClientId(clientId);
  await switchTenant(clientId);
}

function handleOpenWhatsAppSessionModal() {
  whatsappSessionModalOpen.value = true;
}

async function handleSidebarTenantSwitch(clientId: string) {
  const parsedClientId = Number.parseInt(String(clientId ?? "").trim(), 10);
  if (!Number.isFinite(parsedClientId) || parsedClientId <= 0) {
    return;
  }

  await handleSwitchTenant(parsedClientId);
}

async function handleConversationHistoryCleared() {
  await refreshAfterConversationHistoryClear();
}
</script>
<template>
  <div class="chat-page">
    <OmnichannelWhatsAppSessionModal
      v-model:open="whatsappSessionModalOpen"
      @history-cleared="handleConversationHistoryCleared"
    />

    <InboxSaveContactModal
      :open="saveContactModalOpen"
      :payload="saveContactDraft"
      :existing-contact="existingContactForDraft"
      :saving="savingContact"
      :error-message="saveContactModalError"
      :format-display-phone="formatContactDisplayPhone"
      @close="handleCloseSaveContactModal"
      @save="handleSaveContactModal"
      @open-existing="handleOpenExistingContact"
    />

    <UAlert
      v-if="switchTenantError"
      class="chat-page__status-alert"
      color="error"
      variant="soft"
      :title="switchTenantError"
    />

    <UAlert
      v-if="realtimeConnectionState === 'module_denied'"
      class="chat-page__status-alert"
      color="warning"
      variant="soft"
      title="Realtime desativado: modulo Atendimento nao vinculado"
      description="Seu usuario nao tem acesso ao modulo de atendimento no plataforma-api. Mensagens serao atualizadas por polling. Solicite ao admin para vincular o modulo."
    />

    <UAlert
      v-if="showWhatsAppConnectionAlert"
      class="chat-page__status-alert"
      :color="whatsappConnectionAlertColor"
      variant="soft"
      :title="whatsappConnectionAlertTitle"
      :description="isWhatsAppConfigured ? 'Clique ao lado para gerar o QR Code e parear o WhatsApp.' : 'Clique ao lado para criar a primeira conexao deste cliente.'"
    >
      <template v-if="user?.role === 'ADMIN'" #actions>
        <UButton size="xs" color="primary" @click="handleOpenWhatsAppSessionModal">
          {{ isWhatsAppConfigured ? 'Conectar WhatsApp' : 'Configurar WhatsApp' }}
        </UButton>
      </template>
    </UAlert>

    <div v-if="!activeClientHasAtendimento" class="chat-page__no-module">
      <UAlert
        color="warning"
        variant="soft"
        icon="i-lucide-shield-alert"
        title="Modulo Atendimento nao disponivel"
        :description="`O cliente ${activeTenantLabel} nao possui o modulo de Atendimento ativo no auth central. Solicite a liberacao do modulo no shell.`"
      />
    </div>

    <OmnichannelInboxLoading v-else-if="pageBootstrapping" />

    <UDashboardGroup
      v-else
      storage="local"
      storage-key="omni-inbox-layout-v3"
      class="chat-page__dashboard !static !inset-auto !h-auto !w-full !min-h-0"
    >
      <InboxConversationsSidebar
        :collapsed="leftCollapsed"
        :show-filters="showFilters"
        :sidebar-view="sidebarView"
        :loading-conversations="loadingConversations"
        :loading-contacts="loadingContacts"
        :loading-whats-app-status="loadingWhatsAppStatus"
        :whatsapp-banner-message="sidebarWhatsappBannerMessage"
        :is-whats-app-connected="isWhatsAppConnected"
        :current-user-name="user?.name ?? null"
        :conversations="filteredConversations"
        :contacts="filteredContacts"
        :active-conversation-id="activeConversationId"
        :creating-contact="creatingContact"
        :importing-contacts="importingContacts"
        :contact-action-error="contactActionError"
        :contact-import-preview="contactImportPreview"
        :contact-import-result="contactImportResult"
        :unread-conversation-ids="unreadConversationIds"
        :mention-conversation-ids="mentionConversationIds"
        :mention-conversation-counts="mentionConversationCounts"
        :search="search"
        :channel="channel"
        :status="status"
        :instance-id="instanceId"
        :channel-filter-items="channelFilterItems"
        :status-filter-items="statusFilterItems"
        :instance-filter-items="instanceFilterItems"
        :available-instances="whatsappInstances"
        :can-switch-tenant="canSwitchTenant"
        :switching-tenant="switchingTenant"
        :active-tenant-id="String(sessionSimulation.effectiveClientId)"
        :tenant-switch-items="tenantSwitchItems"
        @update:collapsed="updateLeftCollapsed"
        @update:show-filters="updateShowFilters"
        @update:sidebar-view="updateSidebarView"
        @update:search="updateSearch"
        @update:channel="updateChannel"
        @update:status="updateStatus"
        @update:instance-id="updateInstanceId"
        @select-conversation="selectConversation"
        @open-contact="openContactConversation"
        @create-contact="createContactAndOpenConversation"
        @preview-contact-import="previewWhatsAppContactsImport"
        @apply-contact-import="applyWhatsAppContactsImport"
        @clear-contact-import="clearWhatsAppContactsImportPreview"
        @switch-tenant="handleSidebarTenantSwitch"
      />

      <InboxChatPanel
        :active-conversation="activeConversation"
        :active-conversation-label="activeConversationLabel"
        :conversation-options="conversations"
        :saved-contacts="contacts"
        :current-user-id="user?.id ?? null"
        :current-user-name="user?.name ?? null"
        :show-outbound-operator-label="showOutboundOperatorLabel"
        :user-role="user?.role"
        :loading-messages="loadingMessages"
        :loading-older-messages="loadingOlderMessages"
        :has-more-messages="hasMoreMessages"
        :show-load-older-messages-button="showLoadOlderMessagesButton"
        :show-scroll-to-latest-button="showScrollToLatestButton"
        :message-render-items="messageRenderItems"
        :show-sticky-date="showStickyDate"
        :sticky-date-label="stickyDateLabel"
        :is-group-conversation="isGroupConversation"
        :group-participants="activeGroupParticipants"
        :loading-group-participants="loadingGroupParticipants"
        :draft="draft"
        :has-attachment="hasAttachment"
        :attachment-type="attachmentType"
        :attachment-name="attachmentName"
        :attachment-mime-type="attachmentMimeType"
        :attachment-size-bytes="attachmentSizeBytes"
        :attachment-duration-seconds="attachmentDurationSeconds"
        :attachment-preview-url="attachmentPreviewUrl"
        :sending-message="sendingMessage"
        :send-error="sendError"
        :reply-target="replyTarget"
        :can-manage-conversation="canManageConversation"
        :delete-messages-for-me-action="deleteMessagesForMe"
        :delete-messages-for-all-action="deleteMessagesForAll"
        :forward-messages-action="forwardMessagesToConversation"
        @body-mounted="onChatBodyMounted"
        @chat-scroll="onChatScroll"
        @load-older-messages="requestOlderMessages"
        @scroll-to-latest="scrollToLatest"
        @send="sendMessage"
        @send-contact="sendContactCard"
        @save-contact-card="handleOpenSaveContactModal"
        @open-contact-card="handleOpenContactFromCard"
        @open-mention="openMentionConversation"
        @close-conversation="closeConversation"
        @open-whatsapp-session="handleOpenWhatsAppSessionModal"
        @set-reply="setReplyTarget"
        @clear-reply="clearReplyTarget"
        @update:draft="updateDraft"
        @pick-attachment="updateAttachment"
        @clear-attachment="clearAttachment"
        @set-reaction="reactToMessage"
        @update:show-outbound-operator-label="updateShowOutboundOperatorLabel"
      />

      <InboxDetailsSidebar
        :collapsed="rightCollapsed"
        :active-conversation="activeConversation"
        :active-conversation-label="activeConversationLabel"
        :is-group-conversation="isGroupConversation"
        :saving-contact="savingContact"
        :contact-action-error="contactActionError"
        :can-save-active-contact="canSaveActiveContact"
        :status-action-items="statusActionItems"
        :assignee-items="assigneeItems"
        :assignee-model="assigneeModel"
        :updating-status="updatingStatus"
        :updating-assignee="updatingAssignee"
        :loading-users="loadingUsers"
        :internal-notes="internalNotes"
        :can-manage-conversation="canManageConversation"
        @update:collapsed="updateRightCollapsed"
        @update:internal-notes="updateInternalNotes"
        @update:assignee-model="updateAssigneeModel"
        @save-contact="saveActiveConversationContact"
        @update-status="updateConversationStatus"
        @update-assignee="updateConversationAssignee"
      />
    </UDashboardGroup>
  </div>
</template>

<style scoped>
.chat-page {
  height: 100%;
  min-height: 0;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.chat-page__no-module {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  padding: 2rem;
}

.chat-page__status-alert {
  margin: 0.5rem 0.5rem 0;
}

.chat-page__dashboard {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
}
</style>

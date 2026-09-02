<script setup lang="ts">
import { UAlert, UButton, UDashboardGroup } from "#components";
import { computed, nextTick, onMounted, onUpdated, ref, watch } from "vue";
import OmnichannelInboxLoading from "./OmnichannelInboxLoading.vue";
import OmnichannelCRMProfilePanel from "./OmnichannelCRMProfilePanel.vue";
import InboxChatPanel from "./inbox/InboxChatPanel.vue";
import InboxConversationsSidebar from "./inbox/InboxConversationsSidebar.vue";
import InboxDetailsSidebar from "./inbox/InboxDetailsSidebar.vue";
import InboxSaveContactModal from "./inbox/InboxSaveContactModal.vue";
import OmnichannelWhatsAppSessionModal from "./inbox/OmnichannelWhatsAppSessionModal.vue";
import OmnichannelWorkspaceHeader from "./OmnichannelWorkspaceHeader.vue";
import { useOmnichannelInbox } from "~/composables/omnichannel/useOmnichannelInbox";
import {
  buildCanonicalContactPhone,
  normalizePhoneDigits
} from "~/composables/omnichannel/useOmnichannelInboxShared";
import { useOmnichannelCRM, type CRMContactMergeResult, type CRMContactPatch } from "~/composables/omnichannel/useOmnichannelCRM";
import { useOmnichannelHandoff } from "~/composables/omnichannel/useOmnichannelHandoff";
import { useSessionSimulationStore } from "~/stores/session-simulation";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";
import { useAuthStore } from "~/stores/auth";
import { useUiStore } from "~/stores/ui";
import {
  CONVERSATION_PRIVACY_PERMISSION,
  fetchContactAIRestriction,
  fetchHiddenOmnichannelContacts,
  hideOmnichannelContact,
  restoreOmnichannelContact,
  updateContactAIRestriction,
  type ContactAIRestriction,
  type ContactAIRestrictionInput,
  type HiddenOmnichannelContact
} from "~/domain/omnichannel/privacy-api";

const emit = defineEmits<{
  (event: "configure"): void;
}>();

const sessionSimulation = useSessionSimulationStore();
const auth = useAuthStore();
const ui = useUiStore();
const runtimeConfig = useRuntimeConfig();
const privacyApi = createApiRequest(runtimeConfig, () => auth.accessToken);
const hidingContact = ref(false);
const aiRestriction = ref<ContactAIRestriction | null>(null);
const loadingAIRestriction = ref(false);
const updatingAIRestriction = ref(false);
const aiRestrictionError = ref("");
const hiddenContacts = ref<HiddenOmnichannelContact[]>([]);
const loadingHiddenContacts = ref(false);
const hiddenContactsError = ref("");
const restoringHiddenContactIds = ref<string[]>([]);
const canManagePrivacy = computed(() =>
  auth.effectivePermissionKeys.includes(CONVERSATION_PRIVACY_PERMISSION)
);

async function loadHiddenContacts() {
  if (!canManagePrivacy.value || loadingHiddenContacts.value) {
    return;
  }
  loadingHiddenContacts.value = true;
  hiddenContactsError.value = "";
  try {
    hiddenContacts.value = await fetchHiddenOmnichannelContacts(privacyApi);
  } catch (cause) {
    hiddenContactsError.value = getApiErrorMessage(
      cause,
      "Não foi possível carregar as pessoas ocultas."
    );
  } finally {
    loadingHiddenContacts.value = false;
  }
}

async function handleRestoreHiddenContact(item: HiddenOmnichannelContact) {
  if (restoringHiddenContactIds.value.includes(item.contactId)) {
    return;
  }
  restoringHiddenContactIds.value = [...restoringHiddenContactIds.value, item.contactId];
  try {
    await restoreOmnichannelContact(privacyApi, item.contactId);
    hiddenContacts.value = hiddenContacts.value.filter(
      (entry) => entry.contactId !== item.contactId
    );
    await retryConversations();
    ui.success(`${item.contactName || "Contato"} voltou a aparecer nas conversas.`);
  } catch (cause) {
    hiddenContactsError.value = getApiErrorMessage(
      cause,
      "Não foi possível restaurar esta pessoa."
    );
  } finally {
    restoringHiddenContactIds.value = restoringHiddenContactIds.value.filter(
      (contactId) => contactId !== item.contactId
    );
  }
}
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
  loadingMoreConversations,
  loadingContacts,
  loadingUsers,
  loadingWhatsAppStatus,
  pageBootstrapping,
  realtimeConnectionState,
  whatsappConnectionState,
  loadingMessages,
  loadingOlderMessages,
  loadingGroupParticipants,
  conversationsError,
  messagesError,
  hasMoreConversations,
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
  updatingHandoff,
  stickyDateLabel,
  showStickyDate,
  draft,
  aiReplyDraft,
  loadingAIReplyDraft,
  dismissingAIReplyDraft,
  aiReplyDraftError,
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
  loadMoreConversations,
  retryConversations,
  retryActiveConversationMessages,
  reconcileAuthoritativeMessage,
  setReplyTarget,
  clearReplyTarget,
  updateDraft,
  useAIReplyDraft,
  dismissAIReplyDraft,
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
  takeConversation,
  releaseConversation,
  openMentionConversation,
  switchTenant,
  switchingTenant,
  switchTenantError
} = useOmnichannelInbox();

const {
  contacts: crmContacts,
  loading: loadingCRMContacts,
  loadingMore: loadingMoreCRMContacts,
  ready: crmContactsReady,
  error: crmError,
  hasMore: hasMoreCRMContacts,
  nextCursor: nextCRMContactCursor,
  loadContacts: loadCRMContacts,
  profile: crmProfile,
  loadingProfile: loadingCRMProfile,
  loadProfile: loadCRMProfile,
  clearProfile: clearCRMProfile,
  updateContact: updateCRMContact,
  createNote: createCRMNote,
  mergeContacts: mergeCRMContacts,
  undoMerge: undoCRMContactMerge
} = useOmnichannelCRM();

const crmProfileOpen = computed(() => Boolean(crmProfile.value));
const crmActionError = ref("");
const crmSaving = ref(false);
const crmLastMerge = ref<CRMContactMergeResult | null>(null);
const crmMergeCandidates = computed(() =>
  crmContacts.value.filter((entry) => entry.id !== crmProfile.value?.contact.id && !entry.mergedIntoContactId)
);

const {
  queues: handoffQueues,
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
} = useOmnichannelHandoff();

const handoffQueueItems = computed(() =>
  handoffQueues.value.map((queue) => ({
    label: queue.name,
    value: queue.id
  }))
);

watch(
  activeConversationId,
  (conversationId) => {
    void loadConversationOperations(conversationId);
  },
  { immediate: true }
);

watch(
  [activeConversationId, canManagePrivacy, isGroupConversation],
  async ([conversationId, canManage, isGroup]) => {
    aiRestriction.value = null;
    aiRestrictionError.value = "";
    loadingAIRestriction.value = false;
    if (!conversationId || !canManage || isGroup) {
      return;
    }
    const requestedConversationId = conversationId;
    loadingAIRestriction.value = true;
    try {
      const result = await fetchContactAIRestriction(privacyApi, conversationId);
      if (activeConversationId.value === requestedConversationId) {
        aiRestriction.value = result;
      }
    } catch (cause) {
      if (activeConversationId.value === requestedConversationId) {
        aiRestrictionError.value = getApiErrorMessage(
          cause,
          "Não foi possível consultar o bloqueio da IA."
        );
      }
    } finally {
      if (activeConversationId.value === requestedConversationId) {
        loadingAIRestriction.value = false;
      }
    }
  },
  { immediate: true }
);

watch(
  canManageConversation,
  (canManage) => {
    if (canManage) {
      void loadQueues();
    }
  },
  { immediate: true }
);

async function handleTransferQueue(queueId: string) {
  if (!activeConversationId.value || !queueId) {
    return;
  }

  const updated = await transferConversation(activeConversationId.value, queueId);
  if (!updated) {
    return;
  }

  const current = conversations.value.find((entry) => entry.id === updated.id);
  if (current) {
    Object.assign(current, updated);
  }
}

async function handleInspectCRMContact(contactId: string) {
  crmActionError.value = "";
  await loadCRMProfile(contactId);
}

function getCRMActionError(error: unknown, fallback: string) {
  return getApiErrorMessage(error, fallback);
}

async function handleSaveCRMContact(patch: CRMContactPatch) {
  const contactId = crmProfile.value?.contact.id;
  if (!contactId) return;
  crmActionError.value = "";
  crmSaving.value = true;
  try {
    await updateCRMContact(contactId, patch);
    await loadCRMProfile(contactId);
    await loadCRMContacts({ search: search.value, channel: channel.value, status: status.value });
  } catch (error) {
    crmActionError.value = getCRMActionError(error, "Não foi possível salvar o contato.");
  } finally {
    crmSaving.value = false;
  }
}

async function handleCreateCRMNote(content: string) {
  const contactId = crmProfile.value?.contact.id;
  if (!contactId) return;
  crmActionError.value = "";
  crmSaving.value = true;
  try {
    await createCRMNote(contactId, content);
    await loadCRMProfile(contactId);
  } catch (error) {
    crmActionError.value = getCRMActionError(error, "Não foi possível adicionar a nota.");
  } finally {
    crmSaving.value = false;
  }
}

async function handleMergeCRMContact(payload: { targetId: string; reason: string }) {
  const sourceId = crmProfile.value?.contact.id;
  if (!sourceId) return;
  crmActionError.value = "";
  crmSaving.value = true;
  try {
    const result = await mergeCRMContacts(
      sourceId,
      payload.targetId,
      payload.reason,
      `crm-merge:${sourceId}:${payload.targetId}:${Date.now()}`
    );
    crmLastMerge.value = result;
    clearCRMProfile();
    await loadCRMContacts({ search: search.value, channel: channel.value, status: status.value });
  } catch (error) {
    crmActionError.value = getCRMActionError(error, "Não foi possível mesclar os contatos.");
  } finally {
    crmSaving.value = false;
  }
}

async function handleUndoCRMContactMerge() {
  const eventId = crmLastMerge.value?.eventId;
  if (!eventId) return;
  crmActionError.value = "";
  crmSaving.value = true;
  try {
    await undoCRMContactMerge(eventId);
    crmLastMerge.value = null;
    await loadCRMContacts({ search: search.value, channel: channel.value, status: status.value });
  } catch (error) {
    crmActionError.value = getCRMActionError(error, "NÃ£o foi possÃ­vel desfazer a mesclagem.");
  } finally {
    crmSaving.value = false;
  }
}

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

const showWhatsAppConnectionAlert = computed(
  () => !pageBootstrapping.value && !loadingWhatsAppStatus.value
);

const whatsappConnectionAlertTitle = computed(() => {
  if (isWhatsAppConnected.value) {
    return "WhatsApp conectado";
  }

  if (!isWhatsAppConfigured.value) {
    return "Nenhum WhatsApp configurado para este cliente";
  }

  if (whatsappConnectionState.value === "connecting") {
    return "WhatsApp aguardando conexao (leitura do QR Code)";
  }

  return "WhatsApp desconectado";
});

const whatsappConnectionAlertColor = computed(() => {
  if (isWhatsAppConnected.value) {
    return "success";
  }

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
  void loadCRMContacts({ search: search.value, channel: channel.value, status: status.value });
  if (canManagePrivacy.value) {
    void loadHiddenContacts();
  }
});

watch([sidebarView, search, channel, status, canManagePrivacy], ([view]) => {
  if (import.meta.client && view === "contacts") {
    void loadCRMContacts({ search: search.value, channel: channel.value, status: status.value });
  }
  if (import.meta.client && view === "hidden") {
    void loadHiddenContacts();
  }
});

async function handleLoadMoreCRMContacts() {
  if (!hasMoreCRMContacts.value || loadingMoreCRMContacts.value || !nextCRMContactCursor.value) {
    return;
  }
  await loadCRMContacts({
    search: search.value,
    channel: channel.value,
    status: status.value,
    before: nextCRMContactCursor.value,
    append: true
  });
}

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

  crmLastMerge.value = null;
  clearCRMProfile();
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

async function handleHideActiveContact() {
  const conversation = activeConversation.value;
  if (!conversation?.id || !conversation.contactId || hidingContact.value) {
    return;
  }
  if (!window.confirm(`Ocultar ${activeConversationLabel.value || "esta pessoa"} do atendimento?`)) {
    return;
  }
  const clearHistory = window.confirm(
    "Também limpar o histórico anterior? OK limpa o histórico; Cancelar preserva as mensagens para uma futura restauração."
  );
  hidingContact.value = true;
  try {
    await hideOmnichannelContact(privacyApi, conversation.id, clearHistory);
    activeConversationId.value = null;
    await Promise.all([retryConversations(), loadHiddenContacts()]);
    ui.success(
      clearHistory
        ? "Pessoa ocultada e histórico anterior limpo."
        : "Pessoa ocultada com o histórico preservado."
    );
  } catch (cause) {
    ui.error(getApiErrorMessage(cause, "Não foi possível ocultar esta pessoa."));
  } finally {
    hidingContact.value = false;
  }
}

async function handleUpdateAIRestriction(input: ContactAIRestrictionInput) {
  const conversationId = activeConversationId.value;
  if (!conversationId || updatingAIRestriction.value) {
    return;
  }
  updatingAIRestriction.value = true;
  aiRestrictionError.value = "";
  try {
    aiRestriction.value = await updateContactAIRestriction(privacyApi, conversationId, input);
    ui.success(
      input.mode === "allow"
        ? "Atendimento por IA liberado para este contato."
        : input.mode === "indefinite"
          ? "Contato bloqueado para atendimento por IA por tempo indeterminado."
          : "Bloqueio temporário da IA atualizado."
    );
  } catch (cause) {
    aiRestrictionError.value = getApiErrorMessage(
      cause,
      "Não foi possível atualizar o bloqueio da IA."
    );
  } finally {
    updatingAIRestriction.value = false;
  }
}

</script>
<template>
  <div class="chat-page">
    <OmnichannelWhatsAppSessionModal
      v-model:open="whatsappSessionModalOpen"
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

    <OmnichannelCRMProfilePanel
      v-if="crmProfileOpen"
      :profile="crmProfile"
      :loading="loadingCRMProfile"
      :saving="crmSaving"
      :action-error="crmActionError"
      :merge-candidates="crmMergeCandidates"
      @close="clearCRMProfile"
      @open-conversation="openContactConversation"
      @save-contact="handleSaveCRMContact"
      @create-note="handleCreateCRMNote"
      @merge-contact="handleMergeCRMContact"
    />

    <UAlert
      v-if="crmLastMerge"
      class="chat-page__status-alert"
      color="warning"
      variant="soft"
      title="Contatos mesclados e auditados"
      description="A operação pode ser desfeita enquanto o snapshot continuar reversível."
    >
      <template #actions>
        <UButton size="xs" color="warning" :loading="crmSaving" @click="handleUndoCRMContactMerge">
          Desfazer mesclagem
        </UButton>
      </template>
    </UAlert>

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

    <OmnichannelWorkspaceHeader
      mode="inbox"
      :visible="true"
      :title="whatsappConnectionAlertTitle"
      :description="
        isWhatsAppConnected
          ? 'Canal pronto para atendimento.'
          : isWhatsAppConfigured
            ? 'Gere o QR Code e pareie o WhatsApp.'
            : 'Crie a primeira conexão deste cliente.'
      "
      :color="whatsappConnectionAlertColor"
      :connected="isWhatsAppConnected"
      :configured="isWhatsAppConfigured"
      :can-connect="user?.role === 'ADMIN'"
      @connect="handleOpenWhatsAppSessionModal"
      @configure="emit('configure')"
    />

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
        :loading-more-conversations="loadingMoreConversations"
        :loading-contacts="loadingContacts"
        :loading-whats-app-status="loadingWhatsAppStatus"
        :whatsapp-banner-message="sidebarWhatsappBannerMessage"
        :is-whats-app-connected="isWhatsAppConnected"
        :current-user-name="user?.name ?? null"
        :conversations="filteredConversations"
        :conversations-error="conversationsError"
        :has-more-conversations="hasMoreConversations"
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
        :crm-contacts="crmContactsReady ? crmContacts : null"
        :loading-crm-contacts="loadingCRMContacts"
        :crm-error="crmContactsReady ? crmError : ''"
        :loading-more-crm-contacts="loadingMoreCRMContacts"
        :has-more-crm-contacts="hasMoreCRMContacts"
        :can-manage-privacy="canManagePrivacy"
        :hidden-contacts="hiddenContacts"
        :loading-hidden-contacts="loadingHiddenContacts"
        :hidden-contacts-error="hiddenContactsError"
        :restoring-hidden-contact-ids="restoringHiddenContactIds"
        @update:collapsed="updateLeftCollapsed"
        @update:show-filters="updateShowFilters"
        @update:sidebar-view="updateSidebarView"
        @update:search="updateSearch"
        @update:channel="updateChannel"
        @update:status="updateStatus"
        @update:instance-id="updateInstanceId"
        @select-conversation="selectConversation"
        @retry-conversations="retryConversations"
        @load-more-conversations="loadMoreConversations"
        @open-contact="openContactConversation"
        @inspect-contact="handleInspectCRMContact"
        @load-more-crm-contacts="handleLoadMoreCRMContacts"
        @create-contact="createContactAndOpenConversation"
        @preview-contact-import="previewWhatsAppContactsImport"
        @apply-contact-import="applyWhatsAppContactsImport"
        @clear-contact-import="clearWhatsAppContactsImportPreview"
        @switch-tenant="handleSidebarTenantSwitch"
        @refresh-hidden-contacts="loadHiddenContacts"
        @restore-hidden-contact="handleRestoreHiddenContact"
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
        :updating-handoff="updatingHandoff"
        :loading-messages="loadingMessages"
        :messages-error="messagesError"
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
        :ai-reply-draft="aiReplyDraft"
        :loading-ai-reply-draft="loadingAIReplyDraft"
        :dismissing-ai-reply-draft="dismissingAIReplyDraft"
        :ai-reply-draft-error="aiReplyDraftError"
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
        :load-older-messages-action="requestOlderMessages"
        :retry-messages-action="retryActiveConversationMessages"
        :reconcile-message-action="reconcileAuthoritativeMessage"
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
        @take-conversation="takeConversation"
        @release-conversation="releaseConversation"
        @open-whatsapp-session="handleOpenWhatsAppSessionModal"
        @set-reply="setReplyTarget"
        @clear-reply="clearReplyTarget"
        @update:draft="updateDraft"
        @use-ai-reply-draft="useAIReplyDraft"
        @dismiss-ai-reply-draft="dismissAIReplyDraft()"
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
        :handoff-items="handoffs"
        :sla-events="slaEvents"
        :queue-items="handoffQueueItems"
        :loading-handoff="loadingConversationOps"
        :loading-queues="loadingQueues"
        :transferring-queue="transferringQueue"
        :handoff-error="handoffError"
        :queue-error="queueError"
        :can-manage-privacy="canManagePrivacy"
        :hiding-contact="hidingContact"
        :ai-restriction="aiRestriction"
        :loading-ai-restriction="loadingAIRestriction"
        :updating-ai-restriction="updatingAIRestriction"
        :ai-restriction-error="aiRestrictionError"
        @update:collapsed="updateRightCollapsed"
        @update:internal-notes="updateInternalNotes"
        @update:assignee-model="updateAssigneeModel"
        @save-contact="saveActiveConversationContact"
        @update-status="updateConversationStatus"
        @update-assignee="updateConversationAssignee"
        @transfer-queue="handleTransferQueue"
        @hide-contact="handleHideActiveContact"
        @update-ai-restriction="handleUpdateAIRestriction"
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

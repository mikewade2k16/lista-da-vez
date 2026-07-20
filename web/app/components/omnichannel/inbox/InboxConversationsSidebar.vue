<script setup lang="ts">
import {
  UAvatar,
  UBadge,
  UButton,
  UDashboardSidebar,
  UDashboardSidebarCollapse,
  UFormField,
  UInput,
  USelect
} from "#components";
import { computed, nextTick, onMounted, onUpdated, ref } from "vue";
import type { Contact, Conversation, MessageType, WhatsAppInstanceRecord } from "~/types";
import type { InboxSelectOption } from "./types";
import { resolveAvatarSource } from "~/composables/omnichannel/useAvatarProxy";
import type {
  WhatsAppContactImportAction,
  WhatsAppContactImportResponse
} from "~/composables/omnichannel/useOmnichannelInboxShared";

const props = defineProps<{
  collapsed: boolean;
  showFilters: boolean;
  sidebarView: "conversations" | "contacts";
  loadingConversations: boolean;
  loadingContacts: boolean;
  loadingWhatsAppStatus: boolean;
  whatsappBannerMessage: string;
  isWhatsAppConnected: boolean;
  currentUserName: string | null;
  conversations: Conversation[];
  contacts: Contact[];
  activeConversationId: string | null;
  creatingContact: boolean;
  importingContacts: boolean;
  contactActionError: string;
  contactImportPreview: WhatsAppContactImportResponse | null;
  contactImportResult: WhatsAppContactImportResponse | null;
  unreadConversationIds: string[];
  mentionConversationIds: string[];
  mentionConversationCounts: Record<string, number>;
  search: string;
  channel: string;
  status: string;
  instanceId: string;
  channelFilterItems: InboxSelectOption[];
  statusFilterItems: InboxSelectOption[];
  instanceFilterItems: InboxSelectOption[];
  availableInstances: WhatsAppInstanceRecord[];
  canSwitchTenant: boolean;
  switchingTenant: boolean;
  activeTenantId: string;
  tenantSwitchItems: InboxSelectOption[];
}>();

const emit = defineEmits<{
  (event: "update:collapsed", value: boolean): void;
  (event: "update:showFilters", value: boolean): void;
  (event: "update:sidebarView", value: "conversations" | "contacts"): void;
  (event: "update:search", value: string): void;
  (event: "update:channel", value: string): void;
  (event: "update:status", value: string): void;
  (event: "update:instanceId", value: string): void;
  (event: "switch-tenant", value: string): void;
  (event: "selectConversation", conversationId: string): void;
  (event: "openContact", contactId: string): void;
  (event: "createContact", payload: { name: string; phone: string; countryCode?: string | null }): void;
  (event: "previewContactImport"): void;
  (event: "applyContactImport"): void;
  (event: "clearContactImport"): void;
}>();

const collapsedModel = computed({
  get: () => props.collapsed,
  set: (value: boolean) => emit("update:collapsed", value)
});

const showFiltersModel = computed({
  get: () => props.showFilters,
  set: (value: boolean) => emit("update:showFilters", value)
});

const sidebarViewModel = computed({
  get: () => props.sidebarView,
  set: (value: "conversations" | "contacts") => emit("update:sidebarView", value)
});

const searchModel = computed({
  get: () => props.search,
  set: (value: string) => emit("update:search", value)
});

const channelModel = computed({
  get: () => props.channel,
  set: (value: string) => emit("update:channel", value)
});

const statusModel = computed({
  get: () => props.status,
  set: (value: string) => emit("update:status", value)
});

const instanceModel = computed({
  get: () => props.instanceId,
  set: (value: string) => emit("update:instanceId", value)
});

const tenantModel = computed({
  get: () => props.activeTenantId,
  set: (value: string) => emit("switch-tenant", value)
});

const unreadSet = computed(() => new Set(props.unreadConversationIds));
const mentionSet = computed(() => new Set(props.mentionConversationIds));
const mentionCountMap = computed(() => props.mentionConversationCounts ?? {});
const isContactsView = computed(() => sidebarViewModel.value === "contacts");
const MEDIA_PLACEHOLDER_VALUES = new Set(["[imagem]", "[audio]", "[video]", "[documento]", "."]);
const newContactName = ref("");
const newContactPhone = ref("");
const newContactInternational = ref(false);
const newContactCountryCode = ref("");

function normalizeNameForComparison(value: string | null | undefined) {
  return (
    value
      ?.normalize("NFKD")
      .replace(/[^\w\s]/g, "")
      .trim()
      .toLowerCase() ?? ""
  );
}

function isWeakDisplayName(value: string | null | undefined) {
  const normalized = value?.trim();
  if (!normalized) {
    return true;
  }

  if (
    normalized.endsWith("@s.whatsapp.net") ||
    normalized.endsWith("@g.us") ||
    normalized.endsWith("@lid")
  ) {
    return true;
  }

  const digits = normalized.replace(/\D/g, "");
  return digits.length >= 7 && !/[a-zA-Z\u00C0-\u024F]/.test(normalized);
}

function isLikelyOperatorName(value: string | null | undefined) {
  const normalized = normalizeNameForComparison(value);
  if (!normalized) {
    return false;
  }

  const currentUser = normalizeNameForComparison(props.currentUserName);
  if (currentUser && normalized === currentUser) {
    return true;
  }

  return ["voce", "atendente", "agent", "admin demo", "operador"].includes(normalized);
}

function isGroupConversation(conversationEntry: Conversation) {
  return conversationEntry.externalId.endsWith("@g.us");
}

function extractExternalPhone(conversationEntry: Conversation) {
  const fromContact = conversationEntry.contactPhone?.trim() ?? "";
  if (fromContact) {
    return fromContact;
  }

  const digits = conversationEntry.externalId.replace(/\D/g, "");
  return digits || "";
}

function sanitizeConversationDisplayName(
  value: string | null | undefined,
  options: { fallbackPhone?: string | null; fallbackLabel?: string } = {}
) {
  const fallbackLabel = options.fallbackLabel ?? "Contato";
  const trimmed = value?.trim() ?? "";
  if (trimmed && !isWeakDisplayName(trimmed)) {
    return trimmed;
  }

  const fallbackPhone = (options.fallbackPhone ?? "").replace(/\D/g, "");
  if (fallbackPhone) {
    return fallbackPhone;
  }

  return fallbackLabel;
}

function buildFallbackGroupName(conversationEntry: Conversation) {
  const digits = conversationEntry.externalId.replace(/\D/g, "");
  if (!digits) {
    return "Grupo";
  }

  return `Grupo ${digits.slice(-4)}`;
}

function getConversationName(conversationEntry: Conversation) {
  const contactName = conversationEntry.contactName?.trim() ?? "";
  const fallbackPhone = extractExternalPhone(conversationEntry);
  if (isGroupConversation(conversationEntry)) {
    if (contactName && !isWeakDisplayName(contactName)) {
      return sanitizeConversationDisplayName(contactName, {
        fallbackPhone,
        fallbackLabel: "Grupo"
      });
    }

    return buildFallbackGroupName(conversationEntry);
  }

  if (contactName && !isLikelyOperatorName(contactName)) {
    return sanitizeConversationDisplayName(contactName, {
      fallbackPhone
    });
  }

  return fallbackPhone || "Contato";
}

function getConversationAvatar(conversationEntry: Conversation) {
  const fromConversation = resolveAvatarSource(conversationEntry.contactAvatarUrl?.trim()) ?? "";
  if (fromConversation) {
    return fromConversation;
  }

  // Group card must use only the group's own avatar.
  if (isGroupConversation(conversationEntry)) {
    return "";
  }

  if (conversationEntry.contactId) {
    const byId = props.contacts.find((contactEntry) => contactEntry.id === conversationEntry.contactId);
    const avatarById = resolveAvatarSource(byId?.avatarUrl?.trim()) ?? "";
    if (avatarById) {
      return avatarById;
    }
  }

  const conversationPhone = extractExternalPhone(conversationEntry).replace(/\D/g, "");
  if (conversationPhone) {
    const byPhone = props.contacts.find((contactEntry) => {
      const contactPhone = contactEntry.phone.replace(/\D/g, "");
      if (!contactPhone) {
        return false;
      }

      return contactPhone.endsWith(conversationPhone) || conversationPhone.endsWith(contactPhone);
    });
    const avatarByPhone = resolveAvatarSource(byPhone?.avatarUrl?.trim()) ?? "";
    if (avatarByPhone) {
      return avatarByPhone;
    }
  }

  return "";
}

function getConversationInstanceLabel(conversationEntry: Conversation) {
  return conversationEntry.instanceDisplayName?.trim() || conversationEntry.instanceName?.trim() || "Instancia";
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

function formatTime(value: string) {
  return new Date(value).toLocaleTimeString("pt-BR", {
    hour: "2-digit",
    minute: "2-digit"
  });
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

function isConversationUnread(conversationId: string) {
  return unreadSet.value.has(conversationId);
}

function hasMentionAlert(conversationId: string) {
  return mentionSet.value.has(conversationId);
}

function getMentionCount(conversationId: string) {
  const rawCount = mentionCountMap.value?.[conversationId] ?? 0;
  if (!Number.isFinite(rawCount)) {
    return 0;
  }

  return Math.max(0, Math.trunc(rawCount));
}

function getMessageTypeLabel(type: MessageType | null | undefined) {
  if (type === "IMAGE") {
    return "Foto";
  }

  if (type === "AUDIO") {
    return "Audio";
  }

  if (type === "VIDEO") {
    return "Video";
  }

  if (type === "DOCUMENT") {
    return "Documento";
  }

  return "Mensagem";
}

function isMediaPlaceholder(value: string | null | undefined) {
  const normalized = value?.trim().toLowerCase();
  if (!normalized) {
    return false;
  }

  return MEDIA_PLACEHOLDER_VALUES.has(normalized);
}

function getConversationPreview(conversationEntry: Conversation) {
  const lastMessage = conversationEntry.lastMessage;
  if (!lastMessage) {
    return "Sem mensagens ainda.";
  }

  const type = lastMessage.messageType ?? "TEXT";
  const content = lastMessage.content?.trim() ?? "";

  if (type === "TEXT") {
    return content || "Sem mensagens ainda.";
  }

  if (content && !isMediaPlaceholder(content)) {
    return content;
  }

  return `[${getMessageTypeLabel(type)}]`;
}

function getContactName(contactEntry: Contact) {
  const trimmed = contactEntry.name?.trim();
  if (trimmed) {
    return trimmed;
  }

  return formatContactDisplayPhone(contactEntry.phone);
}

function getContactPreview(contactEntry: Contact) {
  if (contactEntry.lastConversationAt) {
    return `Ultima atividade ${formatTime(contactEntry.lastConversationAt)}`;
  }

  return formatContactDisplayPhone(contactEntry.phone);
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

function getContactStatusLabel(contactEntry: Contact) {
  if (contactEntry.lastConversationStatus === "OPEN") {
    return "Aberto";
  }

  if (contactEntry.lastConversationStatus === "PENDING") {
    return "Pendente";
  }

  if (contactEntry.lastConversationStatus === "CLOSED") {
    return "Encerrado";
  }

  return "Sem conversa";
}

function getContactStatusColor(contactEntry: Contact) {
  if (contactEntry.lastConversationStatus === "OPEN") {
    return "success";
  }

  if (contactEntry.lastConversationStatus === "PENDING") {
    return "warning";
  }

  if (contactEntry.lastConversationStatus === "CLOSED") {
    return "neutral";
  }

  return "neutral";
}

function getImportActionLabel(action: WhatsAppContactImportAction) {
  if (action === "create") {
    return "Novo";
  }

  if (action === "update") {
    return "Atualizar";
  }

  return "Ignorar";
}

function getImportActionColor(action: WhatsAppContactImportAction) {
  if (action === "create") {
    return "success";
  }

  if (action === "update") {
    return "warning";
  }

  return "neutral";
}

function onCreateContact() {
  const phone = newContactPhone.value.trim();
  if (!phone) {
    return;
  }

  emit("createContact", {
    name: newContactName.value.trim(),
    phone,
    countryCode: newContactInternational.value ? newContactCountryCode.value.trim() : null
  });
}

function stripSidebarMinHeightClass() {
  if (!import.meta.client) {
    return;
  }

  document
    .querySelectorAll<HTMLElement>(".chat-page__sidebar--left.min-h-svh")
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
    id="omni-inbox-left"
    v-model:collapsed="collapsedModel"
    side="left"
    resizable
    collapsible
    :min-size="18"
    :default-size="24"
    :max-size="36"
    :collapsed-size="0"
    :ui="{
      root: 'chat-page__sidebar chat-page__sidebar--left !min-h-0 !h-full',
      header: 'chat-page__panel-header chat-page__panel-header--sidebar',
      body: 'chat-page__sidebar-body'
    }"
  >
    <template #header>
      <div class="chat-page__header-row">
        <div v-if="!collapsedModel" class="chat-page__header-tabs">
          <UButton
            size="sm"
            color="neutral"
            :variant="!isContactsView ? 'soft' : 'ghost'"
            @click="sidebarViewModel = 'conversations'"
          >
            Conversas
          </UButton>
          <UButton
            size="sm"
            color="neutral"
            :variant="isContactsView ? 'soft' : 'ghost'"
            @click="sidebarViewModel = 'contacts'"
          >
            Contatos
          </UButton>
            <UButton v-if="!collapsedModel && !isContactsView" size="sm" color="neutral" variant="ghost" @click="showFiltersModel = !showFiltersModel">
            Filtros
          </UButton>
          <div
            v-if="!collapsedModel && !isContactsView && canSwitchTenant"
            class="chat-page__tenant-switch-control"
          >
            <USelect
              v-model="tenantModel"
              :items="tenantSwitchItems"
              value-key="value"
              placeholder="Atendimento"
              :loading="switchingTenant"
            />
          </div>
          <UButton v-if="!collapsedModel && isContactsView" size="sm" color="neutral" variant="ghost" @click="showFiltersModel = !showFiltersModel">
            Novo
          </UButton>
          <UButton
            v-if="!collapsedModel && isContactsView"
            size="sm"
            color="neutral"
            variant="ghost"
            :loading="importingContacts"
            :disabled="importingContacts"
            @click="emit('previewContactImport')"
          >
            Importar WA
          </UButton>
          <UDashboardSidebarCollapse side="left" color="neutral" variant="ghost" />
        </div>
        <div class="chat-page__header-actions">
        
        </div>
      </div>
    </template>

    <div class="chat-page__sidebar-content">
      <div
        v-if="!collapsedModel && !loadingWhatsAppStatus && whatsappBannerMessage"
        class="chat-page__channel-banner"
        :class="{
          'chat-page__channel-banner--connected': isWhatsAppConnected,
          'chat-page__channel-banner--disconnected': !isWhatsAppConnected
        }"
      >
        {{ whatsappBannerMessage }}
      </div>

      <div v-if="!collapsedModel" class="chat-page__filters">
        <UInput
          v-model="searchModel"
          icon="i-lucide-search"
          placeholder="Buscar por nome, telefone ou mensagem"
        />

        <div v-if="!isContactsView && showFiltersModel" class="chat-page__filters-grid">
          <USelect
            v-model="channelModel"
            :items="channelFilterItems"
            value-key="value"
            placeholder="Canal"
          />

          <USelect
            v-model="statusModel"
            :items="statusFilterItems"
            value-key="value"
            placeholder="Status"
          />

          <USelect
            v-if="availableInstances.length > 1"
            v-model="instanceModel"
            :items="instanceFilterItems"
            value-key="value"
            placeholder="Instancia"
          />
        </div>

        <div v-else-if="isContactsView && showFiltersModel" class="chat-page__contact-form">
          <UFormField label="Nome do contato" name="contactName">
            <UInput v-model="newContactName" placeholder="Nome para exibir" />
          </UFormField>

          <div class="chat-page__contact-mode">
            <UButton
              size="xs"
              color="neutral"
              :variant="newContactInternational ? 'soft' : 'ghost'"
              @click="newContactInternational = !newContactInternational"
            >
              {{ newContactInternational ? 'Numero internacional' : 'Brasil (sem 55)' }}
            </UButton>
          </div>

          <UFormField v-if="newContactInternational" label="Codigo do pais (DDI)" name="contactCountryCode">
            <UInput v-model="newContactCountryCode" placeholder="1" />
          </UFormField>

          <UFormField :label="newContactInternational ? 'Telefone' : 'Telefone (Brasil)'" name="contactPhone">
            <UInput v-model="newContactPhone" :placeholder="newContactInternational ? '2025550182' : '11999999999'" />
          </UFormField>

          <p class="chat-page__contact-hint">
            {{ newContactInternational ? 'Digite o DDI apenas quando o numero nao for do Brasil.' : 'Brasil e o padrao. Nao precisa informar o codigo 55.' }}
          </p>

          <UButton
            size="sm"
            color="primary"
            variant="soft"
            :loading="creatingContact"
            :disabled="creatingContact"
            @click="onCreateContact"
          >
            Salvar e abrir
          </UButton>
        </div>

        <p v-if="isContactsView && contactActionError" class="chat-page__contact-error">
          {{ contactActionError }}
        </p>

        <div v-if="isContactsView && contactImportPreview" class="chat-page__import-preview">
          <p class="chat-page__import-title">Preview da importacao WhatsApp</p>
          <p class="chat-page__import-summary">
            {{ contactImportPreview.summary.create }} novos, {{ contactImportPreview.summary.update }} para atualizar e
            {{ contactImportPreview.summary.skip }} sem alteracoes.
          </p>

          <div v-if="contactImportPreview.warnings.length" class="chat-page__import-warnings">
            <p v-for="warning in contactImportPreview.warnings" :key="warning" class="chat-page__import-warning">
              {{ warning }}
            </p>
          </div>

          <div class="chat-page__import-actions">
            <UButton
              size="xs"
              color="primary"
              variant="soft"
              :loading="importingContacts"
              :disabled="importingContacts || (!contactImportPreview.summary.create && !contactImportPreview.summary.update)"
              @click="emit('applyContactImport')"
            >
              Aplicar importacao
            </UButton>
            <UButton size="xs" color="neutral" variant="ghost" :disabled="importingContacts" @click="emit('clearContactImport')">
              Limpar
            </UButton>
          </div>

          <ul class="chat-page__import-list">
            <li v-for="entry in contactImportPreview.items.slice(0, 8)" :key="entry.phone" class="chat-page__import-item">
              <div class="chat-page__import-item-main">
                <p class="chat-page__import-item-name">{{ entry.name }}</p>
                <p class="chat-page__import-item-phone">{{ formatContactDisplayPhone(entry.phone) }}</p>
              </div>
              <UBadge :color="getImportActionColor(entry.action)" variant="soft" size="xs">
                {{ getImportActionLabel(entry.action) }}
              </UBadge>
            </li>
          </ul>

          <p v-if="contactImportPreview.items.length > 8" class="chat-page__import-more">
            Mostrando 8 de {{ contactImportPreview.items.length }} contatos no preview.
          </p>
          <p v-if="contactImportResult?.applied" class="chat-page__import-result">
            Importacao concluida: {{ contactImportResult.applied.created }} criados, {{ contactImportResult.applied.updated }} atualizados,
            {{ contactImportResult.applied.skipped }} ignorados, {{ contactImportResult.applied.failed }} falhas.
          </p>
        </div>
      </div>

      <div class="chat-page__panel-body">
        <template v-if="!isContactsView">
          <div v-if="loadingConversations" class="chat-page__empty">Carregando conversas...</div>

          <template v-else>
            <button
              v-for="conversationEntry in conversations"
              :key="conversationEntry.id"
              type="button"
              class="conversation-card"
              :class="{ 'conversation-card--active': conversationEntry.id === activeConversationId }"
              @click="emit('selectConversation', conversationEntry.id)"
            >
              <div class="conversation-card__top">
                <UAvatar
                  :src="getConversationAvatar(conversationEntry) || undefined"
                  :alt="getConversationName(conversationEntry)"
                  :text="getInitials(getConversationName(conversationEntry))"
                  class="conversation-card__avatar"
                />

                <div class="conversation-card__content">
                  <p class="conversation-card__name">{{ getConversationName(conversationEntry) }}</p>

                  <div class="conversation-card__tags">
                    <UBadge color="neutral" variant="soft" size="sm">
                      {{ getChannelLabel(conversationEntry.channel) }}
                    </UBadge>
                    <UBadge
                      v-if="conversationEntry.instanceId"
                      color="primary"
                      variant="soft"
                      size="sm"
                    >
                      {{ getConversationInstanceLabel(conversationEntry) }}
                    </UBadge>
                    <UBadge :color="getStatusColor(conversationEntry.status)" variant="soft" size="sm">
                      {{ getStatusLabel(conversationEntry.status) }}
                    </UBadge>
                    <UBadge v-if="isConversationUnread(conversationEntry.id)" color="warning" variant="soft" size="sm">
                      Novo
                    </UBadge>
                    <UBadge v-if="hasMentionAlert(conversationEntry.id)" color="error" variant="soft" size="sm">
                      @{{ getMentionCount(conversationEntry.id) || 1 }} Mencao
                    </UBadge>
                  </div>
                </div>

                <time class="conversation-card__time">{{ formatTime(conversationEntry.lastMessageAt) }}</time>
              </div>

              <p class="conversation-card__preview">{{ getConversationPreview(conversationEntry) }}</p>
            </button>

            <div v-if="!conversations.length" class="chat-page__empty">Nenhuma conversa encontrada.</div>
          </template>
        </template>

        <template v-else>
          <div v-if="loadingContacts" class="chat-page__empty">Carregando contatos...</div>

          <template v-else>
            <button
              v-for="contactEntry in contacts"
              :key="contactEntry.id"
              type="button"
              class="conversation-card conversation-card--contact"
              @click="emit('openContact', contactEntry.id)"
            >
              <div class="conversation-card__top">
                <UAvatar
                  :src="resolveAvatarSource(contactEntry.avatarUrl) || undefined"
                  :alt="getContactName(contactEntry)"
                  :text="getInitials(getContactName(contactEntry))"
                  class="conversation-card__avatar"
                />

                <div class="conversation-card__content">
                  <p class="conversation-card__name">{{ getContactName(contactEntry) }}</p>

                  <div class="conversation-card__tags">
                    <UBadge color="neutral" variant="soft" size="sm">
                      Contato
                    </UBadge>
                    <UBadge :color="getContactStatusColor(contactEntry)" variant="soft" size="sm">
                      {{ getContactStatusLabel(contactEntry) }}
                    </UBadge>
                  </div>
                </div>

                <time v-if="contactEntry.lastConversationAt" class="conversation-card__time">
                  {{ formatTime(contactEntry.lastConversationAt) }}
                </time>
              </div>

              <p class="conversation-card__preview">{{ getContactPreview(contactEntry) }}</p>
            </button>

            <div v-if="!contacts.length" class="chat-page__empty">Nenhum contato salvo ainda.</div>
          </template>
        </template>
      </div>
    </div>
  </UDashboardSidebar>
</template>

<style scoped>
.chat-page__sidebar {
  min-height: 0;
}
.chat-page__sidebar--left{
  min-height: 0 !important;
}

.chat-page__sidebar-body,
.chat-page__sidebar-content {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  min-width: 0;
}

.chat-page__header-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.chat-page__header-tabs {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.chat-page__header-actions {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex: 1 1 auto;
  justify-content: flex-end;
  flex-wrap: wrap;
}

.chat-page__tenant-switch-control {
  width: min(100%, 13rem);
  min-width: 11rem;
}

.chat-page__filters {
  display: grid;
  gap: 0.75rem;
  padding: 0 0 0.75rem;
}

.chat-page__contact-form {
  display: grid;
  gap: 0.65rem;
}

.chat-page__contact-mode {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.chat-page__contact-hint {
  margin: 0;
  font-size: 0.75rem;
  color: rgb(var(--muted));
  line-height: 1.4;
}

.chat-page__contact-error {
  margin: 0;
  font-size: 0.8rem;
  color: rgb(var(--error));
}

.chat-page__import-preview {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  padding: 0.6rem;
  display: grid;
  gap: 0.5rem;
}

.chat-page__import-title {
  margin: 0;
  font-size: 0.78rem;
  font-weight: 600;
}

.chat-page__import-summary {
  margin: 0;
  font-size: 0.75rem;
  color: rgb(var(--muted));
  line-height: 1.35;
}

.chat-page__import-warnings {
  display: grid;
  gap: 0.25rem;
}

.chat-page__import-warning {
  margin: 0;
  font-size: 0.72rem;
  color: rgb(var(--warning));
}

.chat-page__import-actions {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.chat-page__import-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.35rem;
}

.chat-page__import-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.chat-page__import-item-main {
  min-width: 0;
}

.chat-page__import-item-name {
  margin: 0;
  font-size: 0.74rem;
  font-weight: 600;
  line-height: 1.2;
}

.chat-page__import-item-phone {
  margin: 0;
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.chat-page__import-more,
.chat-page__import-result {
  margin: 0;
  font-size: 0.72rem;
  color: rgb(var(--muted));
}

.chat-page__channel-banner {
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  padding: 0.55rem 0.65rem;
  font-size: 0.8rem;
  font-weight: 500;
  line-height: 1.4;
  margin: 0.5rem 0;
  white-space: normal;
  overflow-wrap: anywhere;
  word-break: break-word;
}

.chat-page__channel-banner--connected {
  color: rgb(var(--success));
  background: rgb(var(--success) / 0.14);
  border-color: rgb(var(--success) / 0.35);
}

.chat-page__channel-banner--disconnected {
  color: rgb(var(--warning));
  background: rgb(var(--warning) / 0.14);
  border-color: rgb(var(--warning) / 0.35);
}

.chat-page__filters-grid {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: nowrap;
}

.chat-page__filters-grid > * {
  flex: 1 1 0;
  min-width: 0;
}

.chat-page__panel-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  display: grid;
  align-content: start;
  gap: 0.5rem;
  padding: 0.5rem;
}

.chat-page__empty {
  color: rgb(var(--muted));
  font-size: 0.85rem;
}

.conversation-card {
  width: 100%;
  border: 1px solid rgb(var(--border));
  border-radius: var(--radius-sm);
  background: rgb(var(--surface));
  color: rgb(var(--text));
  text-align: left;
  padding: 0.65rem;
  display: grid;
  gap: 0.45rem;
  cursor: pointer;
}

.conversation-card--active {
  border-color: rgb(var(--primary));
  box-shadow: 0 0 0 1px rgb(var(--primary) / 0.35);
}

.conversation-card--contact {
  border-style: dashed;
}

.conversation-card__top {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 0.55rem;
  align-items: center;
}

.conversation-card__content {
  min-width: 0;
}

.conversation-card__name {
  margin: 0;
  font-weight: 600;
  font-size: 0.95rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.conversation-card__tags {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.conversation-card__time {
  font-size: 0.74rem;
  color: rgb(var(--muted));
}

.conversation-card__preview {
  margin: 0;
  font-size: 0.82rem;
  color: rgb(var(--muted));
  line-height: 1.3;
  max-height: 2.6em;
  overflow: hidden;
}

@media (max-width: 1280px) {
  .chat-page__filters-grid {
    flex-wrap: wrap;
  }

  .chat-page__filters-grid > * {
    flex-basis: calc(50% - 0.25rem);
  }
}

@media (max-width: 720px) {
  .chat-page__tenant-switch-control {
    width: 100%;
    min-width: 0;
  }

  .chat-page__filters-grid > * {
    flex-basis: 100%;
  }
}
</style>

import { computed, reactive, ref } from "vue";
import type {
  Contact,
  Conversation,
  GroupParticipant,
  Message,
  TenantUser,
  WhatsAppInstanceRecord,
  WhatsAppStatusResponse
} from "~/types";
import {
  DEFAULT_MAX_UPLOAD_MB,
  type InboxSidebarView,
  type OutboundAttachment,
  type ReadStateEntry,
  UNASSIGNED_VALUE
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxState() {
  const conversations = ref<Conversation[]>([]);
  const contacts = ref<Contact[]>([]);
  const users = ref<TenantUser[]>([]);
  const whatsappInstances = ref<WhatsAppInstanceRecord[]>([]);
  const messages = ref<Message[]>([]);
  const visibleMessagesConversationId = ref<string | null>(null);

  const activeConversationId = ref<string | null>(null);
  const leftCollapsed = ref(false);
  const rightCollapsed = ref(false);
  const showFilters = ref(true);
  const sidebarView = ref<InboxSidebarView>("conversations");

  const loadingConversations = ref(false);
  const loadingContacts = ref(false);
  const loadingUsers = ref(false);
  const loadingWhatsAppStatus = ref(false);
  const loadingMessages = ref(false);
  const loadingOlderMessages = ref(false);
  const loadingGroupParticipants = ref(false);
  const savingContact = ref(false);
  const creatingContact = ref(false);
  const pendingSendCount = ref(0);
  const sendingMessage = computed(() => pendingSendCount.value > 0);
  const sendError = ref("");
  const contactActionError = ref("");
  const updatingStatus = ref(false);
  const updatingAssignee = ref(false);

  const hasMoreMessages = ref(false);
  const showLoadOlderMessagesButton = ref(false);
  const showScrollToLatestButton = ref(false);
  const stickyDateLabel = ref("");
  const showStickyDate = ref(false);

  const draft = ref("");
  const search = ref("");
  const channel = ref("all");
  const status = ref("all");
  const instanceId = ref("all");
  const replyTarget = ref<Message | null>(null);
  const attachment = ref<OutboundAttachment | null>(null);
  const tenantMaxUploadMb = ref(DEFAULT_MAX_UPLOAD_MB);
  const assigneeModel = ref<string>(UNASSIGNED_VALUE);
  const whatsappStatus = ref<WhatsAppStatusResponse | null>(null);

  const notesByConversation = reactive<Record<string, string>>({});
  const groupParticipantsByConversation = reactive<Record<string, GroupParticipant[]>>({});
  const readState = ref<Record<string, ReadStateEntry>>({});
  const mentionAlertState = ref<Record<string, number>>({});

  const chatBodyRef = ref<HTMLElement | null>(null);

  return {
    conversations,
    contacts,
    users,
    whatsappInstances,
    messages,
    visibleMessagesConversationId,
    activeConversationId,
    leftCollapsed,
    rightCollapsed,
    showFilters,
    sidebarView,
    loadingConversations,
    loadingContacts,
    loadingUsers,
    loadingWhatsAppStatus,
    loadingMessages,
    loadingOlderMessages,
    loadingGroupParticipants,
    savingContact,
    creatingContact,
    pendingSendCount,
    sendingMessage,
    sendError,
    contactActionError,
    updatingStatus,
    updatingAssignee,
    hasMoreMessages,
    showLoadOlderMessagesButton,
    showScrollToLatestButton,
    stickyDateLabel,
    showStickyDate,
    draft,
    search,
    channel,
    status,
    instanceId,
    replyTarget,
    attachment,
    tenantMaxUploadMb,
    assigneeModel,
    whatsappStatus,
    notesByConversation,
    groupParticipantsByConversation,
    readState,
    mentionAlertState,
    chatBodyRef
  };
}

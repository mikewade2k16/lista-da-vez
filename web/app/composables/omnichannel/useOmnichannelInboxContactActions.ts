import { ref, type ComputedRef, type Ref } from "vue";
import type { Contact, Conversation } from "~/types";
import {
  type CreateContactPayload,
  type InboxSidebarView,
  type SaveContactResponse,
  type WhatsAppContactImportResponse,
  buildCanonicalContactPhone,
  formatSendError
} from "~/composables/omnichannel/useOmnichannelInboxShared";

export function useOmnichannelInboxContactActions(options: {
  activeConversationId: Ref<string | null>;
  canSaveActiveContact: ComputedRef<boolean>;
  savingContact: Ref<boolean>;
  creatingContact: Ref<boolean>;
  contactActionError: Ref<string>;
  sidebarView: Ref<InboxSidebarView>;
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  upsertContact: (contactEntry: Contact) => void;
  upsertConversation: (conversationEntry: Conversation) => void;
  syncSavedContactIntoMessages: (contactEntry: Contact) => void;
  loadContacts: () => Promise<void>;
  loadConversations: () => Promise<void>;
  selectConversation: (conversationId: string) => Promise<void>;
}) {
  const importingContacts = ref(false);
  const contactImportPreview = ref<WhatsAppContactImportResponse | null>(null);
  const contactImportResult = ref<WhatsAppContactImportResponse | null>(null);

  async function saveActiveConversationContact() {
    if (!options.canSaveActiveContact.value || !options.activeConversationId.value) {
      return null;
    }

    options.savingContact.value = true;
    options.contactActionError.value = "";

    try {
      const response = await options.apiFetch<SaveContactResponse>("/contacts", {
        method: "POST",
        body: {
          conversationId: options.activeConversationId.value,
          source: "INBOX"
        }
      });

      if (response.contact) {
        options.upsertContact(response.contact);
        options.syncSavedContactIntoMessages(response.contact);
      }

      if (response.conversation) {
        options.upsertConversation(response.conversation);
      } else {
        await options.loadConversations();
      }

      return response.contact;
    } catch (error) {
      options.contactActionError.value = formatSendError(error, "Nao foi possivel salvar o contato.");
      return null;
    } finally {
      options.savingContact.value = false;
    }
  }

  async function createContactAndOpenConversation(payload: CreateContactPayload) {
    const normalizedName = payload.name.trim();
    const normalizedPhone = buildCanonicalContactPhone({
      phone: payload.phone,
      countryCode: payload.countryCode
    });

    if (!normalizedPhone) {
      options.contactActionError.value = "Informe um telefone valido para criar o contato.";
      return null;
    }

    options.creatingContact.value = true;
    options.contactActionError.value = "";

    try {
      const created = await options.apiFetch<SaveContactResponse>("/contacts", {
        method: "POST",
        body: {
          name: normalizedName || normalizedPhone,
          phone: normalizedPhone,
          source: "MANUAL"
        }
      });

      const contactId = created.contact?.id;
      if (!contactId) {
        throw new Error("Contato nao retornado pela API.");
      }

      if (created.contact) {
        options.upsertContact(created.contact);
        options.syncSavedContactIntoMessages(created.contact);
      }

      const conversation = await options.apiFetch<Conversation>(`/contacts/${contactId}/open-conversation`, {
        method: "POST",
        body: {
          channel: "WHATSAPP"
        }
      });

      options.upsertConversation(conversation);
      await options.loadContacts();
      options.sidebarView.value = "conversations";
      await options.selectConversation(conversation.id);
      return created.contact;
    } catch (error) {
      options.contactActionError.value = formatSendError(error, "Nao foi possivel criar o contato.");
      return null;
    } finally {
      options.creatingContact.value = false;
    }
  }

  async function openContactConversation(contactId: string) {
    const normalizedId = contactId.trim();
    if (!normalizedId) {
      return null;
    }

    options.contactActionError.value = "";

    try {
      const conversation = await options.apiFetch<Conversation>(`/contacts/${normalizedId}/open-conversation`, {
        method: "POST",
        body: {
          channel: "WHATSAPP"
        }
      });

      options.upsertConversation(conversation);
      await options.loadContacts();
      options.sidebarView.value = "conversations";
      await options.selectConversation(conversation.id);
      return conversation;
    } catch (error) {
      options.contactActionError.value = formatSendError(error, "Nao foi possivel abrir a conversa do contato.");
      return null;
    }
  }

  async function saveContactFromMessageCard(payload: {
    name: string;
    phone: string;
    avatarUrl?: string | null;
    contactId?: string | null;
    openConversation?: boolean;
  }) {
    const normalizedPhone = buildCanonicalContactPhone({
      phone: payload.phone
    });
    const normalizedName = payload.name.trim();

    if (!normalizedPhone) {
      options.contactActionError.value = "Telefone invalido para salvar o contato.";
      return null;
    }

    options.savingContact.value = true;
    options.contactActionError.value = "";

    try {
      const created = await options.apiFetch<SaveContactResponse>("/contacts", {
        method: "POST",
        body: {
          name: normalizedName || normalizedPhone,
          phone: normalizedPhone,
          avatarUrl: payload.avatarUrl ?? undefined,
          source: "MESSAGE_CARD"
        }
      });

      const persistedContact = created.contact ?? null;
      if (!persistedContact?.id) {
        throw new Error("Contato nao retornado pela API.");
      }

      options.upsertContact(persistedContact);
      options.syncSavedContactIntoMessages(persistedContact);
      await options.loadContacts();

      if (payload.openConversation) {
        return await openContactConversation(persistedContact.id);
      }

      return persistedContact;
    } catch (error) {
      options.contactActionError.value = formatSendError(error, "Nao foi possivel salvar o contato.");
      return null;
    } finally {
      options.savingContact.value = false;
    }
  }

  async function previewWhatsAppContactsImport(limit = 500) {
    importingContacts.value = true;
    options.contactActionError.value = "";

    try {
      const response = await options.apiFetch<WhatsAppContactImportResponse>("/contacts/import-whatsapp", {
        method: "POST",
        body: {
          dryRun: true,
          updateExisting: true,
          overwriteNames: false,
          overwriteAvatars: false,
          includeGroups: false,
          limit
        }
      });

      contactImportPreview.value = response;
      contactImportResult.value = null;
      return response;
    } catch (error) {
      options.contactActionError.value = formatSendError(
        error,
        "Nao foi possivel gerar preview da importacao de contatos."
      );
      return null;
    } finally {
      importingContacts.value = false;
    }
  }

  async function applyWhatsAppContactsImport(limit = 500) {
    importingContacts.value = true;
    options.contactActionError.value = "";

    try {
      if (!contactImportPreview.value) {
        const preview = await previewWhatsAppContactsImport(limit);
        if (!preview) {
          return null;
        }
      }

      importingContacts.value = true;
      const selectedPhones = (contactImportPreview.value?.items ?? [])
        .filter((entry) => entry.action === "create" || entry.action === "update")
        .map((entry) => entry.phone);

      const response = await options.apiFetch<WhatsAppContactImportResponse>("/contacts/import-whatsapp", {
        method: "POST",
        body: {
          dryRun: false,
          updateExisting: true,
          overwriteNames: false,
          overwriteAvatars: false,
          includeGroups: false,
          limit,
          selectedPhones
        }
      });

      contactImportPreview.value = response;
      contactImportResult.value = response;
      await Promise.all([options.loadContacts(), options.loadConversations()]);
      return response;
    } catch (error) {
      options.contactActionError.value = formatSendError(error, "Nao foi possivel importar contatos do WhatsApp.");
      return null;
    } finally {
      importingContacts.value = false;
    }
  }

  function clearWhatsAppContactsImportPreview() {
    contactImportPreview.value = null;
    contactImportResult.value = null;
  }

  return {
    saveActiveConversationContact,
    createContactAndOpenConversation,
    saveContactFromMessageCard,
    openContactConversation,
    importingContacts,
    contactImportPreview,
    contactImportResult,
    previewWhatsAppContactsImport,
    applyWhatsAppContactsImport,
    clearWhatsAppContactsImportPreview
  };
}

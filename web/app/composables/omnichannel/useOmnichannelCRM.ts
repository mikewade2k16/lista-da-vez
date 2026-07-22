import { ref } from "vue";
import { useApi } from "~/composables/useApi";

export type CRMContact = {
  id: string;
  name: string;
  phone: string;
  primaryEmail?: string | null;
  source: string;
  relationshipStatus: "new_lead" | "known_lead" | "customer" | "inactive" | string;
  tags: string[];
  ownerUserId?: string | null;
  mergedIntoContactId?: string | null;
  firstChannel?: string | null;
  lastChannel?: string | null;
  firstSeenAt?: string | null;
  lastSeenAt?: string | null;
  lastConversationId?: string | null;
  lastConversationAt?: string | null;
  lastConversationStatus?: string | null;
  customFields?: Record<string, unknown> | null;
  classificationSource?: string | null;
  classificationConfidence?: number | null;
  lastQualifiedAt?: string | null;
  createdAt?: string | null;
  updatedAt?: string | null;
};

export type CRMContactProfile = {
  contact: CRMContact;
  identities: Array<{ id: string; channel: string; provider: string; externalId: string; lastSeenAt: string }>;
  touchpoints: Array<{ id: string; channel: string; sourceKind: string; occurredAt: string }>;
  notes: Array<{ id: string; content: string; createdAt: string; authorUserId?: string | null }>;
  conversations: Array<{ id: string; channel: string; state: string; lastMessageAt: string }>;
  touchpointsHasMore: boolean;
  notesHasMore: boolean;
};

export type CRMPage = { contacts?: CRMContact[]; hasMore?: boolean; nextCursor?: string };

export type CRMContactListOptions = {
  search?: string;
  status?: string;
  channel?: string;
  tag?: string;
  owner?: string;
  source?: string;
  before?: string;
  append?: boolean;
};

export type CRMContactPatch = {
  name?: string;
  primaryEmail?: string | null;
  ownerUserId?: string | null;
  relationshipStatus?: string;
  tags?: string[];
  customFields?: Record<string, unknown>;
  expectedUpdatedAt?: string | null;
};

export type CRMContactNote = {
  id: string;
  conversationId?: string | null;
  authorUserId?: string | null;
  content: string;
  createdAt: string;
  updatedAt?: string;
};

export type CRMContactMergeResult = {
  eventId: string;
  sourceContactId: string;
  targetContactId: string;
  undoneAt?: string | null;
  createdAt: string;
};

function asArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === "string") : [];
}

function normalizeContact(value: CRMContact): CRMContact {
  return { ...value, tags: asArray(value.tags) };
}

export function useOmnichannelCRM() {
  const { apiFetch } = useApi();
  const contacts = ref<CRMContact[]>([]);
  const profile = ref<CRMContactProfile | null>(null);
  const loading = ref(false);
  const loadingProfile = ref(false);
  const loadingMore = ref(false);
  const ready = ref(false);
  const error = ref("");
  const hasMore = ref(false);
  const nextCursor = ref("");

  async function loadContacts(options: CRMContactListOptions = {}) {
    const append = options.append === true && Boolean(options.before?.trim());
    loading.value = !append;
    loadingMore.value = append;
    error.value = "";
    try {
      const query = new URLSearchParams({ limit: "100" });
      if (options.search?.trim()) query.set("q", options.search.trim());
      if (options.status?.trim()) query.set("status", options.status.trim());
      if (options.channel?.trim()) query.set("channel", options.channel.trim());
      if (options.tag?.trim()) query.set("tag", options.tag.trim());
      if (options.owner?.trim()) query.set("owner", options.owner.trim());
      if (options.source?.trim()) query.set("source", options.source.trim());
      if (options.before?.trim()) query.set("before", options.before.trim());
      const response = await apiFetch<CRMPage>(`/contacts/crm?${query.toString()}`);
      const incoming = (response.contacts ?? []).map(normalizeContact);
      if (append) {
        const existing = new Set(contacts.value.map((entry) => entry.id));
        contacts.value = [...contacts.value, ...incoming.filter((entry) => !existing.has(entry.id))];
      } else {
        contacts.value = incoming;
      }
      hasMore.value = response.hasMore === true;
      nextCursor.value = response.nextCursor ?? "";
      ready.value = true;
      return response;
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : "Não foi possível carregar o CRM.";
      ready.value = false;
      return null;
    } finally {
      loading.value = false;
      loadingMore.value = false;
    }
  }

  async function loadProfile(contactId: string) {
    loadingProfile.value = true;
    try {
      profile.value = await apiFetch<CRMContactProfile>(`/contacts/${encodeURIComponent(contactId)}/profile?limit=25`);
      return profile.value;
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : "Não foi possível carregar o perfil.";
      profile.value = null;
      return null;
    } finally {
      loadingProfile.value = false;
    }
  }

  async function updateContact(contactId: string, patch: CRMContactPatch) {
    const updated = await apiFetch<CRMContact>(`/contacts/${encodeURIComponent(contactId)}/crm`, { method: "PATCH", body: patch });
    const index = contacts.value.findIndex((entry) => entry.id === contactId);
    if (index >= 0) contacts.value[index] = normalizeContact(updated);
    if (profile.value?.contact.id === contactId) profile.value.contact = normalizeContact(updated);
    return updated;
  }

  async function createNote(contactId: string, content: string, conversationId?: string | null) {
    const note = await apiFetch<CRMContactNote>(`/contacts/${encodeURIComponent(contactId)}/notes`, {
      method: "POST",
      body: {
        content,
        ...(conversationId ? { conversationId } : {})
      }
    });
    if (profile.value?.contact.id === contactId) {
      profile.value.notes = [note, ...profile.value.notes.filter((entry) => entry.id !== note.id)];
    }
    return note;
  }

  async function mergeContacts(sourceContactId: string, targetContactId: string, reason: string, idempotencyKey: string) {
    return apiFetch<CRMContactMergeResult>(`/contacts/${encodeURIComponent(sourceContactId)}/merge`, {
      method: "POST",
      body: { targetId: targetContactId, reason, idempotencyKey }
    });
  }

  async function undoMerge(eventId: string) {
    return apiFetch<CRMContactMergeResult>(`/contacts/merges/${encodeURIComponent(eventId)}/undo`, {
      method: "POST"
    });
  }

  function clearProfile() { profile.value = null; }

  return {
    contacts,
    profile,
    loading,
    loadingProfile,
    loadingMore,
    ready,
    error,
    hasMore,
    nextCursor,
    loadContacts,
    loadProfile,
    updateContact,
    createNote,
    mergeContacts,
    undoMerge,
    clearProfile
  };
}

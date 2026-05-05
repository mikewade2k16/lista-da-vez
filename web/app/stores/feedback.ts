import { computed, ref, type Ref } from "vue";
import { defineStore } from "pinia";
import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiBase, getApiErrorMessage } from "~/utils/api-client";

interface FeedbackItem {
  id: string;
  tenant_id: string;
  store_id: string;
  user_id: string;
  user_name: string;
  kind: string;
  status: string;
  subject: string;
  body: string;
  admin_note: string;
  user_last_read_at: string;
  created_at: string;
  updated_at: string;
}

interface FeedbackMessageItem {
  id: string;
  tenant_id: string;
  feedback_id: string;
  author_user_id: string;
  author_name: string;
  author_role: string;
  body: string;
  image_url: string;
  image_content_type: string;
  image_size_bytes: number;
  image_expires_at?: string | null;
  created_at: string;
}

interface CreateFeedbackInput {
  kind: string;
  subject: string;
  body: string;
  image?: File | null;
}

interface UpdateFeedbackInput {
  status?: string;
  admin_note?: string;
}

interface CreateFeedbackMessageInput {
  body?: string;
  image?: File | null;
}

export const useFeedbackStore = defineStore("feedback", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const items = ref<FeedbackItem[]>([]);
  const myItems = ref<FeedbackItem[]>([]);
  const messagesByFeedbackId = ref<Record<string, FeedbackMessageItem[]>>({});
  const loading = ref(false);
  const error = ref("");

  const feedbacks = computed(() => items.value);
  const myFeedbacks = computed(() => myItems.value);

  function resolveUploadUrl(path: string) {
    const normalizedPath = String(path || "").trim();
    if (!normalizedPath) {
      return "";
    }

    try {
      return new URL(normalizedPath, getApiBase(runtimeConfig)).toString();
    } catch {
      return normalizedPath;
    }
  }

  function normalizeMessage(message: FeedbackMessageItem) {
    if (!message?.id) {
      return message;
    }

    return {
      ...message,
      image_url: resolveUploadUrl(message.image_url)
    };
  }

  function buildFeedbackFormData(fields: Record<string, string>, image?: File | null) {
    const formData = new FormData();

    for (const [key, value] of Object.entries(fields)) {
      formData.append(key, String(value ?? ""));
    }

    if (image) {
      formData.append("image", image);
    }

    return formData;
  }

  function getFeedbackActivityTime(feedback: Partial<FeedbackItem>) {
    const updatedAt = new Date(feedback.updated_at || feedback.created_at || 0).getTime();
    return Number.isFinite(updatedAt) ? updatedAt : 0;
  }

  function sortFeedbacks(feedbacks: FeedbackItem[]) {
    return [...feedbacks].sort((left, right) =>
      getFeedbackActivityTime(right) - getFeedbackActivityTime(left)
    );
  }

  function getLastMessageCreatedAt(feedbackId: string) {
    const messages = messagesByFeedbackId.value[String(feedbackId || "").trim()] || [];
    const timestamps = messages
      .map((message) => new Date(message.created_at).getTime())
      .filter((value) => Number.isFinite(value));

    if (!timestamps.length) {
      return "";
    }

    return new Date(Math.max(...timestamps)).toISOString();
  }

  function upsertIntoCollection(collection: Ref<FeedbackItem[]>, feedbacks: FeedbackItem[]) {
    const byId = new Map(collection.value.map((feedback) => [feedback.id, feedback]));

    for (const feedback of feedbacks) {
      if (feedback?.id) {
        byId.set(feedback.id, feedback);
      }
    }

    collection.value = sortFeedbacks(Array.from(byId.values()));
  }

  function upsertFeedbacks(feedbacks: FeedbackItem[]) {
    upsertIntoCollection(items, feedbacks);
  }

  function upsertMyFeedbacks(feedbacks: FeedbackItem[]) {
    upsertIntoCollection(myItems, feedbacks);
  }

  function upsertMessages(feedbackId: string, messages: FeedbackMessageItem[]) {
    const currentMessages = messagesByFeedbackId.value[feedbackId] || [];
    const byId = new Map(currentMessages.map((message) => [message.id, message]));

    for (const rawMessage of messages) {
      const message = normalizeMessage(rawMessage);
      if (message?.id) {
        byId.set(message.id, message);
      }
    }

    messagesByFeedbackId.value = {
      ...messagesByFeedbackId.value,
      [feedbackId]: Array.from(byId.values()).sort((left, right) =>
        new Date(left.created_at).getTime() - new Date(right.created_at).getTime()
      )
    };
  }

  function patchFeedbackCollections(feedbackId: string, patch: Partial<FeedbackItem>) {
    const applyPatch = (collection: Ref<FeedbackItem[]>) => {
      const current = collection.value.find((feedback) => feedback.id === feedbackId);
      if (!current) {
        return;
      }

      upsertIntoCollection(collection, [{ ...current, ...patch }]);
    };

    applyPatch(items);
    applyPatch(myItems);
  }

  function getFeedbackById(feedbackId: string) {
    const normalizedId = String(feedbackId || "").trim();
    return items.value.find((feedback) => feedback.id === normalizedId) ||
      myItems.value.find((feedback) => feedback.id === normalizedId) ||
      null;
  }

  function getLocalReadCursor(feedbackId: string) {
    const lastMessageCreatedAt = getLastMessageCreatedAt(feedbackId);
    if (lastMessageCreatedAt) {
      return lastMessageCreatedAt;
    }

    const feedback = getFeedbackById(feedbackId);
    return feedback?.updated_at || feedback?.created_at || new Date().toISOString();
  }

  function applyLocalReadState(feedbackId: string, readAt?: string) {
    const normalizedId = String(feedbackId || "").trim();
    if (!normalizedId) {
      return "";
    }

    const feedback = getFeedbackById(normalizedId);
    if (!feedback) {
      return "";
    }

    const nextReadAt = String(readAt || getLocalReadCursor(normalizedId) || "").trim();
    const currentReadAt = String(feedback.user_last_read_at || feedback.created_at || "").trim();

    if (!nextReadAt) {
      return currentReadAt;
    }

    if (currentReadAt) {
      const nextReadTime = new Date(nextReadAt).getTime();
      const currentReadTime = new Date(currentReadAt).getTime();
      if (Number.isFinite(nextReadTime) && Number.isFinite(currentReadTime) && nextReadTime < currentReadTime) {
        return currentReadAt;
      }
    }

    patchFeedbackCollections(normalizedId, {
      user_last_read_at: nextReadAt
    });

    return nextReadAt;
  }

  function isNotFoundError(err: unknown) {
    const error = err as { statusCode?: number; status?: number; response?: { status?: number } };
    return error?.statusCode === 404 || error?.status === 404 || error?.response?.status === 404;
  }

  async function submitFeedback(input: CreateFeedbackInput) {
    try {
      loading.value = true;
      error.value = "";

      const requestBody = input.image
        ? buildFeedbackFormData(
            {
              kind: input.kind,
              subject: input.subject,
              body: input.body
            },
            input.image
          )
        : {
            kind: input.kind,
            subject: input.subject,
            body: input.body
          };

      const response = await apiRequest("/v1/feedback", {
        method: "POST",
        body: requestBody
      });

      const createdFeedback = response.feedback;

      if (createdFeedback?.id) {
        upsertFeedbacks([createdFeedback]);
        upsertMyFeedbacks([createdFeedback]);
      }

      return { ok: true, data: createdFeedback };
    } catch (err) {
      const message = getApiErrorMessage(
        err,
        "Erro ao enviar feedback. Tente novamente."
      );
      error.value = message;
      return { ok: false, message };
    } finally {
      loading.value = false;
    }
  }

  async function fetchFeedbacks(filters?: { kind?: string; status?: string; since?: string }) {
    const shouldShowLoading = !filters?.since;
    try {
      if (shouldShowLoading) {
        loading.value = true;
      }
      error.value = "";

      const params = new URLSearchParams();
      if (filters?.kind) params.append("kind", filters.kind);
      if (filters?.status) params.append("status", filters.status);
      if (filters?.since) params.append("since", filters.since);

      const query = params.toString() ? `?${params.toString()}` : "";
      const response = await apiRequest(`/v1/feedback${query}`);

      if (filters?.since) {
        upsertFeedbacks(response.feedbacks || []);
      } else {
        items.value = sortFeedbacks(response.feedbacks || []);
      }
      return { ok: true, data: items.value };
    } catch (err) {
      const message = getApiErrorMessage(
        err,
        "Erro ao carregar feedbacks. Tente novamente."
      );
      error.value = message;
      return { ok: false, message };
    } finally {
      if (shouldShowLoading) {
        loading.value = false;
      }
    }
  }

  async function fetchMyFeedbacks(filters?: { kind?: string; status?: string; since?: string }) {
    const shouldShowLoading = !filters?.since;
    try {
      if (shouldShowLoading) {
        loading.value = true;
      }
      error.value = "";

      const params = new URLSearchParams();
      if (filters?.kind) params.append("kind", filters.kind);
      if (filters?.status) params.append("status", filters.status);
      if (filters?.since) params.append("since", filters.since);

      const query = params.toString() ? `?${params.toString()}` : "";
      const response = await apiRequest(`/v1/feedback/me${query}`);

      if (filters?.since) {
        upsertMyFeedbacks(response.feedbacks || []);
      } else {
        myItems.value = sortFeedbacks(response.feedbacks || []);
      }
      return { ok: true, data: myItems.value };
    } catch (err) {
      const message = getApiErrorMessage(
        err,
        "Erro ao carregar seus chamados. Tente novamente."
      );
      error.value = message;
      return { ok: false, message };
    } finally {
      if (shouldShowLoading) {
        loading.value = false;
      }
    }
  }

  async function updateFeedback(id: string, input: UpdateFeedbackInput) {
    try {
      loading.value = true;
      error.value = "";
      const currentFeedback =
        items.value.find((feedback) => feedback.id === id) ||
        myItems.value.find((feedback) => feedback.id === id) ||
        null;

      const response = await apiRequest(`/v1/feedback/${id}`, {
        method: "PATCH",
        body: {
          status: input.status,
          admin_note: input.admin_note
        }
      });

      const updatedFeedback = response.feedback?.id
        ? {
            ...response.feedback,
            user_last_read_at:
              currentFeedback?.user_last_read_at || response.feedback.user_last_read_at
          }
        : null;

      if (updatedFeedback?.id) {
        upsertFeedbacks([updatedFeedback]);
        upsertMyFeedbacks([updatedFeedback]);
      }

      return { ok: true, data: updatedFeedback };
    } catch (err) {
      const message = getApiErrorMessage(
        err,
        "Erro ao atualizar feedback. Tente novamente."
      );
      error.value = message;
      return { ok: false, message };
    } finally {
      loading.value = false;
    }
  }

  async function fetchMessages(feedbackId: string, options?: { after?: string }) {
    try {
      error.value = "";

      const params = new URLSearchParams();
      if (options?.after) params.append("after", options.after);

      const query = params.toString() ? `?${params.toString()}` : "";
      const response = await apiRequest(`/v1/feedback/${feedbackId}/messages${query}`);
      const messages = response.messages || [];

      upsertMessages(feedbackId, messages);
      return { ok: true, data: messagesByFeedbackId.value[feedbackId] || [] };
    } catch (err) {
      if (isNotFoundError(err)) {
        return { ok: true, data: messagesByFeedbackId.value[feedbackId] || [] };
      }

      const message = getApiErrorMessage(
        err,
        "Erro ao carregar mensagens do feedback. Tente novamente."
      );
      error.value = message;
      return { ok: false, message };
    }
  }

  async function sendMessage(feedbackId: string, input: CreateFeedbackMessageInput) {
    try {
      loading.value = true;
      error.value = "";

      const requestBody = input.image
        ? buildFeedbackFormData(
            {
              body: String(input.body || "")
            },
            input.image
          )
        : {
            body: String(input.body || "")
          };

      const response = await apiRequest(`/v1/feedback/${feedbackId}/messages`, {
        method: "POST",
        body: requestBody
      });

      if (response.message) {
        upsertMessages(feedbackId, [response.message]);

        if (
          String(response.message.author_user_id || "").trim() ===
          String(auth.user?.id || "").trim()
        ) {
          patchFeedbackCollections(feedbackId, {
            updated_at: response.message.created_at,
            user_last_read_at: response.message.created_at
          });
        }
      }

      return { ok: true, data: response.message };
    } catch (err) {
      if (isNotFoundError(err)) {
        return {
          ok: false,
          message: "Endpoint de mensagens ainda nao esta disponivel no backend em execucao."
        };
      }

      const message = getApiErrorMessage(
        err,
        "Erro ao enviar resposta. Tente novamente."
      );
      error.value = message;
      return { ok: false, message };
    } finally {
      loading.value = false;
    }
  }

  async function markFeedbackAsRead(feedbackId: string) {
    const normalizedId = String(feedbackId || "").trim();
    const currentFeedback = getFeedbackById(normalizedId);
    const previousReadAt = String(currentFeedback?.user_last_read_at || currentFeedback?.created_at || "").trim();
    applyLocalReadState(normalizedId);

    try {
      error.value = "";

      const response = await apiRequest(`/v1/feedback/${normalizedId}/read`, {
        method: "POST"
      });

      if (response.feedback?.id) {
        upsertFeedbacks([response.feedback]);
        upsertMyFeedbacks([response.feedback]);
      }

      return { ok: true, data: response.feedback };
    } catch (err) {
      const message = getApiErrorMessage(
        err,
        "Erro ao marcar chamado como lido. Tente novamente."
      );
      if (normalizedId && currentFeedback) {
        patchFeedbackCollections(normalizedId, {
          user_last_read_at: previousReadAt
        });
      }
      error.value = message;
      return { ok: false, message };
    }
  }

  async function syncMessagesForFeedbacks(feedbackIds: string[]) {
    const uniqueIds = Array.from(
      new Set(
        feedbackIds
          .map((feedbackId) => String(feedbackId || "").trim())
          .filter(Boolean)
      )
    );

    if (!uniqueIds.length) {
      return { ok: true, data: [] };
    }

    const results = await Promise.all(
      uniqueIds.map((feedbackId) =>
        fetchMessages(feedbackId, {
          after: getLastMessageCreatedAt(feedbackId)
        })
      )
    );

    return {
      ok: results.every((result) => result.ok),
      data: uniqueIds.map((feedbackId) => messagesByFeedbackId.value[feedbackId] || [])
    };
  }

  return {
    items,
    myItems,
    messagesByFeedbackId,
    feedbacks,
    myFeedbacks,
    loading,
    error,
    submitFeedback,
    fetchFeedbacks,
    fetchMyFeedbacks,
    updateFeedback,
    fetchMessages,
    applyLocalReadState,
    syncMessagesForFeedbacks,
    sendMessage,
    markFeedbackAsRead
  };
});

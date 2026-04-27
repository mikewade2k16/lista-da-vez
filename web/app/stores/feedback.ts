import { computed, ref } from "vue";
import { defineStore } from "pinia";
import { useAuthStore } from "~/stores/auth";
import { createApiRequest, getApiErrorMessage } from "~/utils/api-client";

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
  created_at: string;
  updated_at: string;
}

interface CreateFeedbackInput {
  kind: string;
  subject: string;
  body: string;
}

interface UpdateFeedbackInput {
  status?: string;
  admin_note?: string;
}

export const useFeedbackStore = defineStore("feedback", () => {
  const runtimeConfig = useRuntimeConfig();
  const auth = useAuthStore();
  const apiRequest = createApiRequest(runtimeConfig, () => auth.accessToken);

  const items = ref<FeedbackItem[]>([]);
  const loading = ref(false);
  const error = ref("");

  const feedbacks = computed(() => items.value);

  async function submitFeedback(input: CreateFeedbackInput) {
    try {
      loading.value = true;
      error.value = "";

      const response = await apiRequest("/v1/feedback", {
        method: "POST",
        body: {
          kind: input.kind,
          subject: input.subject,
          body: input.body
        }
      });

      return { ok: true, data: response.feedback };
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

  async function fetchFeedbacks(filters?: { kind?: string; status?: string }) {
    try {
      loading.value = true;
      error.value = "";

      const params = new URLSearchParams();
      if (filters?.kind) params.append("kind", filters.kind);
      if (filters?.status) params.append("status", filters.status);

      const query = params.toString() ? `?${params.toString()}` : "";
      const response = await apiRequest(`/v1/feedback${query}`);

      items.value = response.feedbacks || [];
      return { ok: true, data: items.value };
    } catch (err) {
      const message = getApiErrorMessage(
        err,
        "Erro ao carregar feedbacks. Tente novamente."
      );
      error.value = message;
      return { ok: false, message };
    } finally {
      loading.value = false;
    }
  }

  async function updateFeedback(id: string, input: UpdateFeedbackInput) {
    try {
      loading.value = true;
      error.value = "";

      const response = await apiRequest(`/v1/feedback/${id}`, {
        method: "PATCH",
        body: {
          status: input.status,
          admin_note: input.admin_note
        }
      });

      const index = items.value.findIndex((f) => f.id === id);
      if (index !== -1) {
        items.value[index] = response.feedback;
      }

      return { ok: true, data: response.feedback };
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

  return {
    items,
    feedbacks,
    loading,
    error,
    submitFeedback,
    fetchFeedbacks,
    updateFeedback
  };
});

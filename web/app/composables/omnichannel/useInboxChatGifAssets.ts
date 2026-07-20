import { ref } from "vue";
import { useApi } from "~/composables/useApi";

// F1 — REPONTADO (nao verbatim). O legado chamava rotas Nitro same-origin
// (`/api/gif/search` e `/api/gif/media`). O web do Omni nao tem Nitro (BFF
// eliminado em 2026-07-02, ADR 0002) — as duas passam a apontar para o Go em
// `/v1/omnichannel/gif/*`, via apiFetch (token e X-Account-Id do provider
// global). Comportamento intacto; so muda para onde apontam.
// As rotas de GIF sao da F12 — ate la, 404 (esperado na F1).

export type GifSearchResultItem = {
  id: string;
  title: string;
  previewUrl: string | null;
  mediaUrl: string | null;
  mimeType: string | null;
};

export function useInboxChatGifAssets(options: {
  onPickAttachment: (payload: { file: File; mode: "gif" }) => void;
  onClosePanel: () => void;
}) {
  const { apiFetch } = useApi();
  const gifSearch = ref("");
  const gifLoading = ref(false);
  const gifError = ref("");
  const gifResults = ref<GifSearchResultItem[]>([]);

  let gifSearchTimer: number | null = null;

  function updateGifSearch(value: string) {
    gifSearch.value = value;
  }

  function clearGifSearchTimer() {
    if (!import.meta.client) {
      return;
    }

    if (gifSearchTimer !== null) {
      window.clearTimeout(gifSearchTimer);
      gifSearchTimer = null;
    }
  }

  async function fetchGifResults() {
    const query = gifSearch.value.trim();
    if (!query) {
      gifResults.value = [];
      gifError.value = "";
      return;
    }

    gifLoading.value = true;
    gifError.value = "";

    try {
      const response = await apiFetch<{ items?: GifSearchResultItem[]; error?: string }>("/gif/search", {
        query: {
          q: query,
          limit: 24
        }
      });

      gifError.value = typeof response.error === "string" ? response.error : "";
      gifResults.value = Array.isArray(response.items) ? response.items : [];
    } catch (error) {
      gifError.value = error instanceof Error ? error.message : "Nao foi possivel consultar GIFs.";
      gifResults.value = [];
    } finally {
      gifLoading.value = false;
    }
  }

  function queueGifSearch() {
    if (!import.meta.client) {
      return;
    }

    clearGifSearchTimer();
    gifSearchTimer = window.setTimeout(() => {
      void fetchGifResults();
    }, 250);
  }

  async function pickGifResult(item: GifSearchResultItem) {
    if (!item.mediaUrl) {
      return;
    }

    gifError.value = "";
    try {
      let blob: Blob;
      try {
        blob = await apiFetch<Blob>("/gif/media", {
          query: { url: item.mediaUrl },
          responseType: "blob"
        });
      } catch {
        throw new Error("Falha ao carregar GIF selecionado.");
      }

      const mimeType = blob.type || item.mimeType || "video/mp4";
      const extension = mimeType.includes("mp4") ? "mp4" : mimeType.includes("gif") ? "gif" : "bin";
      const safeTitle = (item.title || "gif").toLowerCase().replace(/[^a-z0-9_-]+/g, "-").slice(0, 32) || "gif";
      const file = new File([blob], `${safeTitle}-${Date.now()}.${extension}`, {
        type: mimeType
      });

      options.onPickAttachment({
        file,
        mode: "gif"
      });
      options.onClosePanel();
    } catch (error) {
      gifError.value = error instanceof Error ? error.message : "Nao foi possivel anexar o GIF.";
    }
  }

  return {
    gifSearch,
    gifLoading,
    gifError,
    gifResults,
    updateGifSearch,
    clearGifSearchTimer,
    queueGifSearch,
    pickGifResult
  };
}

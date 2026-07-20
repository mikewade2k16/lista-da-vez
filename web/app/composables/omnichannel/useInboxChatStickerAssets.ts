import { ref } from "vue";
import type { SavedSticker } from "~/types";

export type SavedStickerItem = {
  id: string;
  name: string;
  dataUrl: string;
  mimeType: string;
  sizeBytes: number;
  createdAt: string;
  updatedAt?: string;
  createdByUserId?: string | null;
};

const SAVED_STICKERS_MAX = 36;

export function useInboxChatStickerAssets(options: {
  apiFetch: <T = unknown>(path: string, init?: Record<string, unknown>) => Promise<T>;
  asRecord: (value: unknown) => Record<string, unknown> | null;
  onPickAttachment: (payload: { file: File; mode: "sticker" }) => void;
  onClosePanel: () => void;
}) {
  const savedStickers = ref<SavedStickerItem[]>([]);
  const savedStickersLoading = ref(false);
  const stickerError = ref("");

  function normalizeSavedStickerItem(value: unknown): SavedStickerItem | null {
    const entry = options.asRecord(value);
    if (!entry) {
      return null;
    }

    const id = typeof entry.id === "string" ? entry.id.trim() : "";
    const dataUrl = typeof entry.dataUrl === "string" ? entry.dataUrl : "";
    if (!id || !dataUrl.startsWith("data:image/")) {
      return null;
    }

    return {
      id,
      name: typeof entry.name === "string" && entry.name.trim().length > 0 ? entry.name.trim() : "figurinha.webp",
      dataUrl,
      mimeType: typeof entry.mimeType === "string" ? entry.mimeType : "image/webp",
      sizeBytes: typeof entry.sizeBytes === "number" ? Math.max(0, Math.trunc(entry.sizeBytes)) : 0,
      createdAt: typeof entry.createdAt === "string" ? entry.createdAt : new Date().toISOString(),
      updatedAt: typeof entry.updatedAt === "string" ? entry.updatedAt : undefined,
      createdByUserId: typeof entry.createdByUserId === "string" ? entry.createdByUserId : null
    };
  }

  async function loadSavedStickers() {
    savedStickersLoading.value = true;
    try {
      const response = await options.apiFetch<SavedSticker[]>(`/stickers?limit=${SAVED_STICKERS_MAX}`);
      const normalized = Array.isArray(response)
        ? response
            .map((entry) => normalizeSavedStickerItem(entry))
            .filter((entry): entry is SavedStickerItem => Boolean(entry))
        : [];

      savedStickers.value = normalized;
      stickerError.value = "";
    } catch (error) {
      savedStickers.value = [];
      stickerError.value = error instanceof Error ? error.message : "Nao foi possivel carregar figurinhas salvas.";
    } finally {
      savedStickersLoading.value = false;
    }
  }

  function readFileAsDataUrl(file: File) {
    return new Promise<string>((resolve, reject) => {
      const reader = new FileReader();
      reader.onload = () => {
        if (typeof reader.result === "string" && reader.result.startsWith("data:")) {
          resolve(reader.result);
          return;
        }

        reject(new Error("Nao foi possivel ler figurinha."));
      };
      reader.onerror = () => reject(new Error("Nao foi possivel ler figurinha."));
      reader.readAsDataURL(file);
    });
  }

  function getFileExtensionFromMime(mimeType: string) {
    const normalized = mimeType.split(";")[0]?.trim().toLowerCase() || "";
    if (normalized === "image/webp") {
      return "webp";
    }
    if (normalized === "image/png") {
      return "png";
    }
    if (normalized === "image/jpeg" || normalized === "image/jpg") {
      return "jpg";
    }

    return "webp";
  }

  function dataUrlToStickerFile(item: SavedStickerItem) {
    const match = item.dataUrl.match(/^data:([^;,]+);base64,(.+)$/i);
    if (!match) {
      return null;
    }

    const mimeType = match[1] || item.mimeType || "image/webp";
    const base64 = match[2];
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);

    for (let index = 0; index < binary.length; index += 1) {
      bytes[index] = binary.charCodeAt(index);
    }

    const safeName = item.name?.trim() || `figurinha-${item.id}.${getFileExtensionFromMime(mimeType)}`;
    return new File([bytes], safeName, {
      type: mimeType
    });
  }

  async function saveStickerFromFile(file: File) {
    const mimeType = file.type.toLowerCase();
    if (!mimeType.startsWith("image/")) {
      return;
    }

    try {
      const dataUrl = await readFileAsDataUrl(file);
      const duplicated = savedStickers.value.find((entry) => entry.dataUrl === dataUrl);
      if (duplicated) {
        stickerError.value = "";
        return;
      }

      const name = file.name?.trim() || `figurinha-${Date.now()}.${getFileExtensionFromMime(mimeType)}`;
      const created = await options.apiFetch<SavedSticker>("/stickers", {
        method: "POST",
        body: {
          name,
          dataUrl,
          mimeType: file.type || "image/webp",
          sizeBytes: file.size
        }
      });

      const normalized = normalizeSavedStickerItem(created);
      if (normalized) {
        savedStickers.value = [normalized, ...savedStickers.value.filter((entry) => entry.id !== normalized.id)]
          .slice(0, SAVED_STICKERS_MAX);
      } else {
        await loadSavedStickers();
      }

      stickerError.value = "";
    } catch (error) {
      stickerError.value = error instanceof Error ? error.message : "Nao foi possivel salvar figurinha.";
    }
  }

  function selectSavedSticker(item: SavedStickerItem) {
    const file = dataUrlToStickerFile(item);
    if (!file) {
      stickerError.value = "Figurinha salva invalida. Remova e adicione novamente.";
      return;
    }

    options.onPickAttachment({
      file,
      mode: "sticker"
    });
    options.onClosePanel();
  }

  async function removeSavedSticker(stickerId: string) {
    try {
      await options.apiFetch(`/stickers/${stickerId}`, {
        method: "DELETE"
      });

      savedStickers.value = savedStickers.value.filter((entry) => entry.id !== stickerId);
      stickerError.value = "";
    } catch (error) {
      stickerError.value = error instanceof Error ? error.message : "Nao foi possivel remover figurinha.";
    }
  }

  return {
    savedStickers,
    savedStickersLoading,
    stickerError,
    loadSavedStickers,
    saveStickerFromFile,
    selectSavedSticker,
    removeSavedSticker
  };
}

import { ref } from "vue";
import type { Message, MessageType } from "~/types";
import { ApiClientError, useApi } from "~/composables/useApi";

// getToken/getSelectedTenantSlug seguem no contrato porque o chamador verbatim
// (InboxChatPanel.vue:583) os passa. Nao sao mais usados aqui: quem resolve token
// e escopo de conta agora e o apiFetch (ver fetchMessageMediaBlob).
export function useInboxChatMediaActions(options: {
  getToken: () => string | null;
  getSelectedTenantSlug: () => string | null;
  resolveMessageType: (messageEntry: Message) => MessageType;
}) {
  const { apiFetch } = useApi();
  const failedImageMessageIds = ref<Record<string, true>>({});
  const mediaActionLoadingByMessageId = ref<Record<string, true>>({});
  const imagePreviewUrlByMessageId = ref<Record<string, string>>({});
  const imagePreviewLoadingByMessageId = ref<Record<string, true>>({});
  const audioPreviewUrlByMessageId = ref<Record<string, string>>({});
  const audioPreviewLoadingByMessageId = ref<Record<string, true>>({});

  function markImageFailed(messageId: string) {
    failedImageMessageIds.value = {
      ...failedImageMessageIds.value,
      [messageId]: true
    };
  }

  function clearImageFailed(messageId: string) {
    if (!failedImageMessageIds.value[messageId]) {
      return;
    }

    const next = { ...failedImageMessageIds.value };
    delete next[messageId];
    failedImageMessageIds.value = next;
  }

  function isImageFailed(messageId: string) {
    return Boolean(failedImageMessageIds.value[messageId]);
  }

  function getMessageMediaUrl(messageEntry: Message) {
    return messageEntry.mediaUrl ?? undefined;
  }

  function isAudioDocumentMessage(messageEntry: Message) {
    const type = options.resolveMessageType(messageEntry);
    if (type !== "DOCUMENT") {
      return false;
    }

    const mimeType = messageEntry.mediaMimeType?.trim().toLowerCase() ?? "";
    if (mimeType.startsWith("audio/")) {
      return true;
    }

    const metadata =
      messageEntry.metadataJson && typeof messageEntry.metadataJson === "object"
        ? (messageEntry.metadataJson as Record<string, unknown>)
        : null;
    const mediaKind = typeof metadata?.mediaKind === "string" ? metadata.mediaKind.trim().toLowerCase() : "";
    return mediaKind === "audio_file";
  }

  function normalizeMediaUrl(value: string | null | undefined) {
    const normalized = value?.trim();
    return normalized && normalized.length > 0 ? normalized : null;
  }

  function isBlobUrl(value: string) {
    return value.startsWith("blob:");
  }

  function isDataUrl(value: string) {
    return value.startsWith("data:");
  }

  function normalizeMimeType(value: string | null | undefined) {
    const normalized = value?.trim().toLowerCase() ?? "";
    if (!normalized) {
      return null;
    }

    return normalized.split(";")[0]?.trim() || null;
  }

  function sanitizeMimeType(value: string) {
    return value.trim().toLowerCase().replace(/;\s+/g, ";");
  }

  function resolveAudioMimeTypeFromFileName(fileName: string | null | undefined) {
    const normalized = fileName?.trim().toLowerCase() ?? "";
    if (!normalized) {
      return null;
    }

    const sanitized = normalized.replace(/[?#].*$/, "");
    const extension = sanitized.match(/\.([a-z0-9]{2,8})$/i)?.[1]?.toLowerCase() ?? "";
    if (!extension) {
      return null;
    }

    const extensionMimeMap: Record<string, string> = {
      aac: "audio/aac",
      m4a: "audio/mp4",
      mp3: "audio/mpeg",
      mpga: "audio/mpeg",
      oga: "audio/ogg",
      ogg: "audio/ogg",
      opus: "audio/ogg",
      wav: "audio/wav",
      weba: "audio/webm",
      webm: "audio/webm"
    };

    return extensionMimeMap[extension] ?? null;
  }

  function extractDataUrlMimeType(value: string) {
    if (!isDataUrl(value)) {
      return null;
    }

    const commaIndex = value.indexOf(",");
    if (commaIndex <= 5) {
      return null;
    }

    const metadata = value.slice(5, commaIndex);
    const mimeType = metadata.split(";")[0]?.trim().toLowerCase() ?? "";
    return mimeType || null;
  }

  function replaceDataUrlMimeType(value: string, mimeType: string) {
    if (!isDataUrl(value)) {
      return value;
    }

    const commaIndex = value.indexOf(",");
    if (commaIndex <= 5) {
      return value;
    }

    const metadata = value.slice(5, commaIndex);
    const metadataParts = metadata
      .split(";")
      .map((entry) => entry.trim())
      .filter(Boolean);
    const payload = value.slice(commaIndex + 1);
    const flags = metadataParts.slice(1);
    const nextMetadata = [mimeType, ...flags].join(";");
    return `data:${nextMetadata},${payload}`;
  }

  function resolvePlayableAudioMimeType(
    messageEntry: Message,
    fallbackMimeType?: string | null | undefined
  ) {
    const fallbackNormalized = normalizeMimeType(fallbackMimeType);
    if (fallbackNormalized?.startsWith("audio/") && fallbackMimeType?.trim()) {
      return sanitizeMimeType(fallbackMimeType);
    }

    const messageMimeType = messageEntry.mediaMimeType?.trim();
    const normalizedMessageMimeType = normalizeMimeType(messageMimeType);
    if (normalizedMessageMimeType?.startsWith("audio/") && messageMimeType) {
      return sanitizeMimeType(messageMimeType);
    }

    const inferredFromFileName = resolveAudioMimeTypeFromFileName(messageEntry.mediaFileName);
    if (inferredFromFileName) {
      return inferredFromFileName;
    }

    if (options.resolveMessageType(messageEntry) === "AUDIO") {
      return "audio/ogg";
    }

    return null;
  }

  function normalizeDirectAudioPreviewUrl(messageEntry: Message, value: string) {
    if (!isDataUrl(value)) {
      return value;
    }

    const currentMimeType = extractDataUrlMimeType(value);
    const resolvedMimeType = resolvePlayableAudioMimeType(messageEntry, currentMimeType);
    if (!resolvedMimeType) {
      return value;
    }

    const normalizedCurrentMimeType = normalizeMimeType(currentMimeType);
    const normalizedResolvedMimeType = normalizeMimeType(resolvedMimeType);
    if (
      normalizedCurrentMimeType &&
      normalizedCurrentMimeType !== "application/octet-stream" &&
      normalizedCurrentMimeType === normalizedResolvedMimeType
    ) {
      return value;
    }

    return replaceDataUrlMimeType(value, resolvedMimeType);
  }

  function shouldUseDirectImagePreviewUrl(messageEntry: Message) {
    const normalized = normalizeMediaUrl(messageEntry.mediaUrl);
    if (!normalized) {
      return false;
    }

    return isBlobUrl(normalized) || isDataUrl(normalized);
  }

  function shouldUseDirectAudioPreviewUrl(messageEntry: Message) {
    const normalized = normalizeMediaUrl(messageEntry.mediaUrl);
    if (!normalized) {
      return false;
    }

    return isBlobUrl(normalized) || isDataUrl(normalized);
  }

  function getMessageImagePreviewUrl(messageEntry: Message) {
    const directUrl = shouldUseDirectImagePreviewUrl(messageEntry)
      ? normalizeMediaUrl(messageEntry.mediaUrl)
      : null;
    if (directUrl) {
      return directUrl;
    }

    return imagePreviewUrlByMessageId.value[messageEntry.id];
  }

  function getMessageAudioPreviewUrl(messageEntry: Message) {
    const directUrl = shouldUseDirectAudioPreviewUrl(messageEntry)
      ? normalizeMediaUrl(messageEntry.mediaUrl)
      : null;
    if (directUrl) {
      return normalizeDirectAudioPreviewUrl(messageEntry, directUrl);
    }

    return audioPreviewUrlByMessageId.value[messageEntry.id] ?? null;
  }

  function setImagePreviewLoading(messageId: string, loading: boolean) {
    if (loading) {
      imagePreviewLoadingByMessageId.value = {
        ...imagePreviewLoadingByMessageId.value,
        [messageId]: true
      };
      return;
    }

    if (!imagePreviewLoadingByMessageId.value[messageId]) {
      return;
    }

    const next = { ...imagePreviewLoadingByMessageId.value };
    delete next[messageId];
    imagePreviewLoadingByMessageId.value = next;
  }

  function isImagePreviewLoading(messageId: string) {
    return Boolean(imagePreviewLoadingByMessageId.value[messageId]);
  }

  function setAudioPreviewLoading(messageId: string, loading: boolean) {
    if (loading) {
      audioPreviewLoadingByMessageId.value = {
        ...audioPreviewLoadingByMessageId.value,
        [messageId]: true
      };
      return;
    }

    if (!audioPreviewLoadingByMessageId.value[messageId]) {
      return;
    }

    const next = { ...audioPreviewLoadingByMessageId.value };
    delete next[messageId];
    audioPreviewLoadingByMessageId.value = next;
  }

  function isAudioPreviewLoading(messageId: string) {
    return Boolean(audioPreviewLoadingByMessageId.value[messageId]);
  }

  function setImagePreviewUrl(messageId: string, nextUrl: string) {
    const previousUrl = imagePreviewUrlByMessageId.value[messageId];
    if (previousUrl && previousUrl !== nextUrl && isBlobUrl(previousUrl)) {
      URL.revokeObjectURL(previousUrl);
    }

    imagePreviewUrlByMessageId.value = {
      ...imagePreviewUrlByMessageId.value,
      [messageId]: nextUrl
    };
  }

  function releaseImagePreviewUrl(messageId: string) {
    const currentUrl = imagePreviewUrlByMessageId.value[messageId];
    if (!currentUrl) {
      return;
    }

    if (isBlobUrl(currentUrl)) {
      URL.revokeObjectURL(currentUrl);
    }

    const next = { ...imagePreviewUrlByMessageId.value };
    delete next[messageId];
    imagePreviewUrlByMessageId.value = next;
  }

  function setAudioPreviewUrl(messageId: string, nextUrl: string) {
    const previousUrl = audioPreviewUrlByMessageId.value[messageId];
    if (previousUrl && previousUrl !== nextUrl && isBlobUrl(previousUrl)) {
      URL.revokeObjectURL(previousUrl);
    }

    audioPreviewUrlByMessageId.value = {
      ...audioPreviewUrlByMessageId.value,
      [messageId]: nextUrl
    };
  }

  function releaseAudioPreviewUrl(messageId: string) {
    const currentUrl = audioPreviewUrlByMessageId.value[messageId];
    if (!currentUrl) {
      return;
    }

    if (isBlobUrl(currentUrl)) {
      URL.revokeObjectURL(currentUrl);
    }

    const next = { ...audioPreviewUrlByMessageId.value };
    delete next[messageId];
    audioPreviewUrlByMessageId.value = next;
  }

  function revokeAllImagePreviewUrls() {
    const cacheEntries = Object.entries(imagePreviewUrlByMessageId.value);
    for (const [, cachedUrl] of cacheEntries) {
      if (isBlobUrl(cachedUrl)) {
        URL.revokeObjectURL(cachedUrl);
      }
    }
    imagePreviewUrlByMessageId.value = {};
  }

  function revokeAllAudioPreviewUrls() {
    const cacheEntries = Object.entries(audioPreviewUrlByMessageId.value);
    for (const [, cachedUrl] of cacheEntries) {
      if (isBlobUrl(cachedUrl)) {
        URL.revokeObjectURL(cachedUrl);
      }
    }
    audioPreviewUrlByMessageId.value = {};
  }

  function setMediaActionLoading(messageId: string, loading: boolean) {
    if (loading) {
      mediaActionLoadingByMessageId.value = {
        ...mediaActionLoadingByMessageId.value,
        [messageId]: true
      };
      return;
    }

    if (!mediaActionLoadingByMessageId.value[messageId]) {
      return;
    }

    const next = { ...mediaActionLoadingByMessageId.value };
    delete next[messageId];
    mediaActionLoadingByMessageId.value = next;
  }

  function isMediaActionLoading(messageId: string) {
    return Boolean(mediaActionLoadingByMessageId.value[messageId]);
  }

  // F1 — REPONTADO (nao verbatim). O legado montava a URL literal do BFF
  // (`/api/bff/conversations/.../media`) e disparava um fetch cru com
  // Authorization / x-selected-tenant-slug / headers de simulacao montados na mao.
  //
  // Agora vai pelo apiFetch (prefixo /v1/omnichannel). Motivo: um fetch cru NAO
  // recebe o X-Account-Id, que e injetado pelo provider global no createApiRequest
  // (plugins/account-id-bridge.client.ts). Ou o request ia sem escopo de conta, ou
  // o header seria montado na mao aqui — que e a segunda fonte de conta proibida
  // (project_account_source_divergence). O apiFetch resolve os dois.
  //
  // Comportamento preservado: devolve Blob e estoura Error com o mesmo texto.
  // O endpoint de midia so existe a partir da F6 — ate la, 404 (esperado na F1).
  async function fetchMessageMediaBlob(messageEntry: Message, disposition: "inline" | "attachment") {
    const conversationId = encodeURIComponent(messageEntry.conversationId);
    const messageId = encodeURIComponent(messageEntry.id);

    try {
      return await apiFetch<Blob>(
        `/conversations/${conversationId}/messages/${messageId}/media?disposition=${disposition}`,
        { responseType: "blob" }
      );
    } catch (error: unknown) {
      const statusCode = error instanceof ApiClientError ? error.statusCode : 0;
      throw new Error(`Falha ao carregar midia (${statusCode})`);
    }
  }

  function triggerDownloadFromUrl(url: string, fileName: string) {
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = fileName;
    anchor.target = "_blank";
    anchor.rel = "noopener noreferrer";
    anchor.click();
  }

  async function requestImagePreview(messageEntry: Message) {
    if (options.resolveMessageType(messageEntry) !== "IMAGE") {
      return;
    }

    if (shouldUseDirectImagePreviewUrl(messageEntry)) {
      clearImageFailed(messageEntry.id);
      return;
    }

    if (getMessageImagePreviewUrl(messageEntry)) {
      return;
    }

    if (isImagePreviewLoading(messageEntry.id)) {
      return;
    }

    setImagePreviewLoading(messageEntry.id, true);
    try {
      const blob = await fetchMessageMediaBlob(messageEntry, "inline");
      const objectUrl = URL.createObjectURL(blob);
      setImagePreviewUrl(messageEntry.id, objectUrl);
      clearImageFailed(messageEntry.id);
    } catch {
      markImageFailed(messageEntry.id);
      releaseImagePreviewUrl(messageEntry.id);
    } finally {
      setImagePreviewLoading(messageEntry.id, false);
    }
  }

  async function requestAudioPreview(messageEntry: Message) {
    const messageType = options.resolveMessageType(messageEntry);
    const supportsAudioPreview = messageType === "AUDIO" || isAudioDocumentMessage(messageEntry);
    if (!supportsAudioPreview) {
      return;
    }

    if (shouldUseDirectAudioPreviewUrl(messageEntry)) {
      return;
    }

    if (getMessageAudioPreviewUrl(messageEntry)) {
      return;
    }

    if (isAudioPreviewLoading(messageEntry.id)) {
      return;
    }

    setAudioPreviewLoading(messageEntry.id, true);
    try {
      const blob = await fetchMessageMediaBlob(messageEntry, "inline");
      const normalizedBlobType = normalizeMimeType(blob.type);
      const resolvedAudioMimeType = resolvePlayableAudioMimeType(messageEntry, blob.type);
      const needsMimeOverride =
        !normalizedBlobType ||
        normalizedBlobType === "application/octet-stream" ||
        (Boolean(resolvedAudioMimeType) && normalizedBlobType !== normalizeMimeType(resolvedAudioMimeType));
      const playbackBlob = needsMimeOverride && resolvedAudioMimeType
        ? new Blob([blob], { type: resolvedAudioMimeType })
        : blob;
      const objectUrl = URL.createObjectURL(playbackBlob);
      setAudioPreviewUrl(messageEntry.id, objectUrl);
    } catch {
      releaseAudioPreviewUrl(messageEntry.id);
    } finally {
      setAudioPreviewLoading(messageEntry.id, false);
    }
  }

  async function openMessageMedia(messageEntry: Message) {
    const fallbackUrl = getMessageMediaUrl(messageEntry);
    const popup = window.open("", "_blank", "noopener,noreferrer");
    setMediaActionLoading(messageEntry.id, true);

    try {
      const blob = await fetchMessageMediaBlob(messageEntry, "inline");
      const objectUrl = URL.createObjectURL(blob);

      if (popup) {
        popup.location.href = objectUrl;
      } else {
        window.open(objectUrl, "_blank", "noopener,noreferrer");
      }

      window.setTimeout(() => {
        URL.revokeObjectURL(objectUrl);
      }, 60_000);
    } catch {
      if (popup) {
        if (fallbackUrl) {
          popup.location.href = fallbackUrl;
        } else {
          popup.close();
        }
      } else if (fallbackUrl) {
        window.open(fallbackUrl, "_blank", "noopener,noreferrer");
      }
    } finally {
      setMediaActionLoading(messageEntry.id, false);
    }
  }

  async function downloadMessageMedia(messageEntry: Message) {
    const fallbackUrl = getMessageMediaUrl(messageEntry);
    const downloadName = buildMediaDownloadName(messageEntry);
    setMediaActionLoading(messageEntry.id, true);

    try {
      const blob = await fetchMessageMediaBlob(messageEntry, "attachment");
      const objectUrl = URL.createObjectURL(blob);
      triggerDownloadFromUrl(objectUrl, downloadName);
      window.setTimeout(() => {
        URL.revokeObjectURL(objectUrl);
      }, 60_000);
    } catch {
      if (fallbackUrl) {
        triggerDownloadFromUrl(fallbackUrl, downloadName);
      }
    } finally {
      setMediaActionLoading(messageEntry.id, false);
    }
  }

  function resolveMediaExtensionByMime(messageEntry: Message) {
    const mimeType = messageEntry.mediaMimeType?.trim().toLowerCase() || "";
    const mimeExtensionMap: Record<string, string> = {
      "image/jpeg": ".jpg",
      "image/jpg": ".jpg",
      "image/png": ".png",
      "image/webp": ".webp",
      "image/heic": ".heic",
      "image/heif": ".heif",
      "video/mp4": ".mp4",
      "video/quicktime": ".mov",
      "video/webm": ".webm",
      "audio/ogg": ".ogg",
      "audio/opus": ".opus",
      "audio/mp3": ".mp3",
      "audio/mpeg": ".mp3",
      "audio/mp4": ".m4a",
      "audio/aac": ".aac",
      "application/pdf": ".pdf",
      "application/zip": ".zip",
      "application/x-rar-compressed": ".rar",
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document": ".docx",
      "application/msword": ".doc",
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet": ".xlsx",
      "application/vnd.ms-excel": ".xls",
      "application/vnd.openxmlformats-officedocument.presentationml.presentation": ".pptx",
      "application/vnd.ms-powerpoint": ".ppt",
      "text/plain": ".txt",
      "text/csv": ".csv"
    };

    if (mimeType && mimeExtensionMap[mimeType]) {
      return mimeExtensionMap[mimeType];
    }

    const type = options.resolveMessageType(messageEntry);
    if (type === "IMAGE") {
      return ".jpg";
    }
    if (type === "VIDEO") {
      return ".mp4";
    }
    if (type === "AUDIO") {
      return ".ogg";
    }

    return ".bin";
  }

  function sanitizeEncryptedFileName(fileName: string, messageEntry: Message) {
    const trimmed = fileName.trim();
    if (!trimmed) {
      return "";
    }

    if (!/\.enc(?:[?#].*)?$/i.test(trimmed)) {
      return trimmed;
    }

    const withoutEnc = trimmed
      .replace(/[?#].*$/, "")
      .replace(/\.enc$/i, "")
      .trim()
      .replace(/[.\-_ ]+$/g, "");

    if (!withoutEnc) {
      return `arquivo${resolveMediaExtensionByMime(messageEntry)}`;
    }

    if (/\.[a-z0-9]{2,6}$/i.test(withoutEnc)) {
      return withoutEnc;
    }

    return `${withoutEnc}${resolveMediaExtensionByMime(messageEntry)}`;
  }

  function resolveMessageFileName(messageEntry: Message) {
    const rawName = messageEntry.mediaFileName?.trim();
    if (rawName) {
      return sanitizeEncryptedFileName(rawName, messageEntry);
    }

    return "";
  }

  function buildMediaDownloadName(messageEntry: Message) {
    const normalized = resolveMessageFileName(messageEntry);
    if (normalized) {
      return normalized;
    }

    const type = options.resolveMessageType(messageEntry).toLowerCase();
    return `${type}-${messageEntry.id}${resolveMediaExtensionByMime(messageEntry)}`;
  }

  function disposeMediaActions() {
    revokeAllImagePreviewUrls();
    revokeAllAudioPreviewUrls();
    imagePreviewLoadingByMessageId.value = {};
    audioPreviewLoadingByMessageId.value = {};
    mediaActionLoadingByMessageId.value = {};
  }

  return {
    markImageFailed,
    isImageFailed,
    getMessageMediaUrl,
    getMessageAudioPreviewUrl,
    getMessageImagePreviewUrl,
    isImagePreviewLoading,
    isAudioPreviewLoading,
    requestImagePreview,
    requestAudioPreview,
    isMediaActionLoading,
    openMessageMedia,
    downloadMessageMedia,
    resolveMessageFileName,
    disposeMediaActions
  };
}

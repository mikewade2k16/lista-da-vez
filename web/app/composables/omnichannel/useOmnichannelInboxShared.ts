import type {
  Contact,
  Conversation,
  GroupParticipant,
  Message,
  MessageType,
  UserRole
} from "~/types";

export interface MessagesPageResponse {
  conversationId: string;
  messages: Message[];
  hasMore: boolean;
}

export interface SyncConversationHistoryResponse {
  conversationId: string;
  externalId: string;
  conversationLastMessageAt?: string | null;
  queryVariant?: string;
  queryAttempts?: number;
  queryCandidates?: number;
  fetchedCount: number;
  selectedCount: number;
  processedCount: number;
  createdCount: number;
  deduplicatedCount: number;
  ignoredCount: number;
  failedCount: number;
  firstFailureMessage: string | null;
}

export interface SyncOpenConversationsResponse {
  instanceName: string;
  fetchedChatsCount: number;
  selectedChatsCount: number;
  createdCount: number;
  updatedCount: number;
  skippedCount: number;
  totalConversationsAfterSync: number;
  message?: string;
  degraded?: boolean;
}

export interface GroupParticipantsResponse {
  conversationId: string;
  participants: GroupParticipant[];
}

export interface SaveContactResponse {
  contact: Contact | null;
  conversation: Conversation | null;
}

export type WhatsAppContactImportAction = "create" | "update" | "skip";

export interface WhatsAppContactImportItem {
  phone: string;
  remoteJid: string | null;
  name: string;
  avatarUrl: string | null;
  existingContactId: string | null;
  existingName: string | null;
  existingAvatarUrl: string | null;
  action: WhatsAppContactImportAction;
  reason: string;
}

export interface WhatsAppContactImportSummary {
  totalProviderRecords: number;
  candidates: number;
  create: number;
  update: number;
  skip: number;
  invalid: number;
  selected: number;
}

export interface WhatsAppContactImportApplied {
  created: number;
  updated: number;
  skipped: number;
  failed: number;
  linkedConversations: number;
  failures: Array<{
    phone: string;
    reason: string;
  }>;
}

export interface WhatsAppContactImportResponse {
  dryRun: boolean;
  generatedAt: string;
  summary: WhatsAppContactImportSummary;
  items: WhatsAppContactImportItem[];
  applied: WhatsAppContactImportApplied | null;
  warnings: string[];
}

export interface ReadStateEntry {
  lastReadAt: string | null;
  lastReadMessageId: string | null;
}

export interface OutboundAttachment {
  file: File;
  type: MessageType;
  previewUrl: string | null;
  durationSeconds?: number | null;
  sendAsSticker?: boolean;
  sendAsVoiceNote?: boolean;
}

export interface OptimisticMessageOptions {
  conversationId: string;
  text: string;
  attachment: OutboundAttachment | null;
  reply: Message | null;
}

export type AttachmentPickerMode = "document" | "media" | "camera" | "audio" | "voice" | "sticker" | "gif";

export interface AttachmentSelectionPayload {
  file: File | null;
  mode: AttachmentPickerMode;
  durationSeconds?: number | null;
}

export interface SendMessageOptions {
  mentionedJids?: string[];
  linkPreviewEnabled?: boolean;
}

export interface MentionOpenPayload {
  jid: string | null;
  phone: string | null;
  label: string | null;
}

export interface CreateContactPayload {
  name: string;
  phone: string;
  countryCode?: string | null;
}

export type InboxSidebarView = "conversations" | "contacts";

export const UNASSIGNED_VALUE = "__unassigned__";
export const MESSAGE_PAGE_SIZE = 50;
export const DEFAULT_MAX_UPLOAD_MB = 500;
export const MEDIA_MESSAGE_TYPES: MessageType[] = ["IMAGE", "AUDIO", "VIDEO", "DOCUMENT"];

const MEDIA_TYPE_LABEL: Record<MessageType, string> = {
  TEXT: "texto",
  IMAGE: "imagem",
  AUDIO: "audio",
  VIDEO: "video",
  DOCUMENT: "documento"
};

export function toArrayOrEmpty<T>(value: unknown) {
  if (Array.isArray(value)) {
    return value as T[];
  }

  return [];
}

export function canWriteInboxByRole(role: UserRole | null | undefined) {
  return role === "ADMIN" || role === "SUPERVISOR" || role === "AGENT";
}

export function buildDateKey(value: string) {
  const date = new Date(value);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

export function formatDateHeader(value: string) {
  const date = new Date(value);
  const today = new Date();
  const yesterday = new Date();
  yesterday.setDate(today.getDate() - 1);

  if (buildDateKey(date.toISOString()) === buildDateKey(today.toISOString())) {
    return "Hoje";
  }

  if (buildDateKey(date.toISOString()) === buildDateKey(yesterday.toISOString())) {
    return "Ontem";
  }

  return new Intl.DateTimeFormat("pt-BR", {
    day: "2-digit",
    month: "short",
    year: "numeric"
  }).format(date);
}

export function isNearBottom(element: HTMLElement | null) {
  if (!element) {
    return false;
  }

  const distance = element.scrollHeight - (element.scrollTop + element.clientHeight);
  return distance < 48;
}

export function resolveMessageType(messageEntry: Message): MessageType {
  return messageEntry.messageType ?? "TEXT";
}

export function normalizeComparableJid(value: string | null | undefined) {
  const trimmed = value?.trim().toLowerCase();
  if (!trimmed) {
    return null;
  }

  if (trimmed.includes("@lid")) {
    return trimmed;
  }

  if (trimmed.includes("@")) {
    return trimmed
      .replace(/:\d+(?=@)/, "")
      .replace(/@c\.us$/, "@s.whatsapp.net");
  }

  const digits = trimmed.replace(/\D/g, "");
  if (!digits) {
    return null;
  }

  return `${digits}@s.whatsapp.net`;
}

export function normalizePhoneDigits(value: string | null | undefined) {
  const digits = value?.replace(/\D/g, "") ?? "";
  return digits.length > 0 ? digits : null;
}

export function buildCanonicalContactPhone(payload: {
  phone: string | null | undefined;
  countryCode?: string | null | undefined;
}) {
  const phoneDigits = normalizePhoneDigits(payload.phone);
  if (!phoneDigits) {
    return null;
  }

  const countryCodeDigits = normalizePhoneDigits(payload.countryCode);
  if (countryCodeDigits) {
    if (phoneDigits.startsWith(countryCodeDigits)) {
      return phoneDigits;
    }

    return `${countryCodeDigits}${phoneDigits}`;
  }

  if (phoneDigits.startsWith("55") && phoneDigits.length >= 12) {
    return phoneDigits;
  }

  if (phoneDigits.length >= 10 && phoneDigits.length <= 11) {
    return `55${phoneDigits}`;
  }

  return phoneDigits;
}

export function asRecord(value: unknown) {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }

  return value as Record<string, unknown>;
}

export function toOptionalNumber(value: unknown) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return null;
  }

  return value;
}

export function normalizeTenantUploadLimitMb(value: unknown) {
  if (typeof value !== "number" || !Number.isFinite(value)) {
    return DEFAULT_MAX_UPLOAD_MB;
  }

  const normalized = Math.trunc(value);
  if (normalized < 1) {
    return 1;
  }

  if (normalized > 2048) {
    return 2048;
  }

  return normalized;
}

export function formatBytesAsMb(value: number | null | undefined) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) {
    return null;
  }

  return `${Math.round((value / (1024 * 1024)) * 10) / 10}MB`;
}

export function formatUploadApiError(errorCode: string | null, details: Record<string, unknown> | null, fallback: string) {
  if (!errorCode) {
    return fallback;
  }

  const messageType = typeof details?.messageType === "string" ? details.messageType : "DOCUMENT";
  const typeLabel = MEDIA_TYPE_LABEL[messageType as MessageType] ?? "arquivo";
  const maxBytes = toOptionalNumber(details?.maxBytes);
  const maxUploadMbValue = toOptionalNumber(details?.maxUploadMb);
  const maxUploadMb = maxUploadMbValue !== null ? `${maxUploadMbValue}MB` : null;
  const maxMb = formatBytesAsMb(maxBytes);
  const actualBytes = toOptionalNumber(details?.actualBytes);
  const actualMb = formatBytesAsMb(actualBytes);

  if (errorCode === "UPLOAD_LIMIT_EXCEEDED") {
    return `Arquivo acima do limite configurado para ${typeLabel}${maxUploadMb ? ` (${maxUploadMb})` : maxMb ? ` (${maxMb})` : ""}${actualMb ? `. Enviado: ${actualMb}.` : "."}`;
  }

  if (errorCode === "UPLOAD_MIME_INVALID") {
    return `Tipo de arquivo invalido para ${typeLabel}.`;
  }

  if (errorCode === "UPLOAD_SIZE_REQUIRED") {
    return "Nao foi possivel identificar o tamanho do arquivo para validar limite.";
  }

  return fallback;
}

export function formatSendError(error: unknown, fallback = "Nao foi possivel enviar a mensagem.") {
  const errorRecord = asRecord(error);
  const directData = asRecord(errorRecord?.data);
  const directCode = typeof directData?.code === "string" ? directData.code : null;

  if (directCode) {
    const formatted = formatUploadApiError(
      directCode,
      asRecord(directData?.details),
      typeof directData?.message === "string" ? directData.message : fallback
    );
    if (formatted.trim().length > 0) {
      return formatted;
    }
  }

  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }

  return fallback;
}

export function isMediaMessageType(type: MessageType) {
  return MEDIA_MESSAGE_TYPES.includes(type);
}

export function resolveAttachmentType(file: File, mode: AttachmentPickerMode): MessageType {
  if (mode === "sticker") {
    return "IMAGE";
  }

  if (mode === "voice") {
    return "AUDIO";
  }

  if (mode === "gif") {
    const mime = file.type.toLowerCase();
    return mime.startsWith("video/") ? "VIDEO" : "IMAGE";
  }

  if (mode === "document") {
    return "DOCUMENT";
  }

  const mime = file.type.toLowerCase();

  if (mode === "audio") {
    return "DOCUMENT";
  }

  if (mime.startsWith("image/")) {
    return "IMAGE";
  }

  if (mime.startsWith("video/")) {
    return "VIDEO";
  }

  if (mime.startsWith("audio/")) {
    return "AUDIO";
  }

  return "DOCUMENT";
}

export async function readFileAsDataUrl(file: File) {
  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string" && reader.result.trim().length > 0) {
        resolve(reader.result);
        return;
      }

      reject(new Error("Falha ao ler o arquivo selecionado"));
    };
    reader.onerror = () => {
      reject(new Error("Falha ao ler o arquivo selecionado"));
    };
    reader.readAsDataURL(file);
  });
}

export function extractMentionsFromText(value: string) {
  const text = value.trim();
  if (!text) {
    return null;
  }

  const everyOne = /(^|\s)@(all|todos)(?=\s|$)/i.test(text);
  const mentioned = new Set<string>();
  const regex = /@(\d{7,20})(?=\b)/g;

  let match: RegExpExecArray | null;
  while ((match = regex.exec(text)) !== null) {
    const digits = match[1]?.replace(/\D/g, "");
    if (!digits) {
      continue;
    }
    mentioned.add(`${digits}@s.whatsapp.net`);
  }

  if (!everyOne && mentioned.size === 0) {
    return null;
  }

  return {
    everyOne,
    mentioned: [...mentioned]
  };
}

export function normalizeMentionJid(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }

  if (trimmed.includes("@")) {
    return trimmed.toLowerCase();
  }

  const digits = trimmed.replace(/\D/g, "");
  if (!digits) {
    return null;
  }

  return `${digits}@s.whatsapp.net`;
}

export function normalizeOutboundLinkUrl(value: string) {
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }

  const candidate = trimmed.startsWith("www.") ? `https://${trimmed}` : trimmed;
  try {
    const parsed = new URL(candidate);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return null;
    }
    return parsed.toString();
  } catch {
    return null;
  }
}

export function extractFirstOutboundLinkUrl(value: string) {
  const text = value.trim();
  if (!text) {
    return null;
  }

  const match = text.match(/(?:https?:\/\/|www\.)[^\s<>"']+/i);
  if (!match) {
    return null;
  }

  const candidate = match[0].replace(/[),.!?;:]+$/g, "");
  return normalizeOutboundLinkUrl(candidate);
}

export function mediaPlaceholderByType(type: MessageType) {
  if (type === "IMAGE") {
    return "[imagem]";
  }

  if (type === "AUDIO") {
    return "[audio]";
  }

  if (type === "VIDEO") {
    return "[video]";
  }

  if (type === "DOCUMENT") {
    return "[documento]";
  }

  return "";
}

export function normalizeContactPhone(value: string) {
  const normalized = value.trim().replace(/[^\d+]/g, "");
  return normalized.length >= 7 ? normalized : "";
}

export function normalizeReactionEmoji(value: string | null | undefined) {
  const trimmed = typeof value === "string" ? value.trim() : "";
  if (!trimmed) {
    return null;
  }

  return [...trimmed].slice(0, 8).join("") || null;
}

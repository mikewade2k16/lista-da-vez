export function useInboxChatUtilities() {
  function extractIdentifierPhone(value: string | null | undefined) {
    const trimmed = value?.trim() ?? "";
    if (!trimmed) {
      return "";
    }

    const normalized = trimmed
      .replace(/@s\.whatsapp\.net$/i, "")
      .replace(/@g\.us$/i, "")
      .replace(/@lid$/i, "");

    return normalized.replace(/\D/g, "");
  }

  function isTechnicalWhatsAppIdentifier(value: string | null | undefined) {
    const normalized = value?.trim().toLowerCase() ?? "";
    if (!normalized) {
      return false;
    }

    if (
      normalized.endsWith("@s.whatsapp.net") ||
      normalized.endsWith("@g.us") ||
      normalized.endsWith("@lid")
    ) {
      return true;
    }

    const compact = normalized.replace(/\s+/g, "");
    if (/^\+?\d{7,20}$/.test(compact)) {
      return true;
    }

    return false;
  }

  function sanitizeHumanLabel(
    value: string | null | undefined,
    options?: {
      fallbackPhone?: string | null | undefined;
      fallbackLabel?: string;
    }
  ) {
    const fallbackLabel = options?.fallbackLabel ?? "Participante";
    const candidate = value?.trim() ?? "";

    if (candidate && !isTechnicalWhatsAppIdentifier(candidate)) {
      return candidate;
    }

    const fallbackPhone = extractIdentifierPhone(options?.fallbackPhone ?? candidate);
    if (fallbackPhone) {
      return fallbackPhone;
    }

    return fallbackLabel;
  }

  function escapeHtml(value: string) {
    return value
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/\"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  function escapeHtmlAttribute(value: string) {
    return value
      .replace(/&/g, "&amp;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;");
  }

  function escapeRegExp(value: string) {
    return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  }

  function normalizeDigits(value: string) {
    return value.replace(/\D/g, "");
  }

  function normalizeNameForComparison(value: string) {
    if (!value) {
      return "";
    }

    return value
      .normalize("NFD")
      .replace(/[\u0300-\u036f]/g, "")
      .toLowerCase()
      .replace(/\s+/g, " ")
      .trim();
  }

  function normalizeJidForComparison(value: string) {
    if (!value) {
      return "";
    }

    return value
      .trim()
      .toLowerCase()
      .replace(/:\d+(?=@)/, "")
      .replace(/@c\.us$/, "@s.whatsapp.net");
  }

  function normalizeLinkUrl(value: string | null | undefined) {
    const trimmed = value?.trim();
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

  function extractFirstLinkFromText(value: string | null | undefined) {
    if (!value) {
      return null;
    }

    const match = value.match(/(?:https?:\/\/|www\.)[^\s<>"']+/i);
    if (!match) {
      return null;
    }

    const candidate = match[0].replace(/[),.!?;:]+$/g, "");
    return normalizeLinkUrl(candidate);
  }

  function extractLinksFromText(value: string | null | undefined) {
    if (!value) {
      return [];
    }

    const matches = value.match(/(?:https?:\/\/|www\.)[^\s<>"']+/gi) ?? [];
    const links = new Set<string>();

    for (const raw of matches) {
      const candidate = raw.replace(/[),.!?;:]+$/g, "");
      const normalized = normalizeLinkUrl(candidate);
      if (normalized) {
        links.add(normalized);
      }
    }

    return [...links];
  }

  function extractLinkHost(value: string | null | undefined) {
    if (!value) {
      return null;
    }

    try {
      return new URL(value).hostname.replace(/^www\./i, "");
    } catch {
      return null;
    }
  }

  function formatFileSize(value: number | null | undefined) {
    if (!value || value <= 0) {
      return "0 KB";
    }

    if (value < 1024) {
      return `${value} B`;
    }

    if (value < 1024 * 1024) {
      return `${(value / 1024).toFixed(1)} KB`;
    }

    return `${(value / (1024 * 1024)).toFixed(1)} MB`;
  }

  function asRecord(value: unknown) {
    if (typeof value === "string") {
      const trimmed = value.trim();
      if (!trimmed) {
        return null;
      }

      try {
        const parsed = JSON.parse(trimmed) as unknown;
        if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
          return parsed as Record<string, unknown>;
        }
      } catch {
        return null;
      }
    }

    if (!value || typeof value !== "object" || Array.isArray(value)) {
      return null;
    }

    return value as Record<string, unknown>;
  }

  function splitLegacyReplyContent(value: string) {
    const match = value.match(/^>\s*(.+?)\n\n([\s\S]*)$/);
    if (!match) {
      return null;
    }

    return {
      quoted: match[1].trim(),
      content: match[2].trim()
    };
  }

  function parseUnsupportedLabelFromPlaceholder(value: string | null | undefined) {
    const normalized = value?.trim();
    if (!normalized) {
      return null;
    }

    const match = normalized.match(/^\[conteudo nao suportado:\s*(.+)\]$/i);
    if (!match?.[1]) {
      return null;
    }

    return match[1].trim();
  }

  function isMediaPlaceholder(value: string | null | undefined, placeholders: Set<string>) {
    const normalized = value?.trim().toLowerCase();
    if (!normalized) {
      return false;
    }

    return placeholders.has(normalized);
  }

  function normalizeMentionJid(value: string | null | undefined) {
    const normalized = normalizeJidForComparison((value ?? "").trim().replace(/^@+/, ""));
    if (!normalized) {
      return null;
    }
    return normalized;
  }

  function normalizeMentionLabel(value: string | null | undefined) {
    return value
      ?.trim()
      .replace(/\s+/g, " ")
      .toLowerCase() ?? "";
  }

  return {
    extractIdentifierPhone,
    isTechnicalWhatsAppIdentifier,
    sanitizeHumanLabel,
    escapeHtml,
    escapeHtmlAttribute,
    escapeRegExp,
    normalizeDigits,
    normalizeNameForComparison,
    normalizeJidForComparison,
    normalizeLinkUrl,
    extractFirstLinkFromText,
    extractLinksFromText,
    extractLinkHost,
    formatFileSize,
    asRecord,
    splitLegacyReplyContent,
    parseUnsupportedLabelFromPlaceholder,
    isMediaPlaceholder,
    normalizeMentionJid,
    normalizeMentionLabel
  };
}

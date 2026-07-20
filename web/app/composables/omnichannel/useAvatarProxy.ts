import { getApiBase } from "~/utils/api-client";

// F1 — REPONTADO (nao verbatim). O legado proxiava o avatar por uma rota Nitro
// same-origin (`/api/avatar/whatsapp`). O web do Omni nao tem Nitro (BFF
// eliminado em 2026-07-02, ADR 0002), entao a rota passa a ser o Go.
//
// F12/C4: a URL sai daqui DIRETO para `<img src>`, que o navegador carrega SEM
// token — sob o gate de auth daria 401 em toda foto. Por isso o endpoint e
// PUBLICO: `/v1/public/omnichannel/avatar` (fora do gate, allowlist dos 4 hosts
// do WhatsApp + rate-limit + anti-SSRF no back). Por ser outra origem, a URL e
// absoluta. Comportamento preservado: so hosts do WhatsApp sao proxiados; o
// resto passa direto.

const WHATSAPP_AVATAR_PROXY_HOSTS = new Set([
  "pps.whatsapp.net",
  "mmg.whatsapp.net",
  "mmx.whatsapp.net",
  "lookaside.whatsapp.com"
]);

const AVATAR_PROXY_PATH = "/v1/public/omnichannel/avatar";

export function resolveAvatarSource(rawUrl: string | null | undefined) {
  const normalized = rawUrl?.trim() ?? "";
  if (!normalized) {
    return undefined;
  }

  // Ja proxiado: devolve como esta (idempotente, como no legado).
  if (normalized.includes(AVATAR_PROXY_PATH)) {
    return normalized;
  }

  let parsedUrl: URL;
  try {
    parsedUrl = new URL(normalized);
  } catch {
    return normalized;
  }

  if (!WHATSAPP_AVATAR_PROXY_HOSTS.has(parsedUrl.hostname.toLowerCase())) {
    return normalized;
  }

  const base = getApiBase(useRuntimeConfig()).replace(/\/$/, "");
  const encodedUrl = encodeURIComponent(normalized);
  return `${base}${AVATAR_PROXY_PATH}?url=${encodedUrl}`;
}

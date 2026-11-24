// Funções puras da fase de segurança: avaliação de headers, adulteração de JWT,
// heurísticas de CORS/listagem de diretório, varredura de segredos e parsing da
// AUDITORIA. Sem rede nem banco, para poderem ser testadas com node --test.
import { readFileSync } from "node:fs";

// Estados do catálogo (obs.security_checks) <-> status dos results do runner.
export const TO_RESULT = {
  pass: "pass",
  partial: "warn",
  fail: "fail",
  not_tested: "skip",
};

export function loadCatalog(file) {
  const j = JSON.parse(readFileSync(file, "utf8"));
  const seen = new Set();
  for (const c of j.checks) {
    for (const f of ["key", "category", "title", "standard", "howTested"])
      if (typeof c[f] !== "string" || !c[f])
        throw new Error(`catálogo: ${c.key || "?"} sem ${f}`);
    if (seen.has(c.key)) throw new Error(`catálogo: chave repetida ${c.key}`);
    seen.add(c.key);
  }
  return j.checks;
}

// ---------- headers ----------
export function evalHeaders(headers, { https = false } = {}) {
  const h = (n) => headers.get?.(n) ?? headers[n.toLowerCase()] ?? "";
  const csp = h("content-security-policy");
  const xfo = h("x-frame-options");
  const missing = [];
  if (!csp) missing.push("Content-Security-Policy");
  if (!/nosniff/i.test(h("x-content-type-options")))
    missing.push("X-Content-Type-Options");
  const frame =
    /frame-ancestors/i.test(csp) || /^(deny|sameorigin)$/i.test(xfo);
  if (!frame) missing.push("frame-ancestors/X-Frame-Options");
  if (!h("referrer-policy")) missing.push("Referrer-Policy");
  if (https && !/max-age=\d+/i.test(h("strict-transport-security")))
    missing.push("Strict-Transport-Security");
  return { missing, frame };
}

// ---------- JWT ----------
const b64u = (b) => Buffer.from(b).toString("base64url");
export function jwtParts(token) {
  const p = String(token).split(".");
  return p.length === 3 ? p : null;
}
// Assinatura trocada por outra do mesmo tamanho.
export function tamperSignature(token) {
  const p = jwtParts(token);
  if (!p) return null;
  const sig = p[2];
  const flipped = [...sig]
    .map((c) => (c === "A" ? "B" : "A"))
    .join("")
    .slice(0, sig.length);
  return `${p[0]}.${p[1]}.${flipped === sig ? "A" + sig.slice(1) : flipped}`;
}
// Payload alterado (claim extra) mantendo a assinatura original.
export function tamperPayload(token) {
  const p = jwtParts(token);
  if (!p) return null;
  try {
    const payload = JSON.parse(Buffer.from(p[1], "base64url").toString());
    payload.obs_tamper = 1;
    return `${p[0]}.${b64u(JSON.stringify(payload))}.${p[2]}`;
  } catch {
    return null;
  }
}
export function algNone(token) {
  const p = jwtParts(token);
  if (!p) return null;
  const head = b64u(JSON.stringify({ alg: "none", typ: "JWT" }));
  return `${head}.${p[1]}.`;
}

// ---------- CORS ----------
export function corsVerdict(origin, h) {
  const acao = h("access-control-allow-origin");
  const cred = /^true$/i.test(h("access-control-allow-credentials"));
  if (!acao) return { ok: true, why: "sem Access-Control-Allow-Origin" };
  if (acao === origin)
    return {
      ok: false,
      why: `Origin ${origin} refletido${cred ? " com credenciais" : ""}`,
    };
  if (acao === "*" && cred)
    return { ok: false, why: "* combinado com credenciais" };
  return { ok: true, why: `ACAO=${acao}` };
}

// ---------- open redirect ----------
// Location que leva a outro hostname que não o da aplicação (porta é ignorada:
// a API redireciona legitimamente para o web).
export function redirectsExternal(location, base, allowed = []) {
  if (!location) return false;
  try {
    const host = new URL(location, base).hostname;
    return ![new URL(base).hostname, ...allowed].includes(host);
  } catch {
    return false;
  }
}

// ---------- uploads ----------
export function isDirectoryListing(contentType, body) {
  if (!/html|text\/plain/i.test(contentType)) return false;
  return (
    (/<pre>/i.test(body) && (body.match(/<a href=/gi) || []).length >= 1) ||
    /Index of \//i.test(body)
  );
}
export const leaksPasswd = (body) => /root:[x*!]?:0:0:/.test(body);

// ---------- XSS refletido ----------
export function unescapedReflection(contentType, body, marker) {
  if (!/html/i.test(contentType)) return false;
  return body.includes(`<script>${marker}</script>`);
}

// ---------- segredos ----------
// "high" = formato inconfundível de credencial; "heuristic" = atribuição literal.
export const SECRET_RULES = [
  { id: "aws-access-key", level: "high", re: /\bAKIA[0-9A-Z]{16}\b/ },
  { id: "github-token", level: "high", re: /\bgh[pousr]_[A-Za-z0-9]{36,}\b/ },
  { id: "github-pat", level: "high", re: /\bgithub_pat_[A-Za-z0-9_]{50,}\b/ },
  { id: "slack-token", level: "high", re: /\bxox[baprs]-[A-Za-z0-9-]{10,}\b/ },
  {
    id: "private-key",
    level: "high",
    re: /-----BEGIN (?:RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY(?: BLOCK)?-----/,
  },
  { id: "openai-key", level: "high", re: /\bsk-[A-Za-z0-9_-]{32,}\b/ },
  { id: "google-api-key", level: "high", re: /\bAIza[0-9A-Za-z_-]{35}\b/ },
  {
    id: "password-literal",
    level: "heuristic",
    re: /\b(?:password|passwd|secret|api[_-]?key)\b["']?\s*[:=]\s*["']([^"'\s$<{(\\]{8,})["']/i,
  },
];
const PLACEHOLDER =
  /change|example|placeholder|your[-_]|xxx|\*{3}|dummy|sample|fake|test|dev|local|omni_|secret$|password$|<|\.\.\./i;
export function isIgnoredPath(f) {
  return (
    /\.(example|sample|md|png|jpe?g|gif|webp|svg|ico|woff2?|ttf|lock|zip|pdf|mp4|webm)$/i.test(
      f,
    ) ||
    /(^|\/)(node_modules|vendor|\.output|\.nuxt|dist|fixtures?|__snapshots__|docs)\//.test(
      f,
    ) ||
    /(_test\.go|\.test\.[mc]?[jt]s|\.spec\.[mc]?[jt]s)$/.test(f) ||
    /(^|\/)(package-lock\.json|go\.sum|inventory\.json)$/.test(f) ||
    /(^|\/)observability\/(runs|reports)\//.test(f)
  );
}
// Retorna [{line, rule, level}] sem nunca devolver o valor encontrado.
export function scanText(text) {
  const out = [];
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    if (lines[i].length > 2000) continue;
    for (const r of SECRET_RULES) {
      const m = r.re.exec(lines[i]);
      if (!m) continue;
      if (r.level === "heuristic" && PLACEHOLDER.test(m[1])) continue;
      out.push({ line: i + 1, rule: r.id, level: r.level });
      break;
    }
  }
  return out;
}

// ---------- AUDITORIA ----------
const SEV = {
  critico: "critical",
  crítico: "critical",
  alto: "high",
  medio: "medium",
  médio: "medium",
  baixo: "low",
};
export function normalizeSeverity(label, id = "") {
  const k = String(label || "")
    .trim()
    .toLowerCase();
  if (SEV[k]) return SEV[k];
  if (["critical", "high", "medium", "low"].includes(k)) return k;
  return { C: "critical", A: "high", M: "medium", B: "low" }[id[0]] || "low";
}
export function areaOf(location) {
  const f = location.replace(/:\d+.*$/, "");
  let m = f.match(/^back\/internal\/modules\/([^/]+)/);
  if (m) return m[1];
  m = f.match(/^back\/internal\/platform\/([^/]+)/);
  if (m) return `platform/${m[1]}`;
  if (/^back\//.test(f)) return "back";
  m = f.match(/^web\/layers\/([^/]+)/);
  if (m) return `web/${m[1]}`;
  if (/^web\//.test(f)) return "web";
  if (/^\.github\//.test(f)) return "ci";
  if (/docker|compose|scripts\/|Caddy/i.test(f)) return "infra";
  return "geral";
}
// Linha "**Status ...**" da AUDITORIA: "corrigido" vira fixed, "parcial" segue open.
export function findingStatus(body) {
  const line = body.split("\n").find((l) => /^\s*-\s*\*\*Status\b/i.test(l));
  if (!line) return "open";
  const head = line.slice(0, 160);
  if (/parcial/i.test(head)) return "open";
  return /corrigido/i.test(head) ? "fixed" : "open";
}

export function parseFindings(md) {
  const out = [];
  let section = "";
  const blocks = md.split(/\n(?=#{3,4} )/);
  for (const b of blocks) {
    const sec = b.match(/^### \d+\.\d+ (\S+)/);
    if (sec) section = sec[1];
    for (const part of b.split(/\n(?=#### )/)) {
      const h = part.match(/^#### ([A-Z]-\d+) — (.+)$/m);
      if (!h) continue;
      const [, id, rawTitle] = h;
      const title = rawTitle.replace(/`/g, "").trim();
      const body = part.slice(part.indexOf("\n") + 1);
      const loc = body
        .split("\n")
        .slice(0, 3)
        .join("\n")
        .match(/`([\w./@-]+\.[\w]+(?::\d+)?)/);
      const location = loc ? loc[1] : "";
      out.push({
        id,
        severity: normalizeSeverity(section, id),
        title,
        area: areaOf(location),
        location,
        status: findingStatus(body),
      });
    }
  }
  return out;
}

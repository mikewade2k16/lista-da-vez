// Utilitários compartilhados do runner: argumentos, estatística, formato de
// forma de JSON, matcher de rotas, limitador de concorrência e taxa.
import { execFile } from "node:child_process";

export function parseArgs(argv) {
  const out = { _: [], flags: {} };
  for (const a of argv) {
    const m = a.match(/^--([^=]+)(?:=(.*))?$/);
    if (!m) {
      out._.push(a);
      continue;
    }
    const [, k, v] = m;
    if (v === undefined) out.flags[k] = true;
    else if (out.flags[k] === undefined) out.flags[k] = v;
    else out.flags[k] = [].concat(out.flags[k], v);
  }
  return out;
}
export const list = (v) =>
  v === undefined || v === true
    ? []
    : []
        .concat(v)
        .flatMap((s) => String(s).split(","))
        .map((s) => s.trim())
        .filter(Boolean);

export function runId(d = new Date()) {
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}-${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`;
}

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
export const round = (n, d = 1) =>
  n == null || Number.isNaN(n) ? null : Math.round(n * 10 ** d) / 10 ** d;

export function percentile(sorted, p) {
  if (!sorted.length) return null;
  const idx = Math.min(
    sorted.length - 1,
    Math.max(0, Math.ceil((p / 100) * sorted.length) - 1),
  );
  return sorted[idx];
}
export function stats(samples) {
  const s = [...samples].sort((a, b) => a - b);
  return {
    n: s.length,
    min: round(s[0]),
    p50: round(percentile(s, 50)),
    p95: round(percentile(s, 95)),
    p99: round(percentile(s, 99)),
    max: round(s[s.length - 1]),
  };
}

// Limitador de concorrência (como p-limit).
export function pool(max) {
  let active = 0;
  const queue = [];
  const next = () => {
    if (active >= max || !queue.length) return;
    active++;
    const { fn, resolve, reject } = queue.shift();
    fn()
      .then(resolve, reject)
      .finally(() => {
        active--;
        next();
      });
  };
  return (fn) =>
    new Promise((resolve, reject) => {
      queue.push({ fn, resolve, reject });
      next();
    });
}
export async function mapPool(items, max, fn) {
  const run = pool(max);
  return Promise.all(items.map((it, i) => run(() => fn(it, i))));
}

// Espaça requisições para respeitar o rate limit (reserva de slots sequenciais).
export function rateLimiter(perMin) {
  const gap = 60000 / perMin;
  let nextAt = 0;
  return async () => {
    const now = Date.now();
    const at = Math.max(now, nextAt);
    nextAt = at + gap;
    if (at > now) await sleep(at - now);
  };
}

// Assinatura de forma: mapa caminho -> tipo, profundidade 3, array pelo 1º item.
export function shapeOf(value, maxDepth = 3, cap = 250) {
  const out = {};
  const typeOf = (v) =>
    v === null ? "null" : Array.isArray(v) ? "array" : typeof v;
  const walk = (v, path, depth) => {
    if (Object.keys(out).length >= cap) return;
    const t = typeOf(v);
    if (t === "object" && depth < maxDepth) {
      out[path || "$"] = "object";
      for (const k of Object.keys(v).sort())
        walk(v[k], path ? `${path}.${k}` : k, depth + 1);
    } else if (t === "array") {
      out[path || "$"] = "array";
      if (v.length && depth < maxDepth) walk(v[0], `${path}[]`, depth + 1);
    } else out[path || "$"] = t;
  };
  walk(value, "", 0);
  return out;
}
export function shapeDiff(before, after) {
  if (!before || !after) return null;
  const added = [],
    removed = [],
    changed = [];
  for (const k of Object.keys(after)) {
    if (!(k in before)) added.push(k);
    else if (
      before[k] !== after[k] &&
      before[k] !== "null" &&
      after[k] !== "null"
    )
      changed.push(`${k}: ${before[k]} -> ${after[k]}`);
  }
  for (const k of Object.keys(before)) {
    // Chave sumindo só importa se o pai ainda existe (evita ruído de array vazio).
    if (!(k in after)) {
      const parent = k.includes(".") ? k.slice(0, k.lastIndexOf(".")) : null;
      if (!parent || parent in after) removed.push(k);
    }
  }
  return added.length || removed.length || changed.length
    ? { added, removed, changed }
    : null;
}

// Normaliza paths concretos contra os templates do inventário.
export function routeMatcher(endpoints) {
  const compiled = endpoints
    .map((e) => ({
      e,
      re: new RegExp(
        "^" +
          e.path
            .split("/")
            .map((s) =>
              /^\{.+\}$/.test(s)
                ? "[^/]+"
                : s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"),
            )
            .join("/") +
          "/?$",
      ),
      statics: e.path.split("/").filter((s) => !/^\{/.test(s)).length,
    }))
    .sort((a, b) => b.statics - a.statics);
  return (method, path) => {
    const p = path.split("?")[0];
    return (
      compiled.find((c) => c.e.method === method && c.re.test(p))?.e ?? null
    );
  };
}

export function sh(cmd, args, { timeout = 30000, cwd, input } = {}) {
  return new Promise((resolve) => {
    const child = execFile(
      cmd,
      args,
      { timeout, cwd, maxBuffer: 64 * 1024 * 1024 },
      (error, stdout, stderr) => {
        resolve({
          ok: !error,
          code: error?.code ?? 0,
          killed: !!error?.killed,
          stdout: String(stdout),
          stderr: String(stderr),
          error,
        });
      },
    );
    if (input) child.stdin.end(input);
  });
}

export const isLocalHost = (h) =>
  /^(localhost|127\.\d+\.\d+\.\d+|\[?::1\]?|host\.docker\.internal)$/i.test(h);
export const hostOf = (url) => {
  try {
    return new URL(url).hostname;
  } catch {
    return "";
  }
};
export const pad = (s, n) => String(s).padEnd(n);
export const esc = (s) =>
  String(s ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );

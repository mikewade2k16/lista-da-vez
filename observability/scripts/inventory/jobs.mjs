// Jobs em background, workflows n8n, webhooks e integracoes externas.
import path from "node:path";
import {
  ROOT,
  readText,
  rel,
  walk,
  lineAt,
  sortBy,
  uniqSorted,
  readJson,
} from "./util.mjs";
import { findAssignment, moduleOf } from "./go-pkg.mjs";
import { EXTERNALS } from "./go-deps.mjs";

function enclosingFuncName(file, pos) {
  let name = null;
  for (const m of file.masked.matchAll(/^func\s+(?:\([^)]*\)\s*)?(\w+)/gm)) {
    if (m.index > pos) break;
    name = m[1];
  }
  return name || "(anon)";
}

/** Tickers, goroutines de workers, handlers do job worker e workflows n8n. */
export function extractJobs(pkgs, endpoints) {
  const jobs = [];
  for (const file of pkgs.files) {
    for (const m of file.masked.matchAll(/time\.NewTicker\(/g)) {
      const open = m.index + m[0].length - 1;
      const close = file.masked.indexOf(")", open);
      let interval = file.src.slice(open + 1, close).trim() || null;
      if (
        interval &&
        /^[a-z]\w*(Interval|Period|Delay|Every)$/.test(interval)
      ) {
        const def = findAssignment(
          file,
          interval,
          m.index,
          pkgs.byDir.get(file.dir),
        );
        if (def)
          interval = `${interval} = ${def.file.src
            .slice(def.start, def.end)
            .replace(/\s*\/\/.*$/, "")
            .trim()}`;
      }
      jobs.push({
        name: `${enclosingFuncName(file, m.index)}`,
        kind: "ticker",
        module: moduleOf(file.rel),
        file: file.rel,
        line: lineAt(file.src, m.index),
        interval,
      });
    }
    for (const m of file.masked.matchAll(
      /\bgo\s+([A-Za-z_][\w.]*)\.(Run\w*|Start\w*|Loop\w*)\(/g,
    )) {
      jobs.push({
        name: `${m[1]}.${m[2]}`,
        kind: "worker",
        module: moduleOf(file.rel),
        file: file.rel,
        line: lineAt(file.src, m.index),
        interval: null,
      });
    }
    for (const m of file.src.matchAll(/\bworker\.Register\("([\w.]+)"/g)) {
      jobs.push({
        name: `job:${m[1]}`,
        kind: "worker",
        module: moduleOf(file.rel),
        file: file.rel,
        line: lineAt(file.src, m.index),
        interval: null,
      });
    }
  }
  for (const e of endpoints) {
    if (/webhook|callback|ingest/i.test(e.path)) {
      jobs.push({
        name: e.id,
        kind: "webhook",
        module: e.module,
        file: e.file,
        line: e.line,
        interval: null,
      });
    }
  }
  jobs.push(...n8nWorkflows());
  return sortBy(jobs, "kind", "name", "file", "line");
}

function n8nWorkflows() {
  const out = [];
  for (const abs of walk(path.join(ROOT, "automation/export"), (f) =>
    f.endsWith(".json"),
  )) {
    let data;
    try {
      data = readJson(abs);
    } catch {
      continue;
    }
    const wf = Array.isArray(data) ? data[0] : data;
    const triggers = (wf.nodes || [])
      .filter(
        (n) =>
          /webhook|trigger|schedule|cron/i.test(n.type) &&
          !/respondToWebhook/.test(n.type),
      )
      .map(
        (n) =>
          `${n.type.split(".").pop()}${n.parameters?.path ? `:${n.parameters.path}` : ""}`,
      );
    out.push({
      name: wf.name || path.basename(abs, ".json"),
      kind: "n8n",
      module: "automation",
      file: rel(abs),
      line: 1,
      interval: triggers.length ? uniqSorted(triggers).join(",") : null,
    });
  }
  return out;
}

const KIND_BY_ID = {
  meta: "ads",
  n8n: "automation",
  waha: "messaging",
  evolution: "messaging",
  whisper: "ai",
  openai: "ai",
  anthropic: "ai",
  r2: "storage",
  s3: "storage",
  smtp: "mail",
  perola: "erp",
  erp: "erp",
  ftp: "erp",
  redis: "infra",
  google: "oauth",
  tiktok: "social",
};

/** Integracoes: variaveis de ambiente do config.go agrupadas por prefixo + arquivos que as usam. */
export function extractIntegrations(pkgs) {
  const cfgPath = path.join(ROOT, "back/internal/platform/config/config.go");
  const cfg = readText(cfgPath);
  const vars = uniqSorted(
    [...cfg.matchAll(/"([A-Z][A-Z0-9]*_[A-Z0-9_]+)"/g)].map((m) => m[1]),
  );
  const groups = new Map();
  for (const v of vars) {
    const prefix = v.split("_")[0].toLowerCase();
    if (!groups.has(prefix)) groups.set(prefix, []);
    groups.get(prefix).push(v);
  }
  const ids = new Set([...Object.keys(KIND_BY_ID), ...groups.keys()]);
  const result = [];
  for (const id of ids) {
    const envVars = groups.get(id) || [];
    const ext = EXTERNALS.find((e) => e.id === id);
    const files = new Set();
    for (const f of pkgs.files) {
      if (f.rel.endsWith("platform/config/config.go")) continue;
      const usesHttp =
        /http\.(NewRequest|Client|Post|Get)|NewRequestWithContext/.test(
          f.masked,
        );
      if (
        envVars.some((v) => f.src.includes(`"${v}"`)) ||
        (ext && (ext.lit.test(f.src) || (ext.ident.test(f.masked) && usesHttp)))
      )
        files.add(f.rel);
    }
    if (!KIND_BY_ID[id]) continue;
    if (!envVars.length && !files.size) continue;
    result.push({
      id,
      kind: KIND_BY_ID[id] || "config",
      envVars,
      files: [...files].sort(),
    });
  }
  return sortBy(result, "id");
}

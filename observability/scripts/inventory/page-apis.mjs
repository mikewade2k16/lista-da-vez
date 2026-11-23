// Estimativa estatica das APIs que cada pagina usa: grafo de imports + componentes/composables auto-importados.
import path from "node:path";
import fs from "node:fs";
import { ROOT, readText, walk, exists } from "./util.mjs";

const WEB = path.join(ROOT, "web");
const MAX_DEPTH = 6;
const EXTS = [
  "",
  ".vue",
  ".ts",
  ".js",
  ".mjs",
  "/index.vue",
  "/index.ts",
  "/index.js",
];
const API_RE = /(['"`])(\/v[12]\/[^'"`\s]*)\1/g;

let autoIndex = null;

function sourceDirs() {
  const dirs = [path.join(WEB, "app")];
  const layers = path.join(WEB, "layers");
  if (exists(layers))
    for (const l of fs.readdirSync(layers)) dirs.push(path.join(layers, l));
  return dirs;
}

/** Indice de auto-imports: componentes (components.d.ts ou nome de arquivo) e funcoes exportadas em composables/utils/stores. */
function buildAutoIndex() {
  const components = new Map();
  const dts = path.join(WEB, ".nuxt/components.d.ts");
  if (exists(dts)) {
    for (const m of readText(dts).matchAll(
      /export const (\w+): typeof import\("([^"]+)"\)/g,
    )) {
      const target = path.resolve(path.dirname(dts), m[2]);
      if (!target.includes("node_modules") && exists(target))
        components.set(m[1], target);
    }
  }
  const functions = new Map();
  for (const base of sourceDirs()) {
    for (const abs of walk(path.join(base, "components"), (f) =>
      f.endsWith(".vue"),
    )) {
      const name = path.basename(abs, ".vue");
      if (!components.has(name)) components.set(name, abs);
    }
    for (const sub of ["composables", "utils", "stores"]) {
      for (const abs of walk(
        path.join(base, sub),
        (f) => /\.(ts|js)$/.test(f) && !/\.test\./.test(f),
      )) {
        for (const m of readText(abs).matchAll(
          /export\s+(?:default\s+)?(?:async\s+)?(?:function|const)\s+(\w+)/g,
        )) {
          if (!functions.has(m[1])) functions.set(m[1], abs);
        }
      }
    }
  }
  return { components, functions };
}

function resolveSpec(spec, fromFile) {
  let base = null;
  if (spec.startsWith("~/") || spec.startsWith("@/"))
    base = path.join(WEB, "app", spec.slice(2));
  else if (spec.startsWith("~~/")) base = path.join(WEB, spec.slice(3));
  else if (spec.startsWith("."))
    base = path.resolve(path.dirname(fromFile), spec);
  if (!base) return null;
  for (const ext of EXTS) {
    const candidate = base + ext;
    if (exists(candidate) && fs.statSync(candidate).isFile()) return candidate;
  }
  return null;
}

function pascal(tag) {
  return tag.replace(/(^|-)(\w)/g, (_, __, c) => c.toUpperCase());
}

function dependenciesOf(file, src, index) {
  const out = new Set();
  for (const m of src.matchAll(
    /(?:import|export)\s[^'"]*?from\s*['"]([^'"]+)['"]|import\(\s*['"]([^'"]+)['"]\s*\)/g,
  )) {
    const target = resolveSpec(m[1] || m[2], file);
    if (target) out.add(target);
  }
  if (file.endsWith(".vue")) {
    for (const m of src.matchAll(
      /<([A-Z][A-Za-z0-9]+|[a-z]+(?:-[a-z0-9]+)+)[\s/>]/g,
    )) {
      const name = pascal(m[1]).replace(/^Lazy/, "");
      const target =
        index.components.get(name) || index.components.get(`Lazy${name}`);
      if (target) out.add(target);
    }
  }
  for (const m of src.matchAll(/\b(use[A-Z]\w*)\s*\(/g)) {
    const target = index.functions.get(m[1]);
    if (target) out.add(target);
  }
  return out;
}

function methodNear(src, at) {
  const win = src.slice(at, at + 260);
  const m = win.match(/method\s*:\s*['"`](GET|POST|PUT|PATCH|DELETE)['"`]/i);
  return m ? m[1].toUpperCase() : null;
}

function literalsIn(src) {
  const found = [];
  for (const m of src.matchAll(API_RE)) {
    let p = m[2].replace(
      /\$\{\s*([^}]*)\}/g,
      (_, expr) => `{${(expr.match(/(\w+)\s*(?:\)|$)/) || [, "p"])[1]}}`,
    );
    p =
      p
        .split("?")[0]
        .replace(/([^/])\{[^/]*\}$/, "$1")
        .replace(/\/+$/, "") || "/";
    if (/[^\w{}/.:-]/.test(p)) continue;
    found.push({ path: p, method: methodNear(src, m.index + m[0].length) });
  }
  return found;
}

function segmentsMatch(literal, endpoint) {
  const a = literal.split("/");
  const b = endpoint.split("/");
  if (a.length !== b.length) return false;
  const param = (seg) => /^\{.*\}$/.test(seg);
  return a.every((seg, i) => seg === b[i] || param(seg) || param(b[i]));
}

/** Preenche page.apis para cada pagina. */
export function attachPageApis(pages, endpoints) {
  autoIndex ??= buildAutoIndex();
  const byPath = new Map();
  for (const e of endpoints) {
    const key = e.path.split("/").length;
    if (!byPath.has(key)) byPath.set(key, []);
    byPath.get(key).push(e);
  }
  const fileCache = new Map();
  const literalsOf = (file) => {
    if (!fileCache.has(file)) fileCache.set(file, literalsIn(readText(file)));
    return fileCache.get(file);
  };
  const unmatched = new Set();
  for (const page of pages) {
    const start = path.join(ROOT, page.file);
    const seen = new Set([start]);
    let frontier = [start];
    const apis = new Set();
    for (let depth = 0; depth <= MAX_DEPTH && frontier.length; depth += 1) {
      const next = [];
      for (const file of frontier) {
        for (const lit of literalsOf(file)) {
          const candidates = (
            byPath.get(lit.path.split("/").length) || []
          ).filter((e) => segmentsMatch(lit.path, e.path));
          if (!candidates.length) {
            unmatched.add(lit.path);
            apis.add(`${lit.method || "GET?"} ${lit.path}`);
            continue;
          }
          // sem `method:` o cliente usa GET; entre candidatos prefere o metodo detectado/GET
          const want = lit.method || "GET";
          const picked = candidates.filter((e) => e.method === want);
          for (const e of picked.length ? picked : candidates) apis.add(e.id);
        }
        for (const dep of dependenciesOf(file, readText(file), autoIndex)) {
          if (!seen.has(dep)) {
            seen.add(dep);
            next.push(dep);
          }
        }
      }
      frontier = next;
    }
    page.apis = [...apis].sort();
  }
  return { unmatched: [...unmatched].sort() };
}

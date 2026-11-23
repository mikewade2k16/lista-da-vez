// Utilitarios compartilhados do gerador de inventario.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

export const ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../../..",
);

export function rel(abs) {
  return path.relative(ROOT, abs).split(path.sep).join("/");
}

const cache = new Map();
export function readText(abs) {
  if (!cache.has(abs)) cache.set(abs, fs.readFileSync(abs, "utf8"));
  return cache.get(abs);
}

export function exists(abs) {
  return fs.existsSync(abs);
}

/** Lista arquivos recursivamente, ordenados, filtrando por predicado de caminho relativo ao root dado. */
export function walk(dir, predicate = () => true) {
  if (!fs.existsSync(dir)) return [];
  const out = [];
  const skip = new Set(["node_modules", ".nuxt", ".output", ".git", "dist"]);
  const visit = (d) => {
    for (const entry of fs.readdirSync(d, { withFileTypes: true })) {
      if (skip.has(entry.name)) continue;
      const full = path.join(d, entry.name);
      if (entry.isDirectory()) visit(full);
      else if (predicate(full)) out.push(full);
    }
  };
  visit(dir);
  return out.sort();
}

export function sortBy(list, ...keys) {
  return [...list].sort((a, b) => {
    for (const key of keys) {
      const av = typeof key === "function" ? key(a) : a[key];
      const bv = typeof key === "function" ? key(b) : b[key];
      if (av < bv) return -1;
      if (av > bv) return 1;
    }
    return 0;
  });
}

export function uniqSorted(list) {
  return [...new Set(list)].sort();
}

export function readJson(abs) {
  return JSON.parse(fs.readFileSync(abs, "utf8"));
}

export function lineAt(text, index) {
  let line = 1;
  for (let i = 0; i < index; i += 1) if (text.charCodeAt(i) === 10) line += 1;
  return line;
}

// Indice dos pacotes Go do backend e resolucao estatica de strings/definicoes.
import path from "node:path";
import { ROOT, readText, rel, walk, lineAt } from "./util.mjs";
import {
  maskGo,
  matchClose,
  splitArgs,
  splitPlus,
  stripQuotes,
} from "./go-lex.mjs";

const INTERNAL = path.join(ROOT, "back/internal");

/** @typedef {{abs:string, rel:string, dir:string, src:string, masked:string, pkg:object}} GoFile */

export function loadGoPackages() {
  const files = walk(
    INTERNAL,
    (f) => f.endsWith(".go") && !f.endsWith("_test.go"),
  );
  const byDir = new Map();
  const all = [];
  for (const abs of files) {
    const src = readText(abs);
    const file = {
      abs,
      rel: rel(abs),
      dir: path.dirname(abs),
      src,
      masked: maskGo(src),
    };
    all.push(file);
    if (!byDir.has(file.dir)) byDir.set(file.dir, []);
    byDir.get(file.dir).push(file);
  }
  return { files: all, byDir };
}

/** Modulo logico do arquivo: modules/<x>/... -> x; platform/... -> platform. */
export function moduleOf(relPath) {
  const m = relPath.match(/^back\/internal\/modules\/([^/]+)\//);
  if (m) return m[1];
  if (relPath.startsWith("back/internal/platform/")) return "platform";
  return "unknown";
}

const esc = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");

function nearest(matches, pos) {
  let best = null;
  for (const m of matches) if (m.index < pos) best = m;
  return best || matches[0] || null;
}

const LITERAL_HEAD = /^(\[\]string|map\[string\][\w.*]+)\s*\{/;

/** Localiza `for [k,] v := range <expr> {` que declara `name` e envolve pos. Retorna {start,end,role}. */
function loopRange(file, name, pos) {
  const re = /for\s+(\w+)\s*(?:,\s*(\w+))?\s*:=\s*range\s+/g;
  let found = null;
  for (const m of file.masked.matchAll(re)) {
    if (m.index >= pos) break;
    if (m[1] !== name && m[2] !== name) continue;
    const start = m.index + m[0].length;
    const exprEnd = rangeExprEnd(file.masked, start);
    if (exprEnd < 0) continue;
    const open = file.masked.indexOf("{", exprEnd);
    const close = open >= 0 ? matchClose(file.masked, open) : -1;
    if (close > pos)
      found = { start, end: exprEnd, role: m[2] === name ? "value" : "key" };
  }
  return found;
}

function rangeExprEnd(masked, start) {
  // literal `[]string{...}`/`map[string]T{...}` termina no fechamento da chave; senao e um identificador.
  const slice = masked.slice(start);
  const lit = slice.match(LITERAL_HEAD);
  if (lit) return matchClose(masked, start + lit[0].length - 1) + 1;
  const m = slice.match(/^[\w.]+/);
  return m ? start + m[0].length : -1;
}

/** Strings de um literal `[]string{...}` (role value) ou chaves de `map[string]T{...}` (role key). */
function listLiteral(file, start, pos, depth, role) {
  const head = file.masked.slice(start, start + 80).match(LITERAL_HEAD);
  if (head) {
    const open = start + head[0].length - 1;
    const close = matchClose(file.masked, open);
    const isMap = head[1].startsWith("map");
    if (isMap !== (role === "key")) return null;
    const out = [];
    for (const a of splitArgs(file.masked, open, close)) {
      const text = file.src.slice(a.start, a.end);
      const lit = isMap
        ? text.match(/^\s*"([^"\n]*)"\s*:/)
        : text.match(/^\s*"([^"\n]*)"\s*$/);
      if (lit) out.push(lit[1]);
    }
    return out.length ? out : null;
  }
  const ident = file.masked.slice(start, start + 80).match(/^\w+/);
  if (ident && depth < 4) {
    const def = findAssignment(file, ident[0], pos);
    if (def)
      return listLiteral(def.file, def.start, def.start, depth + 1, role);
  }
  return null;
}

/** Acha a atribuicao `name :=|= rhs` mais proxima antes de pos (arquivo, depois pacote). */
export function findAssignment(file, name, pos, pkgFiles = [file]) {
  const re = new RegExp(
    `(?<![\\w.])${esc(name)}(?:\\s+[A-Za-z_][\\w.]*)?\\s*(?::=|=)(?!=)[ \\t]*`,
    "g",
  );
  for (const f of [file, ...pkgFiles.filter((p) => p !== file)]) {
    const matches = [...f.masked.matchAll(re)];
    if (!matches.length) continue;
    const m = nearest(matches, f === file ? pos : Infinity);
    const start = m.index + m[0].length;
    let end = f.masked.indexOf("\n", start);
    if (end < 0) end = f.masked.length;
    return { file: f, start, end };
  }
  return null;
}

/**
 * Avalia expressao de string (literais, concatenacao, identificadores de const/var/loop) em [start,end).
 * Retorna array de valores possiveis (cartesiano) ou null se nao resolvivel.
 */
export function evalString(file, start, end, pos, pkgFiles, depth = 0) {
  if (depth > 5) return null;
  let results = [""];
  for (const part of splitPlus(file.masked, start, end)) {
    const text = file.src.slice(part.start, part.end).trim();
    let values = null;
    const lit = stripQuotes(text);
    if (lit !== null) values = [lit];
    else if (/^[A-Za-z_]\w*$/.test(text))
      values = evalIdent(file, text, pos, pkgFiles, depth);
    if (!values) return null;
    results = results.flatMap((r) => values.map((v) => r + v));
  }
  return results;
}

function evalIdent(file, name, pos, pkgFiles, depth) {
  const loop = loopRange(file, name, pos);
  if (loop) return listLiteral(file, loop.start, pos, 0, loop.role);
  const local = findAssignment(file, name, pos, [file]);
  if (local) {
    const values = evalString(
      local.file,
      local.start,
      local.end,
      local.start,
      pkgFiles,
      depth + 1,
    );
    if (values) return values;
  }
  const viaParam = paramValues(file, name, pos, pkgFiles, depth);
  if (viaParam) return viaParam;
  const shared = findAssignment(file, name, pos, pkgFiles);
  if (shared && shared.file !== file)
    return evalString(
      shared.file,
      shared.start,
      shared.end,
      shared.start,
      pkgFiles,
      depth + 1,
    );
  return null;
}

/** Nomes de parametros de uma definicao (closure ou func), em ordem. */
export function defParams(def) {
  const text = def.file.masked.slice(def.start, def.end);
  const m =
    def.kind === "func"
      ? text.match(/^func\s+(?:\([^)]*\)\s*)?\w+\s*\(/)
      : text.match(/func\s*\(/);
  if (!m) return [];
  const open = def.start + m.index + m[0].length - 1;
  const close = matchClose(def.file.masked, open);
  return splitArgs(def.file.masked, open, close).map(
    (a) => def.file.masked.slice(a.start, a.end).trim().split(/\s+/)[0],
  );
}

/** Se `name` e parametro da funcao que envolve pos, devolve os argumentos correspondentes nos pontos de chamada. */
export function paramArgRanges(file, name, pos, pkgFiles) {
  const declRe = /^func\s+(?:\([^)]*\)\s*)?(\w+)\s*\(/gm;
  let decl = null;
  for (const m of file.masked.matchAll(declRe)) {
    if (m.index > pos) break;
    decl = m;
  }
  if (!decl) return null;
  const open = decl.index + decl[0].length - 1;
  const close = matchClose(file.masked, open);
  if (close < 0 || pos < close) return null;
  const index = splitArgs(file.masked, open, close)
    .map((a) => file.masked.slice(a.start, a.end).trim().split(/\s+/)[0])
    .indexOf(name);
  if (index < 0) return null;
  const callRe = new RegExp(`(?<![\\w])${esc(decl[1])}\\s*\\(`, "g");
  const out = [];
  for (const f of pkgFiles) {
    for (const m of f.masked.matchAll(callRe)) {
      if (f === file && m.index === decl.index + decl[0].indexOf(decl[1]))
        continue;
      if (
        /func\s*(\([^)]*\)\s*)?$/.test(
          f.masked.slice(Math.max(0, m.index - 40), m.index),
        )
      )
        continue;
      const o = m.index + m[0].length - 1;
      const c = matchClose(f.masked, o);
      const args = splitArgs(f.masked, o, c);
      if (!args[index]) return null;
      out.push({
        file: f,
        start: args[index].start,
        end: args[index].end,
        pos: o,
      });
    }
  }
  return out.length ? out : null;
}

function paramValues(file, name, pos, pkgFiles, depth) {
  const ranges = paramArgRanges(file, name, pos, pkgFiles);
  if (!ranges) return null;
  const out = [];
  for (const r of ranges) {
    const values = evalString(
      r.file,
      r.start,
      r.end,
      r.pos,
      pkgFiles,
      depth + 1,
    );
    if (!values) return null;
    out.push(...values);
  }
  return [...new Set(out)];
}

/**
 * Corpo de uma definicao nomeada: closure `name := func(...){...}`, rhs de atribuicao
 * (`wrap := middleware.RequireAuth`) ou `func [(recv)] name(...) {...}`.
 * Retorna {file, start, end, kind} ou null.
 */
const defCache = new Map();

function definitionIndex(f, name) {
  const key = `${f.rel}|${name}`;
  if (!defCache.has(key)) {
    const closure = new RegExp(
      `(?<![\\w.])${esc(name)}\\s*(?::=|=)\\s*func\\b`,
      "g",
    );
    const decl = new RegExp(
      `^func\\s+(?:\\([^)]*\\)\\s*)?${esc(name)}\\s*[\\[(]`,
      "gm",
    );
    const assign = new RegExp(
      `(?<![\\w.])${esc(name)}\\s*:=\\s*(?!func\\b)`,
      "g",
    );
    defCache.set(key, {
      closures: [...f.masked.matchAll(closure)],
      decl: [...f.masked.matchAll(decl)][0] || null,
      assigns: [...f.masked.matchAll(assign)],
    });
  }
  return defCache.get(key);
}

export function findDefinition(
  file,
  name,
  pkgFiles,
  pos = Infinity,
  localOnly = false,
) {
  const files = localOnly
    ? [file]
    : [file, ...pkgFiles.filter((f) => f !== file)];
  for (const f of files) {
    const idx = definitionIndex(f, name);
    const c = nearest(idx.closures, f === file ? pos : Infinity);
    if (c) return span(f, c.index, "closure", c.index + c[0].length);
    if (idx.decl)
      return span(
        f,
        idx.decl.index,
        "func",
        idx.decl.index + idx.decl[0].length - 1,
      );
  }
  for (const f of files) {
    const a = nearest(
      definitionIndex(f, name).assigns,
      f === file ? pos : Infinity,
    );
    if (a) {
      let end = f.masked.indexOf("\n", a.index + a[0].length);
      if (end < 0) end = f.masked.length;
      return { file: f, start: a.index, end, kind: "assign" };
    }
  }
  return null;
}

function span(f, index, kind, parenFrom) {
  const open = f.masked.indexOf(
    "(",
    parenFrom - 1 > index && kind === "closure" ? parenFrom : parenFrom - 1,
  );
  const sigClose = open >= 0 ? matchClose(f.masked, open) : -1;
  const bodyOpen = sigClose > 0 ? f.masked.indexOf("{", sigClose) : -1;
  const close = bodyOpen >= 0 ? matchClose(f.masked, bodyOpen) : -1;
  return {
    file: f,
    start: index,
    end: close > 0 ? close + 1 : f.masked.length,
    kind,
  };
}

export { lineAt };

const funcStarts = new Map();
/** Indice (offset) do `func` de topo que envolve pos; serve de chave de escopo. */
export function enclosingFunc(file, pos) {
  if (!funcStarts.has(file.rel))
    funcStarts.set(
      file.rel,
      [...file.masked.matchAll(/^func\b/gm)].map((m) => m.index),
    );
  let found = -1;
  for (const i of funcStarts.get(file.rel)) {
    if (i > pos) break;
    found = i;
  }
  return found;
}

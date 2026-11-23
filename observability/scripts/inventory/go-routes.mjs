// Extracao de endpoints HTTP (mux.Handle e helpers de registro) do backend Go.
import { defParams, evalString, findDefinition, moduleOf } from "./go-pkg.mjs";
import { classifyRange, finalizeAuth, handlerAuthHint } from "./go-auth.mjs";
import { matchClose, splitArgs } from "./go-lex.mjs";
import { packageDeps } from "./go-deps.mjs";
import { lineAt, sortBy } from "./util.mjs";

const METHODS = "GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS";
const PATTERN_RE = new RegExp(`^(${METHODS}) (/\\S*)$`);
const LITERAL_START = new RegExp(`^"(?:${METHODS})[ "]`);
const SECOND_LITERAL = new RegExp(
  String.raw`^[\w.]+\s*,\s*"(?:${METHODS})[ "]`,
);
const CALL_RE = /([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*\(/g;

const QUERY_RES = [
  /Query\(\)\s*\.\s*(?:Get|Has)\(\s*"([\w.-]+)"/g,
  /Query\(\)\s*\[\s*"([\w.-]+)"\s*\]/g,
  /FormValue\(\s*"([\w.-]+)"/g,
  /\w*[Qq]uery\w*\(\s*r\s*,\s*"([\w.-]+)"/g,
];

function queryParamsIn(text) {
  const found = new Set();
  for (const re of QUERY_RES)
    for (const m of text.matchAll(re)) found.add(m[1]);
  for (const v of text.matchAll(/(\w+)\s*:?=\s*r\.URL\.Query\(\)/g)) {
    const name = v[1];
    for (const m of text.matchAll(
      new RegExp(`\\b${name}\\.(?:Get|Has)\\(\\s*"([\\w.-]+)"`, "g"),
    ))
      found.add(m[1]);
    for (const m of text.matchAll(
      new RegExp(`\\b${name}\\[\\s*"([\\w.-]+)"\\s*\\]`, "g"),
    ))
      found.add(m[1]);
  }
  return found;
}

const loopVarCache = new Map();
function loopVars(file) {
  if (!loopVarCache.has(file.rel)) {
    const names = new Set();
    for (const m of file.masked.matchAll(
      /for\s+(\w+)\s*(?:,\s*(\w+))?\s*:=\s*range\s/g,
    )) {
      names.add(m[1]);
      if (m[2]) names.add(m[2]);
    }
    loopVarCache.set(file.rel, names);
  }
  return loopVarCache.get(file.rel);
}

function enclosingIsHelperParam(file, pos, ident) {
  let idx = -1;
  for (const m of file.masked.matchAll(/^func\b|:=\s*func\b/gm)) {
    if (m.index > pos) break;
    idx = m.index;
  }
  if (idx < 0) return false;
  const sig = file.masked.slice(idx, file.masked.indexOf("{", idx));
  return new RegExp(`\\b${ident}\\b`).test(sig);
}

/** Textos do handler: o proprio argumento mais o corpo das funcoes do pacote citadas nele. */
function handlerBodies(file, argsText, pkgFiles) {
  const texts = [argsText];
  const idents = new Set(
    [...argsText.matchAll(/(?:\w+\.)?([A-Za-z_]\w*)\s*[(,)]/g)].map(
      (m) => m[1],
    ),
  );
  for (const name of idents) {
    if (texts.length > 8) break;
    if (/^(func|wrap|http|if|for)$/.test(name)) continue;
    const def = findDefinition(file, name, pkgFiles);
    if (def && def.kind === "func")
      texts.push(def.file.src.slice(def.start, def.end));
  }
  return texts;
}

function queryParamsFor(texts) {
  const found = new Set();
  texts.forEach((t) => queryParamsIn(t).forEach((q) => found.add(q)));
  return [...found].sort();
}

/** Extrai todos os endpoints. Retorna { endpoints, gaps }. */
export function extractEndpoints(pkgs) {
  const endpoints = new Map();
  const gaps = [];
  const depsCache = new Map();
  const depsOf = (dir) => {
    if (!depsCache.has(dir))
      depsCache.set(dir, packageDeps(pkgs.byDir.get(dir)));
    return depsCache.get(dir);
  };

  for (const file of pkgs.files) {
    if (!/Handle|"\s*(?:GET|POST|PUT|PATCH|DELETE|HEAD)\s/.test(file.src))
      continue;
    const pkgFiles = pkgs.byDir.get(file.dir);
    for (const m of file.masked.matchAll(CALL_RE)) {
      const callee = m[1];
      const calleeName = callee.split(".").pop();
      const isMux = /(^|\.)mux\.Handle(Func)?$/.test(callee);
      const open = m.index + m[0].length - 1;
      const head = file.src.slice(open + 1, open + 200).trimStart();
      const loopHead = head.match(/^([A-Za-z_]\w*)\s*,/);
      const patternIdx =
        LITERAL_START.test(head) || isMux
          ? 0
          : SECOND_LITERAL.test(head)
            ? 1
            : loopHead && loopVars(file).has(loopHead[1])
              ? 0
              : -1;
      if (patternIdx < 0) continue;
      if (isMux && !/^[`"A-Za-z_\n]/.test(head)) continue;
      const close = matchClose(file.masked, open);
      if (close < 0) continue;
      const args = splitArgs(file.masked, open, close);
      if (!args.length) continue;
      if (!args[patternIdx]) continue;
      const first = file.src
        .slice(args[patternIdx].start, args[patternIdx].end)
        .trim();
      if (/^(strings|fmt|http|httptest)\./.test(callee) && !isMux) continue;
      if (
        isMux &&
        /^[A-Za-z_]\w*$/.test(first) &&
        enclosingIsHelperParam(file, open, first)
      )
        continue;
      const values = evalString(
        file,
        args[patternIdx].start,
        args[patternIdx].end,
        open,
        pkgFiles,
      );
      const line = lineAt(file.src, m.index);
      if (!values) {
        if (
          isMux &&
          /^[A-Za-z_]\w*$/.test(first) &&
          enclosingIsHelperParam(file, open, first)
        )
          continue;
        if (isMux)
          gaps.push({
            kind: "endpoint",
            item: `${file.rel}:${line}`,
            reason: `pattern nao resolvido estaticamente: ${first.slice(0, 80)}`,
          });
        continue;
      }
      const rest = args.slice(patternIdx + 1);
      const argsStart = rest.length ? rest[0].start : close;
      let result;
      const def = isMux
        ? null
        : findDefinition(file, calleeName, pkgFiles, open);
      if (def) {
        const bindings = new Map();
        defParams(def).forEach((name, i) => {
          if (args[i] && i !== patternIdx)
            bindings.set(name, {
              file,
              start: args[i].start,
              end: args[i].end,
            });
        });
        result = classifyRange(
          def.file,
          def.start,
          def.end,
          pkgFiles,
          undefined,
          undefined,
          0,
          bindings,
          true,
        );
      } else {
        result = classifyRange(file, argsStart, close, pkgFiles);
      }
      const argsText = file.src.slice(argsStart, close);
      for (const value of values) {
        const pm = value.replace(/\s+/g, " ").match(PATTERN_RE);
        if (!pm) {
          if (isMux)
            gaps.push({
              kind: "endpoint",
              item: `${file.rel}:${line}`,
              reason: `pattern fora do formato "METHOD /path": ${value}`,
            });
          continue;
        }
        const [, method, routePath] = pm;
        const id = `${method} ${routePath}`;
        if (endpoints.has(id)) continue;
        const bodies = handlerBodies(file, argsText, pkgFiles);
        const final = finalizeAuth(result, routePath, method);
        const hint = final.auth === "unknown" ? handlerAuthHint(bodies) : null;
        if (hint) {
          final.auth = hint.auth;
          final.notes = [hint.note];
        }
        const { auth, roles, permissions, notes } = final;
        const deps =
          moduleOf(file.rel) === "platform"
            ? { db: false, external: [] }
            : depsOf(file.dir);
        endpoints.set(id, {
          id,
          method,
          path: routePath,
          module: moduleOf(file.rel),
          file: file.rel,
          line,
          auth,
          roles,
          permissions,
          access: method === "GET" || method === "HEAD" ? "read" : "write",
          pathParams: [...routePath.matchAll(/\{(\w+)(?:\.\.\.)?\}/g)].map(
            (x) => x[1],
          ),
          queryParams: queryParamsFor(bodies),
          deps: { db: deps.db, external: deps.external },
          notes,
        });
      }
    }
  }
  const list = sortBy([...endpoints.values()], "path", "method");
  for (const e of list) {
    if (e.auth === "unknown")
      gaps.push({
        kind: "endpoint-auth",
        item: e.id,
        reason: `${e.file}:${e.line} ${e.notes.join("; ")}`,
      });
  }
  return { endpoints: list, gaps };
}

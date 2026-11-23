// Classificacao estatica de autenticacao/autorizacao de um handler registrado.
import path from "node:path";
import { ROOT, readText } from "./util.mjs";
import {
  defParams,
  enclosingFunc,
  evalString,
  findDefinition,
  paramArgRanges,
} from "./go-pkg.mjs";
import { matchClose, splitArgs } from "./go-lex.mjs";

let roleValues = null;
/** RoleXxx -> valor ("platform_admin"), lido de auth/roles.go. */
function roles() {
  if (!roleValues) {
    roleValues = {};
    const src = readText(
      path.join(ROOT, "back/internal/modules/auth/roles.go"),
    );
    for (const m of src.matchAll(/(Role\w+)\s+Role\s*=\s*"([^"]+)"/g))
      roleValues[m[1]] = m[2];
  }
  return roleValues;
}

const PERMISSION_FN = /permission/i;
const SKIP_CALLEES = new Set([
  "func",
  "if",
  "for",
  "switch",
  "make",
  "len",
  "append",
  "string",
  "http",
  "HandlerFunc",
  "Handler",
  "return",
  "range",
  "nil",
  "true",
  "false",
  "err",
  "ok",
  "var",
  "else",
  "case",
  "mux",
  "w",
  "r",
  "ctx",
]);

function emptyResult() {
  return {
    steps: 0,
    requireAuth: false,
    requireAccount: false,
    roles: new Set(),
    permissions: new Set(),
    unresolved: [],
  };
}

/**
 * Classifica o trecho [start,end) de `file`. Segue chamadas a closures/funcoes do pacote
 * (exceto handlers `handleX`) ate profundidade 4.
 */
export function classifyRange(
  file,
  start,
  end,
  pkgFiles,
  result = emptyResult(),
  seen = new Set(),
  depth = 0,
  bindings = null,
  isDef = false,
) {
  const masked = file.masked;
  result.steps += 1;
  if (result.steps > 400) return result;
  const text = masked.slice(start, end);
  const tokenRe = /([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)\s*(\()?/g;
  let m;
  let ownFuncSeen = false;
  while ((m = tokenRe.exec(text)) !== null) {
    if (m[1] === "func" && m[2] && isDef && !ownFuncSeen) {
      ownFuncSeen = true;
      continue;
    }
    if (m[1] === "func" && m[2]) {
      // literal de funcao: nao varre o corpo do handler inline
      const sigClose = matchClose(masked, start + m.index + m[0].length - 1);
      const bodyOpen = sigClose > 0 ? masked.indexOf("{", sigClose) : -1;
      const bodyClose = bodyOpen > 0 ? matchClose(masked, bodyOpen) : -1;
      const isHandlerLiteral =
        file.src.slice(start + m.index, sigClose).includes("ResponseWriter") &&
        /^[\s\w.*]*$/.test(masked.slice(sigClose + 1, bodyOpen));
      if (isHandlerLiteral && bodyClose > 0 && bodyClose < end)
        tokenRe.lastIndex = bodyClose - start + 1;
      continue;
    }
    const name = m[1].split(".").pop();
    if (/^x+$/.test(name)) continue;
    const hasCall = Boolean(m[2]);
    const open = start + m.index + m[0].length - 1;
    if (name === "RequireRoles") {
      const close = matchClose(masked, open);
      for (const r of file.src.slice(open, close).matchAll(/\bRole\w+/g))
        result.roles.add(roles()[r[0]] || r[0]);
      continue;
    }
    if (name === "RequireAuthWithAccount") {
      result.requireAccount = true;
      continue;
    }
    if (name === "RequireAuth") {
      result.requireAuth = true;
      continue;
    }
    const bound = bindings && !m[1].includes(".") ? bindings.get(name) : null;
    if (bound && depth < 6) {
      classifyRange(
        bound.file,
        bound.start,
        bound.end,
        pkgFiles,
        result,
        seen,
        depth + 1,
        null,
      );
      if (hasCall)
        classifyRange(
          file,
          open,
          matchClose(masked, open) + 1,
          pkgFiles,
          result,
          seen,
          depth + 1,
          bindings,
        );
      continue;
    }
    if (!hasCall) {
      if (depth < 6 && /^[a-z]\w*$/.test(name) && !SKIP_CALLEES.has(name))
        followDefinition(
          file,
          name,
          start + m.index,
          pkgFiles,
          result,
          seen,
          depth,
          true,
        );
      continue;
    }
    if (
      PERMISSION_FN.test(name) &&
      !/^(has|can|allows|contains|list|resolve|replace|invalid|recognized|default)/i.test(
        name,
      )
    ) {
      collectPermissions(file, open, pkgFiles, result, bindings);
    }
    if (SKIP_CALLEES.has(name) || depth >= 6) continue;
    followDefinition(
      file,
      name,
      start + m.index,
      pkgFiles,
      result,
      seen,
      depth,
    );
    permissionFromParams(file, name, open, pkgFiles, result, bindings);
  }
  return result;
}

function followDefinition(
  file,
  name,
  pos,
  pkgFiles,
  result,
  seen,
  depth,
  onlyLocal = false,
) {
  const key = `${file.rel}:${enclosingFunc(file, pos)}:${name}`;
  if (seen.has(key)) return;
  const def = findDefinition(file, name, pkgFiles, pos, onlyLocal);
  if (!def) {
    const ranges = paramArgRanges(file, name, pos, pkgFiles);
    if (ranges && depth < 6) {
      seen.add(key);
      for (const r of ranges)
        classifyRange(
          r.file,
          r.start,
          r.end,
          pkgFiles,
          result,
          seen,
          depth + 1,
        );
    }
    return;
  }
  if (onlyLocal && def.kind === "func") return;
  seen.add(key);
  if (def.kind === "func" && !returnsHandler(def)) return;
  classifyRange(
    def.file,
    def.start,
    def.end,
    pkgFiles,
    result,
    seen,
    depth + 1,
    null,
    true,
  );
}

function returnsHandler(def) {
  const text = def.file.masked.slice(def.start, def.end);
  const firstLine = text.slice(
    0,
    text.indexOf("{") > 0 ? text.indexOf("{") + 1 : 200,
  );
  return (
    /http\.Handler(Func)?\s*\{$/.test(firstLine.trim()) ||
    /\)\s*(http\.Handler|http\.HandlerFunc)\b/.test(firstLine)
  );
}

function collectPermissions(file, open, pkgFiles, result, bindings) {
  const close = matchClose(file.masked, open);
  if (close < 0) return;
  const args = splitArgs(file.masked, open, close);
  // a permissao e o 1o argumento que for literal ou identificador resolvivel
  for (const a of args.slice(0, 3)) {
    const text = file.src.slice(a.start, a.end).trim();
    if (/^func\b|\bfunc\(/.test(text)) break;
    const boundArg = bindings && /^\w+$/.test(text) ? bindings.get(text) : null;
    const values = boundArg
      ? evalString(
          boundArg.file,
          boundArg.start,
          boundArg.end,
          boundArg.start,
          pkgFiles,
        )
      : evalString(file, a.start, a.end, open, pkgFiles);
    if (values && values.every((v) => /^[\w.:-]+$/.test(v) && v.length > 2)) {
      values.forEach((v) => result.permissions.add(v));
      return;
    }
    const ident = text.match(/^(?:\w+\.)?([A-Za-z_]\w*)$/);
    if (
      ident &&
      /^(perm|Perm)/.test(ident[1]) &&
      !/^(permission|perm|permissions)$/i.test(ident[1])
    ) {
      // constante de outro pacote: nao resolvida
      result.unresolved.push(text);
      return;
    }
  }
}

export function finalizeAuth(result, path, method) {
  const notes = [];
  const permissions = [...result.permissions].sort();
  const roleList = [...result.roles].sort();
  let auth = "unknown";
  if (roleList.length) auth = "role";
  else if (result.requireAccount) auth = "auth_account";
  else if (result.requireAuth) auth = "auth";
  const publicPath =
    /^\/(v1\/public\/|healthz|uploads\/|s\/|q\/)/.test(path) ||
    /webhook|callback|ingest/i.test(path);
  if (auth === "unknown") {
    if (publicPath) auth = "public";
    else notes.push("wrapper de autenticacao nao reconhecido estaticamente");
  } else if (publicPath && /webhook|callback|ingest/i.test(path)) {
    notes.push("rota de webhook/callback com wrapper de auth");
  }
  if (result.unresolved.length)
    notes.push(
      `permissao nao resolvida: ${[...new Set(result.unresolved)].join(", ")}`,
    );
  if (permissions.length === 0 && method && auth === "public" && !publicPath)
    notes.push("sem auth");
  return { auth, roles: roleList, permissions, notes };
}

/** Autenticacao feita dentro do handler (sem wrapper no registro). Retorna {auth, note} ou null. */
export function handlerAuthHint(texts) {
  const text = texts.join("\n");
  if (
    /ConsumeTicket|ValidateTicket|authenticateRealtimeRequest|AuthenticateToken|ParseToken|authenticateRequest/.test(
      text,
    )
  ) {
    return {
      auth: "auth",
      note: "autenticacao feita dentro do handler (ticket/token)",
    };
  }
  if (/RequireAuth\b|PrincipalFromContext/.test(text)) {
    return { auth: "auth", note: "autenticacao resolvida no corpo do handler" };
  }
  if (
    /[Bb]earer|[Ss]erviceToken|bridgeToken|runtimeAuth|[Ss]ignature|[Ss]haredSecret|[Gg]atewayToken/.test(
      text,
    )
  ) {
    return {
      auth: "unknown",
      note: "protegida por token de servico/assinatura dentro do handler (nao JWT)",
    };
  }
  return null;
}

/** Closure/func chamada com parametro "permission": avalia o argumento correspondente no ponto de chamada. */
function permissionFromParams(file, name, open, pkgFiles, result, bindings) {
  const def = findDefinition(file, name, pkgFiles, open);
  if (!def || def.kind === "assign") return;
  const idx = defParams(def).findIndex((p) => /permission/i.test(p));
  if (idx < 0) return;
  const close = matchClose(file.masked, open);
  const arg = splitArgs(file.masked, open, close)[idx];
  if (!arg) return;
  const text = file.src.slice(arg.start, arg.end).trim();
  const boundArg = bindings && /^\w+$/.test(text) ? bindings.get(text) : null;
  const values = boundArg
    ? evalString(
        boundArg.file,
        boundArg.start,
        boundArg.end,
        boundArg.start,
        pkgFiles,
      )
    : evalString(file, arg.start, arg.end, open, pkgFiles);
  if (values && values.every((v) => /^[\w.:-]+$/.test(v) && v.length > 2))
    values.forEach((v) => result.permissions.add(v));
}

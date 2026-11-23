// Utilitarios minimos de analise lexica de Go (sem AST): mascara de strings/comentarios,
// casamento de parenteses e divisao de argumentos.

/**
 * Devolve texto de mesmo tamanho com comentarios virando espacos e o conteudo de
 * strings/runes virando 'x' (aspas preservadas). Newlines sao mantidos.
 */
export function maskGo(src) {
  const out = src.split("");
  const n = src.length;
  let i = 0;
  const blank = (from, to, fill) => {
    for (let k = from; k < to; k += 1) if (out[k] !== "\n") out[k] = fill;
  };
  while (i < n) {
    const c = src[i];
    const d = src[i + 1];
    if (c === "/" && d === "/") {
      let j = i;
      while (j < n && src[j] !== "\n") j += 1;
      blank(i, j, " ");
      i = j;
    } else if (c === "/" && d === "*") {
      let j = src.indexOf("*/", i + 2);
      j = j === -1 ? n : j + 2;
      blank(i, j, " ");
      i = j;
    } else if (c === '"') {
      let j = i + 1;
      while (j < n && src[j] !== '"' && src[j] !== "\n")
        j += src[j] === "\\" ? 2 : 1;
      blank(i + 1, j, "x");
      i = j + 1;
    } else if (c === "`") {
      let j = src.indexOf("`", i + 1);
      if (j === -1) j = n;
      blank(i + 1, j, "x");
      i = j + 1;
    } else if (c === "'") {
      let j = i + 1;
      while (j < n && src[j] !== "'" && src[j] !== "\n")
        j += src[j] === "\\" ? 2 : 1;
      blank(i + 1, j, "x");
      i = j + 1;
    } else {
      i += 1;
    }
  }
  return out.join("");
}

const PAIRS = { "(": ")", "{": "}", "[": "]" };

/** Indice do fechamento que casa com a abertura em `open` (usar texto mascarado). -1 se nao achar. */
export function matchClose(masked, open) {
  const stack = [];
  for (let i = open; i < masked.length; i += 1) {
    const c = masked[i];
    if (PAIRS[c]) stack.push(PAIRS[c]);
    else if (c === ")" || c === "}" || c === "]") {
      if (stack.pop() !== c) return -1;
      if (stack.length === 0) return i;
    }
  }
  return -1;
}

/** Divide o intervalo (open, close) em argumentos de topo. Retorna [{start,end}] sobre o texto. */
export function splitArgs(masked, open, close) {
  const args = [];
  let depth = 0;
  let start = open + 1;
  for (let i = open + 1; i < close; i += 1) {
    const c = masked[i];
    if (PAIRS[c]) depth += 1;
    else if (c === ")" || c === "}" || c === "]") depth -= 1;
    else if (c === "," && depth === 0) {
      args.push({ start, end: i });
      start = i + 1;
    }
  }
  if (masked.slice(start, close).trim() !== "")
    args.push({ start, end: close });
  return args;
}

/** Divide um intervalo em partes separadas por '+' de topo. */
export function splitPlus(masked, start, end) {
  const parts = [];
  let depth = 0;
  let from = start;
  for (let i = start; i < end; i += 1) {
    const c = masked[i];
    if (PAIRS[c]) depth += 1;
    else if (c === ")" || c === "}" || c === "]") depth -= 1;
    else if (c === "+" && depth === 0) {
      parts.push({ start: from, end: i });
      from = i + 1;
    }
  }
  parts.push({ start: from, end });
  return parts;
}

export function stripQuotes(lit) {
  const t = lit.trim();
  if (/^"[^"]*"$/.test(t) || /^`[^`]*`$/.test(t)) return t.slice(1, -1);
  return null;
}

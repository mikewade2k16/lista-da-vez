// Dependencias detectadas por pacote Go: banco de dados e integracoes externas.

const SQL_RE =
  /pgx|database\/sql|\.(?:Query|QueryRow|Exec|SendBatch|CopyFrom)\(/;
const SQL_CALL_RES = {
  Query: /\.Query\(/g,
  QueryRow: /\.QueryRow\(/g,
  Exec: /\.Exec\(/g,
  SendBatch: /\.SendBatch\(/g,
  CopyFrom: /\.CopyFrom\(/g,
};

/** Integracao -> deteccao em identificadores (texto mascarado) ou literais (texto original). */
export const EXTERNALS = [
  {
    id: "meta",
    ident: /graph\.facebook|(?<![A-Za-z])[Mm]eta[A-Z_]|META_|MetaGraph|metaads/,
    lit: /graph\.facebook\.com|facebook\.com\/v\d/,
  },
  { id: "n8n", ident: /n8n|N8N/, lit: /n8n/i },
  { id: "waha", ident: /[Ww]aha|WAHA/, lit: /waha/i },
  {
    id: "evolution",
    ident: /[Ee]volution(?:API|Client|URL|_)|EVOLUTION_/,
    lit: /evolution/i,
  },
  { id: "whisper", ident: /[Ww]hisper|WHISPER_/, lit: /whisper/i },
  { id: "openai", ident: /[Oo]pen[Aa][Ii]|OPENAI_/, lit: /api\.openai\.com/ },
  {
    id: "anthropic",
    ident: /[Aa]nthropic|ANTHROPIC_/,
    lit: /api\.anthropic\.com/,
  },
  {
    id: "r2",
    ident: /(?<![A-Za-z0-9])R2[A-Z_]|(?<![A-Za-z0-9])r2[A-Z_]|\bs3\b|S3[A-Z_]/,
    lit: /cloudflarestorage\.com|amazonaws\.com/,
  },
  { id: "smtp", ident: /[Ss]mtp|SMTP_/, lit: /smtp\./ },
  { id: "perola", ident: /[Pp]erola|PEROLA_/, lit: /perola/i },
  { id: "erp", ident: /(?<![a-z])[Ee]rp(?![a-z])|ERP_/, lit: /$^/ },
  { id: "ftp", ident: /[Ss]?[Ff]tp(?![a-z])|FTP_/, lit: /$^/ },
  {
    id: "google",
    ident: /[Gg]oogle[A-Z]/,
    lit: /googleapis\.com|accounts\.google\.com/,
  },
  { id: "tiktok", ident: /[Tt]ik[Tt]ok/, lit: /tiktok\.com/ },
];

export function packageDeps(files) {
  const external = new Set();
  let db = false;
  for (const f of files) {
    if (!db && SQL_RE.test(f.masked)) db = true;
    for (const e of EXTERNALS) {
      if (external.has(e.id)) continue;
      if (e.ident.test(f.masked) || e.lit.test(f.src)) external.add(e.id);
    }
  }
  return { db, external: [...external].sort() };
}

export function countSqlCallSites(files) {
  let total = 0;
  const withSql = new Set();
  for (const f of files) {
    let n = 0;
    for (const re of Object.values(SQL_CALL_RES))
      n += (f.masked.match(re) || []).length;
    if (n > 0) withSql.add(f.rel);
    total += n;
  }
  return { sqlCallSites: total, files: withSql.size };
}

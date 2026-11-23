// Schema do banco: catalogos do Postgres (somente leitura) com fallback para parse das migrations.
import path from "node:path";
import pg from "pg";
import { ROOT, readText, rel, walk, sortBy } from "./util.mjs";

const MIGRATIONS = path.join(
  ROOT,
  "back/internal/platform/database/migrations",
);
const SYSTEM_SCHEMAS = ["pg_catalog", "information_schema", "pg_toast"];

export function moduleOfTable(schema, name, map) {
  if (schema !== "public") return map.schemaToModule[schema] || null;
  for (const [prefix, mod] of map.publicTablePrefixes)
    if (name.startsWith(prefix)) return mod;
  return null;
}

const Q_RELATIONS = `
  select n.nspname as schema, c.relname as name, case c.relkind when 'v' then 'view' when 'm' then 'view' else 'table' end as kind,
         c.oid, greatest(c.reltuples, 0)::bigint as rows, pg_total_relation_size(c.oid)::bigint as size
    from pg_class c join pg_namespace n on n.oid = c.relnamespace
   where c.relkind in ('r','p','v','m') and n.nspname <> all($1) and n.nspname not like 'pg_temp%' and n.nspname not like 'pg_toast%'`;
const Q_COLUMNS = `
  select a.attrelid as oid, a.attname as name, format_type(a.atttypid, a.atttypmod) as type, not a.attnotnull as nullable,
         pg_get_expr(d.adbin, d.adrelid) as "default"
    from pg_attribute a left join pg_attrdef d on d.adrelid = a.attrelid and d.adnum = a.attnum
   where a.attnum > 0 and not a.attisdropped and a.attrelid = any($1) order by a.attrelid, a.attnum`;
const Q_CONSTRAINTS = `
  select conrelid as oid, conname as name, contype as type, pg_get_constraintdef(oid) as def, conkey as keys,
         confrelid::regclass::text as ref_table, (select array_agg(attname::text order by ord) from unnest(conkey) with ordinality k(attnum, ord)
            join pg_attribute a on a.attrelid = conrelid and a.attnum = k.attnum) as cols,
         (select array_agg(attname::text order by ord) from unnest(confkey) with ordinality k(attnum, ord)
            join pg_attribute a on a.attrelid = confrelid and a.attnum = k.attnum) as ref_cols
    from pg_constraint where conrelid = any($1) and contype in ('p','u','c','f')`;
const Q_INDEXES = `
  select i.indrelid as oid, ic.relname as name, pg_get_indexdef(i.indexrelid) as def, i.indisunique as "unique", i.indisprimary as "primary",
         (select array_agg(a.attname::text order by k.ord) from unnest(i.indkey::int2[]) with ordinality k(attnum, ord)
            join pg_attribute a on a.attrelid = i.indrelid and a.attnum = k.attnum where k.attnum > 0) as cols
    from pg_index i join pg_class ic on ic.oid = i.indexrelid where i.indrelid = any($1)`;

function covers(indexCols, fkCols) {
  if (!indexCols || indexCols.length < fkCols.length) return false;
  const head = new Set(indexCols.slice(0, fkCols.length));
  return fkCols.every((c) => head.has(c));
}

/** Le o schema via Postgres vivo. Lanca erro se inacessivel. */
async function fromDatabase(url, map) {
  const client = new pg.Client({
    connectionString: url,
    connectionTimeoutMillis: 4000,
  });
  await client.connect();
  try {
    await client.query("BEGIN READ ONLY");
    const rels = (await client.query(Q_RELATIONS, [SYSTEM_SCHEMAS])).rows;
    const oids = rels.map((r) => r.oid);
    const cols = await client.query(Q_COLUMNS, [oids]);
    const cons = await client.query(Q_CONSTRAINTS, [oids]);
    const idx = await client.query(Q_INDEXES, [oids]);
    const group = (rows) =>
      rows.reduce(
        (m, r) => m.set(r.oid, [...(m.get(r.oid) || []), r]),
        new Map(),
      );
    const colsBy = group(cols.rows);
    const consBy = group(cons.rows);
    const idxBy = group(idx.rows);
    await client.query("ROLLBACK");
    return rels.map((r) => {
      const indexes = idxBy.get(r.oid) || [];
      const constraints = consBy.get(r.oid) || [];
      return {
        schema: r.schema,
        name: r.name,
        module: moduleOfTable(r.schema, r.name, map),
        kind: r.kind,
        columns: (colsBy.get(r.oid) || []).map((c) => ({
          name: c.name,
          type: c.type,
          nullable: c.nullable,
          default: c.default,
        })),
        primaryKey: (constraints.find((c) => c.type === "p") || {}).cols || [],
        indexes: sortBy(
          indexes.map((i) => ({
            name: i.name,
            definition: i.def,
            unique: i.unique,
            columns: i.cols || [],
          })),
          "name",
        ),
        foreignKeys: sortBy(
          constraints
            .filter((c) => c.type === "f")
            .map((c) => ({
              name: c.name,
              columns: c.cols,
              references: c.ref_table,
              referencedColumns: c.ref_cols,
              indexed: indexes.some((i) => covers(i.cols, c.cols)),
            })),
          "name",
        ),
        constraints: sortBy(
          constraints
            .filter((c) => c.type === "u" || c.type === "c")
            .map((c) => ({
              name: c.name,
              type: c.type === "u" ? "unique" : "check",
              definition: c.def,
            })),
          "name",
        ),
        rowEstimate: Number(r.rows),
        sizeBytes: Number(r.size),
      };
    });
  } finally {
    await client.end().catch(() => {});
  }
}

/** Fallback: parse grosseiro das migrations (create table/index, alter add column). */
function fromMigrations(map) {
  const tables = new Map();
  const files = walk(MIGRATIONS, (f) => f.endsWith(".sql"));
  const key = (s, n) => `${s}.${n}`;
  for (const file of files) {
    const sql = readText(file).replace(/--[^\n]*/g, "");
    for (const m of sql.matchAll(
      /create\s+table\s+(?:if\s+not\s+exists\s+)?(?:(\w+)\.)?(\w+)\s*\(([\s\S]*?)\n\)\s*;/gi,
    )) {
      const schema = m[1] || "public";
      const columns = [];
      for (const line of m[3].split("\n")) {
        const c = line
          .trim()
          .match(
            /^(\w+)\s+([a-z][\w() ,]*?)(?:\s+(?:not null|null|default|primary|references|check|unique|generated)\b.*|,)?$/i,
          );
        if (
          c &&
          !/^(constraint|primary|unique|foreign|check|like)$/i.test(c[1])
        ) {
          columns.push({
            name: c[1],
            type: c[2].trim(),
            nullable: !/not null/i.test(line),
            default: (line.match(/default\s+(.+?)(?:,|$)/i) || [])[1] || null,
          });
        }
      }
      tables.set(key(schema, m[2]), {
        schema,
        name: m[2],
        columns,
        indexes: [],
        file: rel(file),
      });
    }
    for (const m of sql.matchAll(
      /alter\s+table\s+(?:if\s+exists\s+)?(?:(\w+)\.)?(\w+)\s+add\s+column\s+(?:if\s+not\s+exists\s+)?(\w+)\s+([a-z][\w() ]*)/gi,
    )) {
      const t = tables.get(key(m[1] || "public", m[2]));
      if (t && !t.columns.some((c) => c.name === m[3]))
        t.columns.push({
          name: m[3],
          type: m[4].trim(),
          nullable: true,
          default: null,
        });
    }
    for (const m of sql.matchAll(
      /create\s+(unique\s+)?index\s+(?:concurrently\s+)?(?:if\s+not\s+exists\s+)?(\w+)\s+on\s+(?:(\w+)\.)?(\w+)[^;]*;/gi,
    )) {
      const t = tables.get(key(m[3] || "public", m[4]));
      if (t)
        t.indexes.push({
          name: m[2],
          definition: m[0].replace(/\s+/g, " ").trim(),
          unique: Boolean(m[1]),
          columns: [],
        });
    }
  }
  return [...tables.values()].map((t) => ({
    schema: t.schema,
    name: t.name,
    module: moduleOfTable(t.schema, t.name, map),
    kind: "table",
    columns: t.columns,
    primaryKey: [],
    indexes: sortBy(t.indexes, "name"),
    foreignKeys: [],
    constraints: [],
    rowEstimate: null,
    sizeBytes: null,
  }));
}

/** Retorna { tables, gaps, source }. */
export async function readSchema(map) {
  const url =
    process.env.OBS_DATABASE_URL ||
    "postgres://omni:omni_dev@127.0.0.1:5432/omni";
  const gaps = [];
  let tables;
  let source = "database";
  try {
    tables = await fromDatabase(url, map);
  } catch (error) {
    source = "migrations";
    tables = fromMigrations(map);
    gaps.push({
      kind: "db",
      item: "schema",
      reason: `banco inacessivel (${String(error.message).slice(0, 80)}); usado parse de migrations: sem PK/FK/constraints/tamanhos`,
    });
  }
  for (const t of tables) {
    if (!t.module)
      gaps.push({
        kind: "table-module",
        item: `${t.schema}.${t.name}`,
        reason:
          "sem mapeamento em config/module-map.json (schemaToModule/publicTablePrefixes)",
      });
    if (!t.module) t.module = "unknown";
  }
  return { tables: sortBy(tables, "schema", "name"), gaps, source };
}

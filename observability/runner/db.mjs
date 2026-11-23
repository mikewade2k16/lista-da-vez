// Fase banco: checagens estruturais (somente leitura) + dados capturados em obs.*.
// Cobertura de tabelas: uma tabela é "testada" se (a) entrou nas checagens
// estruturais (existe no catálogo e foi varrida por FK/PK/índices) E/OU (b)
// aparece em uma query capturada (obs.db_query_stats / db_slow_queries).
import { config } from "../config.mjs";
import { hostOf } from "./util.mjs";
import { captured } from "./db-captured.mjs";

const SYSTEM = `('pg_catalog','information_schema','pg_toast','obs')`;

export async function connectReadOnly(url = config.databaseUrl) {
  const { default: pg } = await import("pg");
  const client = new pg.Client({
    connectionString: url,
    options: "-c default_transaction_read_only=on",
    statement_timeout: 20000,
  });
  await client.connect();
  return client;
}

export async function tableExists(c, name) {
  return (
    (await c.query("select to_regclass($1) as t", [name])).rows[0].t !== null
  );
}
export async function columnsOf(c, schema, table) {
  const r = await c.query(
    "select column_name from information_schema.columns where table_schema=$1 and table_name=$2",
    [schema, table],
  );
  return new Set(r.rows.map((x) => x.column_name));
}

const SQL = {
  relations: `select n.nspname as schema, c.relname as name, c.relkind as kind, c.reltuples::bigint as rows,
      pg_total_relation_size(c.oid) as bytes
    from pg_class c join pg_namespace n on n.oid=c.relnamespace
    where c.relkind in ('r','p','v','m') and n.nspname not in ${SYSTEM} and n.nspname not like 'pg_%'`,
  fkNoIndex: `select n.nspname as schema, cl.relname as table, c.conname,
      (select array_agg(a.attname::text order by k.ord) from unnest(c.conkey) with ordinality k(attnum, ord)
        join pg_attribute a on a.attrelid=c.conrelid and a.attnum=k.attnum) as columns
    from pg_constraint c join pg_class cl on cl.oid=c.conrelid join pg_namespace n on n.oid=cl.relnamespace
    where c.contype='f' and n.nspname not in ${SYSTEM}
      and not exists (select 1 from pg_index i where i.indrelid=c.conrelid and i.indisvalid
        and (string_to_array(i.indkey::text,' ')::int2[])[1:array_length(c.conkey,1)] @> c.conkey
        and (string_to_array(i.indkey::text,' ')::int2[])[1:array_length(c.conkey,1)] <@ c.conkey)
    order by 1,2,3`,
  noPk: `select n.nspname as schema, c.relname as table from pg_class c join pg_namespace n on n.oid=c.relnamespace
    where c.relkind in ('r','p') and not c.relispartition and n.nspname not in ${SYSTEM} and n.nspname not like 'pg_%'
      and not exists (select 1 from pg_constraint k where k.conrelid=c.oid and k.contype='p') order by 1,2`,
  unusedIdx: `select s.schemaname as schema, s.relname as table, s.indexrelname as index, pg_relation_size(s.indexrelid) as bytes
    from pg_stat_user_indexes s join pg_index i on i.indexrelid=s.indexrelid
    where s.idx_scan=0 and not i.indisunique and not i.indisprimary and s.schemaname not in ${SYSTEM}
    order by bytes desc limit 40`,
  dupIdx: `select n.nspname as schema, c.relname as table, array_agg(ic.relname::text order by ic.relname) as indexes
    from pg_index i join pg_class ic on ic.oid=i.indexrelid join pg_class c on c.oid=i.indrelid join pg_namespace n on n.oid=c.relnamespace
    where n.nspname not in ${SYSTEM}
    group by n.nspname, c.relname, i.indrelid, i.indkey::text, i.indclass::text,
      coalesce(pg_get_expr(i.indpred,i.indrelid),''), coalesce(pg_get_expr(i.indexprs,i.indrelid),'')
    having count(*)>1 order by 1,2`,
  bigTables: `select n.nspname as schema, c.relname as table, pg_total_relation_size(c.oid) as bytes, c.reltuples::bigint as rows
    from pg_class c join pg_namespace n on n.oid=c.relnamespace
    where c.relkind in ('r','p') and n.nspname not in ${SYSTEM} and n.nspname not like 'pg_%' order by bytes desc limit 15`,
};

export function tablesInStatement(stmt) {
  const out = new Set();
  for (const m of stmt.matchAll(
    /\b(?:from|join|into|update)\s+("?[\w]+"?(?:\."?[\w]+"?)?)/gi,
  ))
    out.add(m[1].replace(/"/g, "").toLowerCase());
  return out;
}

export async function runDb(ctx) {
  const { inv, args } = ctx;
  const results = [];
  const out = {
    checks: {},
    notes: [],
    explains: [],
    topQueries: [],
    slowQueries: [],
    nPlusOne: [],
    bigTables: [],
    pgStatStatements: null,
  };
  const modOf = new Map(
    (inv?.tables || []).map((t) => [`${t.schema}.${t.name}`, t.module]),
  );
  const routeModule = new Map(
    (inv?.endpoints || []).map((e) => [e.id, e.module]),
  );
  const add = (status, kind, module, target, reason, metrics = {}) =>
    results.push({
      kind,
      module: module || "database",
      target,
      status,
      reason,
      metrics,
    });
  const summary = (key, label, rows, warnText) => {
    out.checks[key] = rows;
    add(
      rows.length ? "warn" : "pass",
      "db",
      "database",
      `db:${key}`,
      rows.length ? `${rows.length} ${warnText}` : "",
      { count: rows.length, label },
    );
  };

  let c;
  try {
    c = await connectReadOnly();
  } catch (e) {
    add(
      "skip",
      "db",
      "database",
      "db:conexao",
      `banco inacessível (${hostOf(config.databaseUrl.replace(/^postgres(ql)?:/, "http:"))}): ${e.message}`,
    );
    return { results, db: out, structural: new Set(), queried: new Set() };
  }
  const structural = new Set(),
    queried = new Set();
  try {
    const rels = (await c.query(SQL.relations)).rows;
    for (const r of rels) structural.add(`${r.schema}.${r.name}`);
    out.relationCount = rels.length;

    const fk = (await c.query(SQL.fkNoIndex)).rows;
    summary("fkWithoutIndex", "FKs sem índice", fk, "FKs sem índice");
    for (const r of fk)
      add(
        "warn",
        "db",
        modOf.get(`${r.schema}.${r.table}`),
        `db:fk-sem-indice:${r.schema}.${r.table}(${r.columns.join(",")})`,
        `FK ${r.conname} sem índice em (${r.columns.join(", ")})`,
        r,
      );
    const nopk = (await c.query(SQL.noPk)).rows;
    summary(
      "tablesWithoutPk",
      "Tabelas sem PK",
      nopk,
      "tabelas sem chave primária",
    );
    for (const r of nopk)
      add(
        "warn",
        "db",
        modOf.get(`${r.schema}.${r.table}`),
        `db:sem-pk:${r.schema}.${r.table}`,
        "tabela sem chave primária",
        r,
      );
    const unused = (await c.query(SQL.unusedIdx)).rows;
    summary(
      "unusedIndexes",
      "Índices nunca usados (idx_scan=0, não unique/pk)",
      unused,
      "índices sem uso desde o último reset de estatísticas",
    );
    const dup = (await c.query(SQL.dupIdx)).rows;
    summary(
      "duplicateIndexes",
      "Índices duplicados",
      dup,
      "grupos de índices duplicados",
    );
    for (const r of dup)
      add(
        "warn",
        "db",
        modOf.get(`${r.schema}.${r.table}`),
        `db:indice-duplicado:${r.schema}.${r.table}`,
        `índices duplicados: ${r.indexes.join(", ")}`,
        r,
      );
    out.bigTables = (await c.query(SQL.bigTables)).rows;
    add("pass", "db", "database", "db:maiores-tabelas", "", {
      top: out.bigTables.slice(0, 5),
    });

    // pg_stat_statements
    const ext = (
      await c.query(
        `select 1 from pg_extension where extname='pg_stat_statements'`,
      )
    ).rowCount;
    if (!ext) {
      out.notes.push("pg_stat_statements: extensão não instalada");
      add(
        "skip",
        "db",
        "database",
        "db:pg_stat_statements",
        "extensão não instalada",
      );
    } else {
      try {
        out.pgStatStatements = (
          await c.query(
            `select left(query,300) as query, calls, round(total_exec_time::numeric,1) as total_ms, round(mean_exec_time::numeric,2) as mean_ms from pg_stat_statements order by total_exec_time desc limit 15`,
          )
        ).rows;
        add("pass", "db", "database", "db:pg_stat_statements", "", {
          rows: out.pgStatStatements.length,
        });
      } catch (e) {
        add(
          "skip",
          "db",
          "database",
          "db:pg_stat_statements",
          `sem acesso: ${e.message}`,
        );
      }
    }

    // Dados capturados pela instrumentação.
    await captured(c, { out, add, routeModule, queried });
  } catch (e) {
    add(
      "fail",
      "db",
      "database",
      "db:checks",
      `erro nas checagens: ${e.message}`,
    );
  } finally {
    await c.end().catch(() => {});
  }
  void args;
  return { results, db: out, structural, queried };
}

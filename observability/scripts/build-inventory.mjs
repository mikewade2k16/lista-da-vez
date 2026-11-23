#!/usr/bin/env node
// Gera observability/inventory.json lendo codigo (Go/Nuxt), schema do Postgres e configs. Sem IA, deterministico.
// Uso: node scripts/build-inventory.mjs [--sync] [--check] [--out <arquivo>]
//   --sync   grava tambem em obs.inventory_items (se a tabela existir)
//   --check  sai com 1 se o inventory.json em disco difere do gerado (ignora generatedAt, git e metricas voláteis do banco)
import fs from "node:fs";
import path from "node:path";
import { execFileSync } from "node:child_process";
import pg from "pg";
import { ROOT, exists, readJson, sortBy } from "./inventory/util.mjs";
import { loadGoPackages, moduleOf } from "./inventory/go-pkg.mjs";
import { extractEndpoints } from "./inventory/go-routes.mjs";
import { countSqlCallSites } from "./inventory/go-deps.mjs";
import { discoverPages } from "./inventory/nuxt-pages.mjs";
import { attachPageApis } from "./inventory/page-apis.mjs";
import { readSchema } from "./inventory/db-schema.mjs";
import { extractJobs, extractIntegrations } from "./inventory/jobs.mjs";
import { readStack } from "./inventory/stack.mjs";

const OBS = path.resolve(path.dirname(new URL(import.meta.url).pathname), "..");
const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const outFile = args.includes("--out")
  ? path.resolve(args[args.indexOf("--out") + 1])
  : path.join(OBS, "inventory.json");

function git(...a) {
  try {
    return execFileSync("git", a, {
      cwd: ROOT,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return null;
  }
}

function buildModules(endpoints, pages, tables, queries) {
  const ids = new Set([
    ...endpoints.map((e) => e.module),
    ...pages.map((p) => p.module),
    ...tables.map((t) => t.module),
    ...Object.keys(queries),
  ]);
  const count = (list, id) => list.filter((x) => x.module === id).length;
  return [...ids].sort().map((id) => {
    const dir = `back/internal/modules/${id}`;
    return {
      id,
      backendDirs: exists(path.join(ROOT, dir))
        ? [dir]
        : id === "platform"
          ? ["back/internal/platform"]
          : [],
      endpoints: count(endpoints, id),
      pages: count(pages, id),
      tables: count(tables, id),
    };
  });
}

function sqlByModule(pkgs) {
  const byModule = new Map();
  for (const f of pkgs.files) {
    const id = moduleOf(f.rel);
    if (!byModule.has(id)) byModule.set(id, []);
    byModule.get(id).push(f);
  }
  const out = {};
  for (const [id, files] of [...byModule].sort(([a], [b]) =>
    a.localeCompare(b),
  ))
    out[id] = countSqlCallSites(files);
  return out;
}

async function syncToDatabase(inventory) {
  const url =
    process.env.OBS_DATABASE_URL ||
    "postgres://omni:omni_dev@127.0.0.1:5432/omni";
  const client = new pg.Client({
    connectionString: url,
    connectionTimeoutMillis: 4000,
  });
  try {
    await client.connect();
    await client.query("BEGIN");
    const kinds = {
      endpoint: inventory.endpoints.map((e) => [e.id, e.module, e]),
      page: inventory.pages.map((p) => [p.path, p.module, p]),
      table: inventory.tables.map((t) => [
        `${t.schema}.${t.name}`,
        t.module,
        t,
      ]),
      job: inventory.jobs.map((j) => [
        `${j.kind}:${j.name}:${j.file}:${j.line}`,
        j.module,
        j,
      ]),
      integration: inventory.integrations.map((i) => [i.id, "platform", i]),
    };
    let rows = 0;
    for (const [kind, items] of Object.entries(kinds)) {
      for (const [key, module, data] of items) {
        await client.query(
          `insert into obs.inventory_items (kind, key, module, data, updated_at) values ($1,$2,$3,$4,now())
           on conflict (kind, key) do update set module = excluded.module, data = excluded.data, updated_at = now()`,
          [kind, key, module, JSON.stringify(data)],
        );
        rows += 1;
      }
      await client.query(
        "delete from obs.inventory_items where kind = $1 and key <> all($2::text[])",
        [kind, items.map((i) => i[0])],
      );
    }
    await client.query("COMMIT");
    return { ok: true, rows };
  } catch (error) {
    await client.query("ROLLBACK").catch(() => {});
    return {
      ok: false,
      reason:
        error.code === "42P01"
          ? "tabela obs.inventory_items ainda nao existe (migration pendente)"
          : String(error.message),
    };
  } finally {
    await client.end().catch(() => {});
  }
}

/** Remove campos que mudam sem mudanca de codigo, para o --check. */
function stable(inv) {
  const copy = JSON.parse(JSON.stringify(inv));
  delete copy.generatedAt;
  delete copy.git;
  for (const t of copy.tables || []) {
    delete t.rowEstimate;
    delete t.sizeBytes;
  }
  return JSON.stringify(copy);
}

function summarize(inv, extra) {
  const by = (list, key) =>
    list.reduce((m, x) => ({ ...m, [x[key]]: (m[x[key]] || 0) + 1 }), {});
  const gapKinds = by(inv.gaps, "kind");
  const lines = [
    `inventory: ${path.relative(ROOT, outFile)}`,
    `endpoints: ${inv.endpoints.length} ${JSON.stringify(by(inv.endpoints, "auth"))}`,
    `paginas: ${inv.pages.length} (dinamicas: ${inv.pages.filter((p) => p.dynamic).length}, com apis: ${inv.pages.filter((p) => p.apis.length).length})`,
    `tabelas/views: ${inv.tables.length} (fonte: ${extra.dbSource})`,
    `jobs: ${inv.jobs.length} ${JSON.stringify(by(inv.jobs, "kind"))}`,
    `integracoes: ${inv.integrations.length}`,
    `modulos: ${inv.modules.length}`,
    `gaps: ${inv.gaps.length} ${JSON.stringify(gapKinds)}`,
  ];
  if (extra.sync)
    lines.push(
      `sync: ${extra.sync.ok ? `${extra.sync.rows} linhas em obs.inventory_items` : `ignorado - ${extra.sync.reason}`}`,
    );
  process.stdout.write(`${lines.join("\n")}\n`);
}

async function main() {
  const map = readJson(path.join(OBS, "config/module-map.json"));
  const pkgs = loadGoPackages();
  const { endpoints, gaps: routeGaps } = extractEndpoints(pkgs);
  const moduleOfRoute = (route) =>
    map.frontToBack[route.split("/")[1] || ""] ?? null;
  const { pages, gaps: pageGaps } = discoverPages(moduleOfRoute);
  const { unmatched } = attachPageApis(pages, endpoints);
  const schema = await readSchema(map);
  const jobs = extractJobs(pkgs, endpoints);
  const queries = sqlByModule(pkgs);

  const gaps = [...routeGaps, ...pageGaps, ...schema.gaps];
  for (const p of unmatched)
    gaps.push({
      kind: "page-api",
      item: p,
      reason:
        "literal de API no front sem endpoint correspondente no back (rota dinamica, prefixo ou endpoint inexistente)",
    });

  const inventory = {
    schemaVersion: 1,
    generatedAt: new Date().toISOString(),
    git: {
      sha: git("rev-parse", "HEAD"),
      branch: git("rev-parse", "--abbrev-ref", "HEAD"),
    },
    stack: readStack(),
    modules: buildModules(endpoints, pages, schema.tables, queries),
    endpoints,
    pages,
    tables: schema.tables,
    jobs,
    integrations: extractIntegrations(pkgs),
    queries,
    gaps: sortBy(gaps, "kind", "item", "reason"),
  };

  if (flag("--check")) {
    const onDisk = exists(outFile)
      ? JSON.parse(fs.readFileSync(outFile, "utf8"))
      : null;
    const fresh = onDisk && stable(onDisk) === stable(inventory);
    process.stdout.write(
      fresh
        ? "inventory.json atualizado\n"
        : "inventory.json DESATUALIZADO: rode npm run obs:inventory\n",
    );
    process.exit(fresh ? 0 : 1);
  }

  fs.writeFileSync(outFile, `${JSON.stringify(inventory, null, 2)}\n`);
  const sync = flag("--sync") ? await syncToDatabase(inventory) : null;
  summarize(inventory, { dbSource: schema.source, sync });
}

main().catch((error) => {
  process.stderr.write(`${error.stack || error}\n`);
  process.exit(2);
});

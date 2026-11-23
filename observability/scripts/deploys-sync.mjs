#!/usr/bin/env node
// Importa o histórico de deploys (.tmp/deploy/*/summary.json) para obs.deploys.
//   sem flags        só lista o que faria (não grava)
//   --sync           grava por upsert (idempotente por id)
//   --target=local   (padrão) docker exec no postgres local
//   --target=prod|staging   mesma conexão SSH do deploy (psql dentro do container)
//   --dry-run-db     só com target local: begin; migration 0330; upsert; select; rollback
import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { parseArgs } from "../runner/util.mjs";
import {
  buildUpsertSql,
  runSql,
  summaryToDeployRow,
} from "../../scripts/deploy/deploy-record.mjs";

const ROOT = join(dirname(fileURLToPath(import.meta.url)), "..", "..");
const DEPLOY_DIR = join(ROOT, ".tmp", "deploy");
const MIGRATION = join(
  ROOT,
  "back/internal/platform/database/migrations/0330_observability_health.sql",
);

const { flags } = parseArgs(process.argv.slice(2));
const sync = !!flags.sync;
const dry = !!flags["dry-run-db"];
const target = typeof flags.target === "string" ? flags.target : "local";
if (!["local", "prod", "staging"].includes(target)) {
  console.error("--target deve ser local, prod ou staging.");
  process.exit(2);
}
if (dry && target !== "local") {
  console.error("--dry-run-db só funciona com --target=local.");
  process.exit(2);
}

function minutes(ms) {
  if (ms == null) return "-";
  const s = Math.round(ms / 1000);
  return `${Math.floor(s / 60)}min${String(s % 60).padStart(2, "0")}s`;
}

const rows = [];
const skipped = [];
for (const id of existsSync(DEPLOY_DIR) ? readdirSync(DEPLOY_DIR).sort() : []) {
  const file = join(DEPLOY_DIR, id, "summary.json");
  if (!existsSync(file)) {
    if (/^\d{4}-\d{2}-\d{2}T/.test(id)) skipped.push(id);
    continue;
  }
  try {
    rows.push(summaryToDeployRow(id, JSON.parse(readFileSync(file, "utf8"))));
  } catch (e) {
    skipped.push(`${id} (${e.message})`);
  }
}

console.log(`${rows.length} deploy(s) em ${DEPLOY_DIR}:`);
for (const r of rows) {
  console.log(
    `  ${r.id}  ${r.command.padEnd(12)} ${minutes(r.duration_ms).padStart(8)}  ${r.status}  ${r.environment || "-"}  ${r.image_tag || "-"}`,
  );
}
for (const id of skipped)
  console.log(`  ignorado (sem summary.json válido): ${id}`);

if (!sync && !dry) {
  console.log(
    `\nNada gravado. Use --sync [--target=${target}] para gravar ou --dry-run-db para testar com rollback.`,
  );
  process.exit(0);
}
if (rows.length === 0) {
  console.log("Nada a importar.");
  process.exit(0);
}

try {
  if (dry) {
    const sql = [
      "begin;",
      readFileSync(MIGRATION, "utf8"),
      buildUpsertSql(rows),
      "select id, command, status, round(duration_ms) as ms, image_tag, jsonb_array_length(stages) as etapas from obs.deploys order by started_at;",
      "rollback;",
    ].join("\n");
    const out = await runSql("local", sql, {
      timeoutMs: 60000,
    });
    console.log(`\n--dry-run-db (rollback ao final):\n${out}`);
  } else {
    await runSql(target, buildUpsertSql(rows), { timeoutMs: 60000 });
    console.log(`\n${rows.length} deploy(s) gravado(s) em ${target}.`);
  }
} catch (e) {
  console.error(`\nnão gravou em ${target}: ${e.message}`);
  process.exit(2);
}

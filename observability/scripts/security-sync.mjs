#!/usr/bin/env node
// Sincroniza o catálogo de segurança e os achados da AUDITORIA com o banco.
//   sem flags      só imprime o que faria (não grava)
//   --sync         grava em obs.security_checks e obs.findings
//   --dry-run-db   faz tudo numa transação e dá rollback (aplica a 0330 dentro
//                  da transação se as tabelas ainda não existem)
import { readFileSync } from "node:fs";
import { parseArgs } from "../runner/util.mjs";
import { openWriteConn } from "../runner/persist.mjs";
import {
  AUDIT_FILE,
  CATALOG_FILE,
  MIGRATION_FILE,
  catalog,
  findings,
  tablesExist,
  upsertCatalog,
  upsertFindings,
} from "../runner/security-db.mjs";

const { flags } = parseArgs(process.argv.slice(2));
const sync = !!flags.sync;
const dry = !!flags["dry-run-db"];

const checks = catalog();
const list = findings();
const auto = checks.filter((c) => c.automated).length;
const bySev = list.reduce(
  (a, f) => ((a[f.severity] = (a[f.severity] || 0) + 1), a),
  {},
);
console.log(
  `catálogo: ${checks.length} itens (${auto} automatizados, ${checks.length - auto} não) em ${CATALOG_FILE}`,
);
console.log(
  `achados: ${list.length} em ${AUDIT_FILE} (${Object.entries(bySev)
    .map(([k, v]) => `${k}=${v}`)
    .join(" ")})`,
);

if (!sync && !dry) {
  console.log(
    "\nNada gravado. Use --sync para gravar ou --dry-run-db para testar com rollback.",
  );
  process.exit(0);
}

let c;
try {
  c = await openWriteConn();
} catch (e) {
  console.error(`sem conexão para gravar: ${e.message}`);
  process.exit(2);
}
try {
  await c.query("begin");
  let applied = false;
  if (!(await tablesExist(c))) {
    if (!dry) {
      console.error(
        "tabelas obs.security_checks/obs.findings não existem (migration 0330 não aplicada). Nada gravado.",
      );
      await c.query("rollback");
      process.exit(2);
    }
    await c.query(readFileSync(MIGRATION_FILE, "utf8"));
    applied = true;
    console.log("(dry-run) 0330 aplicada apenas dentro da transação.");
  }
  const nc = await upsertCatalog(c, checks);
  const nf = await upsertFindings(c, list);
  // segunda passada: não pode alterar nada (idempotência)
  await upsertCatalog(c, checks);
  await upsertFindings(c, list);
  const r = await c.query(
    `select (select count(*) from obs.security_checks)::int sc, (select count(*) from obs.findings)::int f`,
  );
  console.log(
    `upsert: ${nc} checagens, ${nf} achados; linhas na tabela: security_checks=${r.rows[0].sc} findings=${r.rows[0].f}${applied ? " (tabelas criadas só na transação)" : ""}`,
  );
  if (dry) {
    await c.query("rollback");
    console.log("rollback feito: nada foi gravado.");
  } else {
    await c.query("commit");
    console.log("gravado.");
  }
} catch (e) {
  await c.query("rollback").catch(() => {});
  console.error(`falhou: ${e.message}`);
  process.exitCode = 2;
} finally {
  await c.end().catch(() => {});
}

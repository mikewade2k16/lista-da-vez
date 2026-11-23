// Persistência da segurança: catálogo -> obs.security_checks, achados da
// AUDITORIA -> obs.findings e resultados do runner -> obs.security_checks.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { ROOT } from "../config.mjs";
import { openWriteConn } from "./persist.mjs";
import { loadCatalog, parseFindings } from "./security-lib.mjs";

export const CATALOG_FILE = join(ROOT, "config", "security-catalog.json");
export const AUDIT_FILE = join(ROOT, "reports", "AUDITORIA.md");
export const MIGRATION_FILE = join(
  ROOT,
  "..",
  "back/internal/platform/database/migrations/0330_observability_health.sql",
);

export const catalog = () => loadCatalog(CATALOG_FILE);
export const findings = () => parseFindings(readFileSync(AUDIT_FILE, "utf8"));

export async function tablesExist(c) {
  const r = await c.query(
    `select to_regclass('obs.security_checks') a, to_regclass('obs.findings') b`,
  );
  return !!(r.rows[0].a && r.rows[0].b);
}

// Upsert do catálogo: atualiza só título/categoria/padrão; nunca status,
// detail, evidence, checked_at nem source de quem já foi testado.
export async function upsertCatalog(c, checks) {
  for (const k of checks)
    await c.query(
      `insert into obs.security_checks (key, category, title, standard)
       values ($1,$2,$3,$4)
       on conflict (key) do update
         set category = excluded.category, title = excluded.title, standard = excluded.standard,
             updated_at = now()
       where (obs.security_checks.category, obs.security_checks.title, obs.security_checks.standard)
             is distinct from (excluded.category, excluded.title, excluded.standard)`,
      [k.key, k.category, k.title, k.standard],
    );
  return checks.length;
}

// Achados: status editado (fixed/accepted) nunca é sobrescrito.
export async function upsertFindings(c, list) {
  for (const f of list)
    await c.query(
      `insert into obs.findings (id, severity, title, area, location)
       values ($1,$2,$3,$4,$5)
       on conflict (id) do update
         set severity = excluded.severity, title = excluded.title,
             area = excluded.area, location = excluded.location, updated_at = now()
       where (obs.findings.severity, obs.findings.title, obs.findings.area, obs.findings.location)
             is distinct from (excluded.severity, excluded.title, excluded.area, excluded.location)`,
      [f.id, f.severity, f.title, f.area, f.location],
    );
  return list.length;
}

// Grava o resultado de uma checagem automática. 'not_tested' (não deu para
// rodar) só atualiza o motivo se a checagem ainda nunca foi concluída.
export async function recordOutcomes(c, outcomes, byKey) {
  let n = 0;
  for (const o of outcomes) {
    const k = byKey.get(o.key);
    if (!k) continue;
    await upsertCatalog(c, [k]);
    if (o.status === "not_tested") {
      await c.query(
        `update obs.security_checks set detail=$2, source='runner', updated_at=now()
         where key=$1 and status='not_tested'`,
        [o.key, o.detail],
      );
    } else {
      await c.query(
        `update obs.security_checks
           set status=$2, detail=$3, evidence=$4::jsonb, source='runner',
               checked_at=now(), updated_at=now()
         where key=$1`,
        [o.key, o.status, o.detail, JSON.stringify(o.evidence || {})],
      );
    }
    n++;
  }
  return n;
}

// Usado pelo runner. Tolerante a ausência das tabelas (migration 0330).
export async function storeOutcomes(outcomes) {
  let c;
  try {
    c = await openWriteConn();
  } catch (e) {
    return { ok: false, why: e.message };
  }
  try {
    if (!(await tablesExist(c)))
      return {
        ok: false,
        why: "tabelas obs.security_checks/obs.findings não existem (migration 0330 não aplicada)",
      };
    const byKey = new Map(catalog().map((k) => [k.key, k]));
    await c.query("begin");
    const rows = await recordOutcomes(c, outcomes, byKey);
    await c.query("commit");
    return { ok: true, rows };
  } catch (e) {
    await c.query("rollback").catch(() => {});
    return { ok: false, why: e.message };
  } finally {
    await c.end().catch(() => {});
  }
}

#!/usr/bin/env node
// Worker: heartbeat em obs.workers (10s) + consome obs.run_requests (queued -> running -> done/failed).
// Recusa iniciar em produção.
import { spawn } from "node:child_process";
import { hostname } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { config } from "../config.mjs";
import { looksLikeProduction } from "./safety.mjs";
import { openWriteConn, columns } from "./persist.mjs";
import { sleep } from "./util.mjs";

const HERE = dirname(fileURLToPath(import.meta.url));
const workerId = `${hostname()}-${process.pid}`;
const TAIL = 8000;
const log = (m) =>
  console.log(`[worker ${new Date().toISOString().slice(11, 19)}] ${m}`);

const onlyList = (v) =>
  (Array.isArray(v) ? v : String(v || "").split(","))
    .map((s) => s.replace(/[{}"]/g, "").trim())
    .filter((s) => /^[a-z]+$/.test(s));
const safeModule = (v) =>
  typeof v === "string" && /^[A-Za-z0-9_-]{1,64}$/.test(v) ? v : null;

async function heartbeat(c) {
  await c.query(
    `insert into obs.workers (id, host, last_seen) values ($1,$2,now())
    on conflict (id) do update set last_seen = now(), host = excluded.host`,
    [workerId, hostname()],
  );
}

async function claim(c, cols) {
  const order =
    ["created_at", "requested_at", "id"].find((k) => cols.has(k)) || "id";
  const started = cols.has("started_at") ? ", started_at = now()" : "";
  const r =
    await c.query(`update obs.run_requests set status='running'${started}
    where id = (select id from obs.run_requests where status='queued' order by ${order} limit 1 for update skip locked) returning *`);
  return r.rows[0] || null;
}

function execRun(req) {
  const args = [join(HERE, "run.mjs"), "--no-fail-on-regression"];
  const mod = safeModule(req.module);
  if (mod) args.push(`--module=${mod}`);
  const only = onlyList(req.only_kinds ?? req.only);
  if (only.length) args.push(`--only=${only.join(",")}`);
  return new Promise((resolve) => {
    let out = "";
    const child = spawn(process.execPath, args, {
      env: process.env,
      cwd: join(HERE, ".."),
    });
    const add = (b) => {
      out = (out + b).slice(-TAIL * 4);
    };
    child.stdout.on("data", add);
    child.stderr.on("data", add);
    child.on("error", (e) => resolve({ code: 2, out: String(e) }));
    child.on("close", (code) => resolve({ code, out }));
  });
}

async function main() {
  const why = looksLikeProduction();
  if (/^prod/i.test(config.env) || why.some((w) => w.startsWith("env="))) {
    console.error(`RECUSADO: worker não roda em produção (${why.join("; ")})`);
    process.exit(2);
  }
  const c = await openWriteConn();
  const ok = await c.query(
    `select to_regclass('obs.workers') w, to_regclass('obs.run_requests') r`,
  );
  if (!ok.rows[0].w || !ok.rows[0].r) {
    console.error(
      "tabelas obs.workers/obs.run_requests não existem (migration 0329 não aplicada)",
    );
    process.exit(2);
  }
  const cols = await columns(c, "obs", "run_requests");
  log(`iniciado (${workerId}); heartbeat 10s; env=${config.env}`);
  let stop = false;
  for (const s of ["SIGINT", "SIGTERM"])
    process.on(s, () => {
      stop = true;
    });
  let lastBeat = 0,
    busy = false;
  const beat = setInterval(() => {
    if (Date.now() - lastBeat >= 9500) {
      lastBeat = Date.now();
      heartbeat(c).catch((e) => log(`heartbeat falhou: ${e.message}`));
    }
  }, 2000);
  await heartbeat(c);
  lastBeat = Date.now();
  while (!stop) {
    try {
      const req = busy ? null : await claim(c, cols);
      if (!req) {
        await sleep(2000);
        continue;
      }
      busy = true;
      log(
        `executando pedido ${req.id} módulo=${req.module || "*"} only=${req.only_kinds ?? req.only ?? "*"}`,
      );
      const { code, out } = await execRun(req);
      const runId = (out.match(/^Run:\s*(\S+)/m) || [])[1] || null;
      const fin = cols.has("finished_at") ? ", finished_at = now()" : "";
      await c.query(
        `update obs.run_requests set status=$2, run_id=$3, log=$4${fin} where id=$1`,
        [
          req.id,
          code === 0 || code === 1 ? "done" : "failed",
          runId,
          out.slice(-TAIL),
        ],
      );
      log(`pedido ${req.id}: exit ${code} run=${runId}`);
      busy = false;
    } catch (e) {
      busy = false;
      log(`erro: ${e.message}`);
      await sleep(5000);
    }
  }
  clearInterval(beat);
  await c.end().catch(() => {});
}

main().catch((e) => {
  console.error(e);
  process.exit(2);
});

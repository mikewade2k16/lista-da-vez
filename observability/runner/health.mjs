// Fase saúde: /healthz, containers omni-*, dependências locais, npm audit, govulncheck.
import { join } from "node:path";
import { config, REPO } from "../config.mjs";
import { sh, round } from "./util.mjs";

const mk = (target, status, reason = "", metrics = {}) => ({
  kind: "health",
  module: "infra",
  target,
  status,
  reason,
  metrics,
});

async function ping(url, timeoutMs = 4000) {
  const t0 = performance.now();
  try {
    const res = await fetch(url, {
      signal: AbortSignal.timeout(timeoutMs),
      redirect: "manual",
    });
    await res.arrayBuffer();
    return { status: res.status, ms: round(performance.now() - t0) };
  } catch (e) {
    return {
      status: 0,
      ms: round(performance.now() - t0),
      error: String(e?.cause?.code || e.message),
    };
  }
}

async function audit(dir, label) {
  const r = await sh("npm", ["audit", "--omit=dev", "--json"], {
    cwd: dir,
    timeout: config.health.auditTimeoutMs,
  });
  if (r.killed)
    return mk(
      `audit:${label}`,
      "skip",
      "npm audit excedeu 60s (offline/lento)",
    );
  let j;
  try {
    j = JSON.parse(r.stdout);
  } catch {
    return mk(
      `audit:${label}`,
      "skip",
      `npm audit sem saída JSON: ${(r.stderr || "").slice(0, 120)}`,
    );
  }
  if (j.error)
    return mk(
      `audit:${label}`,
      "skip",
      /ENOTFOUND|EAI_AGAIN|ENETUNREACH|network|offline/i.test(
        JSON.stringify(j.error),
      )
        ? "offline: registro npm inacessível"
        : `npm audit: ${j.error.summary || j.error.code}`,
    );
  const v = j.metadata?.vulnerabilities || {};
  const bad = (v.high || 0) + (v.critical || 0);
  return mk(
    `audit:${label}`,
    bad ? "warn" : "pass",
    bad ? `${v.critical || 0} críticas, ${v.high || 0} altas (prod)` : "",
    { vulnerabilities: v },
  );
}

async function govuln() {
  if (!config.health.govulncheck)
    return mk(
      "govulncheck",
      "skip",
      "desabilitado (--no-govulncheck / OBS_GOVULNCHECK=0)",
    );
  const img = config.health.govulncheckImage;
  const has = await sh("docker", ["image", "inspect", img], { timeout: 10000 });
  if (!has.ok)
    return mk(
      "govulncheck",
      "skip",
      `imagem ${img} não existe localmente (ou docker indisponível)`,
    );
  const script =
    "go install golang.org/x/vuln/cmd/govulncheck@latest >/dev/null 2>&1 && govulncheck -format json ./... || true";
  const r = await sh(
    "docker",
    [
      "run",
      "--rm",
      "-v",
      `${join(REPO, "back")}:/src:ro`,
      "-v",
      "omni_gomodcache:/go/pkg/mod",
      "-w",
      "/src",
      img,
      "sh",
      "-c",
      script,
    ],
    { timeout: 300000 },
  );
  if (r.killed) return mk("govulncheck", "skip", "govulncheck excedeu 5 min");
  const found = new Set();
  for (const m of r.stdout.matchAll(
    /"finding":\s*\{[^]*?"osv":\s*"(GO-[\d-]+)"/g,
  ))
    found.add(m[1]);
  if (!r.stdout.includes('"config"') && !found.size)
    return mk(
      "govulncheck",
      "skip",
      `sem saída do govulncheck (rede/instalação): ${r.stderr.slice(0, 120)}`,
    );
  return mk(
    "govulncheck",
    found.size ? "warn" : "pass",
    found.size
      ? `${found.size} vulnerabilidade(s) alcançável(is): ${[...found].slice(0, 5).join(", ")}`
      : "",
    { osv: [...found] },
  );
}

export async function runHealth() {
  const out = [];
  const h = await ping(`${config.apiUrl}/healthz`);
  out.push(
    mk(
      "GET /healthz",
      h.status === 200 ? "pass" : "fail",
      h.status === 200 ? "" : `HTTP ${h.status} ${h.error || ""}`,
      h,
    ),
  );
  const w = await ping(config.webUrl);
  out.push(
    mk(
      "web",
      w.status > 0 && w.status < 500 ? "pass" : "fail",
      w.status ? "" : `web inacessível: ${w.error}`,
      w,
    ),
  );
  const ps = await sh(
    "docker",
    ["ps", "--filter", "name=omni-", "--format", "{{.Names}}|{{.Status}}"],
    { timeout: 10000 },
  );
  if (!ps.ok) out.push(mk("docker", "skip", "docker indisponível"));
  else {
    for (const line of ps.stdout.split("\n").filter(Boolean)) {
      const [name, st] = line.split("|");
      const bad = /unhealthy|Restarting|Dead/i.test(st);
      out.push(
        mk(`container:${name}`, bad ? "fail" : "pass", bad ? st : "", {
          status: st,
        }),
      );
    }
  }
  for (const d of config.health.externals) {
    const p = await ping(d.url);
    out.push(
      mk(
        `dep:${d.name}`,
        p.status > 0 && p.status < 500 ? "pass" : "warn",
        p.status
          ? p.status >= 500
            ? `HTTP ${p.status}`
            : ""
          : `inacessível: ${p.error}`,
        { ...p, url: d.url },
      ),
    );
  }
  out.push(await audit(join(REPO, "web"), "web"));
  out.push(await audit(join(REPO, "observability"), "observability"));
  out.push(await govuln());
  return out;
}

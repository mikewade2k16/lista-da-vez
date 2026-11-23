// Cobertura: % de endpoints/páginas/tabelas realmente testados + não testados com motivo.
// Endpoint/página "testado" = rodou e não foi skip. Tabela "testada" = varrida pelas
// checagens estruturais (existe no catálogo) E/OU citada em query capturada.
export function computeCoverage({
  inv,
  results,
  structural,
  queried,
  filters,
}) {
  const byTarget = (kind) =>
    new Map(results.filter((r) => r.kind === kind).map((r) => [r.target, r]));
  const mkCov = () => ({ total: 0, tested: 0, pct: 0, untested: [] });
  const out = {
    endpoints: mkCov(),
    pages: mkCov(),
    tables: { ...mkCov(), queried: 0, queriedPct: 0 },
  };
  const byModule = {};
  const bump = (mod, kind, tested) => {
    const m = (byModule[mod] ||= {
      endpoints: [0, 0],
      pages: [0, 0],
      tables: [0, 0],
    });
    m[kind][1]++;
    if (tested) m[kind][0]++;
  };
  const api = byTarget("api"),
    pg = byTarget("page");
  const inScope = (mod) =>
    !filters.modules.length || filters.modules.includes(mod);
  const phases = filters.only;
  for (const e of inv.endpoints || []) {
    if (!inScope(e.module)) continue;
    out.endpoints.total++;
    const r = api.get(e.id);
    const tested = !!r && r.status !== "skip";
    bump(e.module, "endpoints", tested);
    if (tested) out.endpoints.tested++;
    else
      out.endpoints.untested.push({
        key: e.id,
        module: e.module,
        reason:
          r?.reason ||
          (phases.length && !phases.includes("api")
            ? "fase api não executada neste run"
            : filters.endpoint
              ? "fora do filtro --endpoint"
              : "não executado"),
      });
  }
  for (const p of inv.pages || []) {
    if (!inScope(p.module)) continue;
    out.pages.total++;
    const r = pg.get(p.path);
    const tested = !!r && r.status !== "skip";
    bump(p.module || "core", "pages", tested);
    if (tested) out.pages.tested++;
    else
      out.pages.untested.push({
        key: p.path,
        module: p.module,
        reason:
          r?.reason ||
          (phases.length && !phases.includes("pages")
            ? "fase pages não executada neste run"
            : filters.page
              ? "fora do filtro --page"
              : "não executado"),
      });
  }
  const dbRan = structural.size > 0;
  for (const t of inv.tables || []) {
    if (!inScope(t.module)) continue;
    const key = `${t.schema}.${t.name}`;
    out.tables.total++;
    const q =
      queried.has(key) ||
      queried.has(t.name.toLowerCase()) ||
      queried.has(key.toLowerCase());
    const s = structural.has(key);
    const tested = s || q;
    bump(t.module, "tables", tested);
    if (q) out.tables.queried++;
    if (tested) out.tables.tested++;
    else
      out.tables.untested.push({
        key,
        module: t.module,
        reason: dbRan
          ? "tabela do inventário ausente no banco conectado"
          : "fase db não executada (ou banco inacessível)",
      });
  }
  for (const k of ["endpoints", "pages", "tables"]) {
    const c = out[k];
    c.pct = c.total ? Math.round((c.tested / c.total) * 1000) / 10 : 0;
  }
  out.tables.queriedPct = out.tables.total
    ? Math.round((out.tables.queried / out.tables.total) * 1000) / 10
    : 0;
  out.byModule = Object.fromEntries(
    Object.entries(byModule).map(([m, v]) => [
      m,
      Object.fromEntries(
        Object.entries(v).map(([k, [t, n]]) => [
          k,
          {
            tested: t,
            total: n,
            pct: n ? Math.round((t / n) * 1000) / 10 : null,
          },
        ]),
      ),
    ]),
  );
  return out;
}

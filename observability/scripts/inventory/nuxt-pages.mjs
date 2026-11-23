// Descoberta de paginas Nuxt (web/app/pages e web/layers/*/pages) e metadados de definePageMeta.
import path from "node:path";
import { ROOT, readText, rel, walk, sortBy } from "./util.mjs";
import { maskGo, matchClose } from "./go-lex.mjs";

const WEB = path.join(ROOT, "web");

export function pageDirs() {
  const dirs = [path.join(WEB, "app/pages")];
  const layers = path.join(WEB, "layers");
  for (const name of walk(layers, (f) => /\/pages\/.*\.vue$/.test(f)).map(
    (f) => f.split("/pages/")[0],
  )) {
    const dir = path.join(name, "pages");
    if (!dirs.includes(dir)) dirs.push(dir);
  }
  return dirs;
}

/** Converte o caminho do arquivo em rota Nuxt. */
export function routeFromFile(relToPages) {
  const noExt = relToPages.replace(/\.vue$/, "");
  const parts = noExt.split("/").map((seg) => {
    if (seg === "index") return "";
    const catchAll = seg.match(/^\[\.\.\.(\w+)\]$/);
    if (catchAll) return `:${catchAll[1]}(.*)*`;
    const dyn = seg.match(/^\[(\w+)\]$/);
    return dyn ? `:${dyn[1]}` : seg;
  });
  const route = `/${parts.filter((p) => p !== "").join("/")}`;
  return { path: route, dynamic: /\[/.test(noExt) };
}

function stringProp(block, key) {
  const m = block.match(
    new RegExp(`\\b${key}\\s*:\\s*(?:'([^']*)'|"([^"]*)")`),
  );
  return m ? (m[1] ?? m[2]) : null;
}

/** Extrai layout, workspaceId, pageLabel e middleware do definePageMeta (sem executar). */
export function parsePageMeta(src) {
  const idx = src.indexOf("definePageMeta(");
  if (idx < 0) return {};
  const open = src.indexOf("(", idx);
  const close = matchClose(maskGo(src), open);
  const block = src.slice(open, close);
  const meta = {
    layout: stringProp(block, "layout"),
    workspaceId: stringProp(block, "workspaceId"),
    pageLabel: stringProp(block, "pageLabel"),
  };
  if (/\blayout\s*:\s*false/.test(block)) meta.layout = false;
  const mw = block.match(
    /\bmiddleware\s*:\s*(\[[\s\S]*?\]|'[^']*'|"[^"]*"|\([^)]*\)\s*=>[^\n,]*)/,
  );
  if (mw) {
    const names = [...mw[1].matchAll(/['"]([\w-]+)['"]/g)].map((m) => m[1]);
    meta.middleware = names.length ? names : ["(inline)"];
  }
  return meta;
}

export function discoverPages(moduleOfRoute) {
  const pages = [];
  const gaps = [];
  for (const dir of pageDirs()) {
    for (const abs of walk(dir, (f) => f.endsWith(".vue"))) {
      const relToPages = path.relative(dir, abs).split(path.sep).join("/");
      const route = routeFromFile(relToPages);
      const meta = parsePageMeta(readText(abs));
      const requiresLogin = !(
        route.path === "/offline" || route.path.startsWith("/auth")
      );
      const mod = moduleOfRoute(route.path, rel(abs));
      if (!mod)
        gaps.push({
          kind: "page-module",
          item: route.path,
          reason:
            "segmento de rota ausente em config/module-map.json (frontToBack)",
        });
      pages.push({
        path: route.path,
        file: rel(abs),
        module: mod || "unknown",
        requiresLogin,
        layout: meta.layout ?? null,
        workspaceId: meta.workspaceId ?? null,
        pageLabel: meta.pageLabel ?? null,
        middleware: meta.middleware ?? [],
        dynamic: route.dynamic,
        apis: [],
      });
    }
  }
  return { pages: sortBy(pages, "path", "file"), gaps };
}

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  evalHeaders,
  tamperSignature,
  tamperPayload,
  algNone,
  corsVerdict,
  redirectsExternal,
  isDirectoryListing,
  unescapedReflection,
  scanText,
  isIgnoredPath,
  parseFindings,
  normalizeSeverity,
  loadCatalog,
} from "./security-lib.mjs";

const jwt = `${Buffer.from('{"alg":"HS256"}').toString("base64url")}.${Buffer.from('{"sub":"1"}').toString("base64url")}.abcDEF123`;

test("evalHeaders aponta o que falta", () => {
  const h = new Headers({
    "x-content-type-options": "nosniff",
    "x-frame-options": "DENY",
  });
  assert.deepEqual(evalHeaders(h).missing, [
    "Content-Security-Policy",
    "Referrer-Policy",
  ]);
  assert.ok(
    evalHeaders(h, { https: true }).missing.includes(
      "Strict-Transport-Security",
    ),
  );
});

test("JWT adulterado difere do original e alg none não tem assinatura", () => {
  assert.notEqual(tamperSignature(jwt), jwt);
  assert.equal(tamperSignature(jwt).split(".").length, 3);
  assert.notEqual(tamperPayload(jwt), jwt);
  assert.ok(algNone(jwt).endsWith("."));
  assert.equal(tamperSignature("abc"), null);
});

test("corsVerdict", () => {
  const h = (m) => (n) => m[n] || "";
  assert.equal(
    corsVerdict(
      "https://e.x",
      h({ "access-control-allow-origin": "https://e.x" }),
    ).ok,
    false,
  );
  assert.equal(corsVerdict("https://e.x", h({})).ok, true);
  assert.equal(
    corsVerdict(
      "https://e.x",
      h({
        "access-control-allow-origin": "*",
        "access-control-allow-credentials": "true",
      }),
    ).ok,
    false,
  );
});

test("listagem de diretório e XSS refletido", () => {
  assert.ok(
    isDirectoryListing(
      "text/html; charset=utf-8",
      '<pre>\n<a href="a.png">a.png</a>\n</pre>',
    ),
  );
  assert.ok(!isDirectoryListing("application/json", '<pre><a href="x">'));
  assert.ok(unescapedReflection("text/html", "x<script>m1</script>", "m1"));
  assert.ok(
    !unescapedReflection("application/json", "<script>m1</script>", "m1"),
  );
  assert.ok(
    !unescapedReflection("text/html", "&lt;script&gt;m1&lt;/script&gt;", "m1"),
  );
});

test("scanText nunca devolve o valor e ignora placeholders", () => {
  const aws = "AKIA" + "ABCDEFGHIJKLMNOP";
  const hits = scanText(
    `ok\nconst k = "${aws}"\npassword: "changeme-please"\npassword = "Zx9fQ2mPq7"`,
  );
  assert.deepEqual(
    hits.map((h) => [h.line, h.rule]),
    [
      [2, "aws-access-key"],
      [4, "password-literal"],
    ],
  );
  assert.ok(!JSON.stringify(hits).includes(aws));
  assert.ok(isIgnoredPath("back/.env.example") && isIgnoredPath("a/b_test.go"));
  assert.ok(!isIgnoredPath("back/cmd/api/main.go"));
});

test("parseFindings lê a AUDITORIA real", () => {
  const md = readFileSync(
    new URL("../reports/AUDITORIA.md", import.meta.url),
    "utf8",
  );
  const f = parseFindings(md);
  assert.ok(f.length >= 30);
  assert.equal(new Set(f.map((x) => x.id)).size, f.length);
  assert.ok(
    f.every(
      (x) =>
        ["critical", "high", "medium", "low"].includes(x.severity) &&
        x.location,
    ),
  );
  assert.equal(f.find((x) => x.id === "C-01").severity, "critical");
  assert.equal(normalizeSeverity("Médio"), "medium");
});

test("catálogo válido", () => {
  const c = loadCatalog(
    new URL("../config/security-catalog.json", import.meta.url).pathname,
  );
  assert.ok(c.length > 60);
  assert.ok(
    c
      .filter((x) => x.automated)
      .every((x) => x.howTested.startsWith("automático")),
  );
});

test("redirectsExternal só acusa host diferente", () => {
  const base = "http://localhost:3003/login?x=1";
  assert.ok(
    !redirectsExternal("/auth/login?redirect=https://evil.example", base),
  );
  assert.ok(redirectsExternal("https://evil.example/x", base));
  assert.ok(redirectsExternal("//evil.example", base));
  assert.ok(!redirectsExternal("", base));
  assert.ok(!redirectsExternal("http://localhost:3000/auth/login", base));
});

test("findingStatus le corrigido e parcial da AUDITORIA", async () => {
  const { findingStatus } = await import("./security-lib.mjs");
  assert.equal(
    findingStatus("x\n- **Status: CORRIGIDO em 2026-11-23.** ok"),
    "fixed",
  );
  assert.equal(findingStatus("- **Status 2026-11-23: corrigido.** x"), "fixed");
  assert.equal(
    findingStatus("- **Status 2026-11-23 (parcial):** listagem"),
    "open",
  );
  assert.equal(findingStatus("sem linha de status"), "open");
});

test("upsertFindings nunca sobrescreve owner/due_date/note/updated_by (fila 2.4)", () => {
  const src = readFileSync(new URL("./security-db.mjs", import.meta.url), "utf8");
  const start = src.indexOf("export async function upsertFindings");
  const body = src.slice(start, src.indexOf("export async function recordOutcomes"));
  const setClause = body.slice(body.indexOf("do update"), body.indexOf("where (obs.findings"));
  for (const col of ["owner", "due_date", "note", "updated_by"])
    assert.ok(!setClause.includes(col), `SET nao pode tocar ${col}`);
});

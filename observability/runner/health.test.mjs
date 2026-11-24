import { test } from "node:test";
import assert from "node:assert/strict";
import { reachableOsv } from "./health.mjs";

test("reachableOsv conta só findings com função no trace", () => {
  const stream = [
    '{"config":{"scanner_name":"govulncheck"}}',
    '{"finding":{"osv":"GO-1","trace":[{"module":"x","package":"x/y"}]}}',
    '{"finding":{"osv":"GO-2","trace":[{"module":"x","package":"x/y","function":"Do"}]}}',
    '{"osv":{"id":"GO-3","summary":"texto com { chave } e \\"aspas\\""}}',
  ].join("\n");
  assert.deepEqual([...reachableOsv(stream)], ["GO-2"]);
});

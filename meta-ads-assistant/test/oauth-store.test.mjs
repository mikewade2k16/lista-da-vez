import assert from "node:assert/strict";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { isAbsolute, join, relative } from "node:path";
import test from "node:test";

import { InvalidAccountIdError } from "../src/account-id.mjs";
import { createOAuthStore } from "../src/oauth-store.mjs";

const ACCOUNT_A = "11111111-1111-4111-8111-111111111111";
const ACCOUNT_B = "22222222-2222-4222-8222-222222222222";

test("persiste tokens em paths distintos e confinados por account", () => {
  const root = mkdtempSync(join(tmpdir(), "omni-meta-oauth-"));
  try {
    const store = createOAuthStore(root);
    store.saveTokens(ACCOUNT_A, {
      access_token: "token-a",
      refresh_token: "refresh-a",
      expiresInSec: 3600,
      clientId: "client-a",
      tokenEndpoint: "https://meta.example/token-a",
    });
    store.saveTokens(ACCOUNT_B, {
      access_token: "token-b",
      refresh_token: "refresh-b",
      expiresInSec: 3600,
    });

    assert.equal(store.loadTokens(ACCOUNT_A).access_token, "token-a");
    assert.equal(store.loadTokens(ACCOUNT_A).client_id, "client-a");
    assert.equal(
      store.loadTokens(ACCOUNT_A).token_endpoint,
      "https://meta.example/token-a",
    );
    assert.equal(store.loadTokens(ACCOUNT_B).access_token, "token-b");
    const pathA = store.paths.tokensFile(ACCOUNT_A);
    const pathB = store.paths.tokensFile(ACCOUNT_B);
    assert.notEqual(pathA, pathB);
    for (const filePath of [pathA, pathB]) {
      const rel = relative(root, filePath);
      assert.equal(rel.startsWith(".."), false);
      assert.equal(isAbsolute(rel), false);
    }
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("store rejeita account que tentaria escapar do cofre", () => {
  const root = mkdtempSync(join(tmpdir(), "omni-meta-oauth-"));
  try {
    const store = createOAuthStore(root);
    assert.throws(
      () => store.paths.tokensFile("../../tokens"),
      InvalidAccountIdError,
    );
    assert.throws(() => store.loadTokens("not-a-uuid"), InvalidAccountIdError);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

import assert from "node:assert/strict";
import test from "node:test";

import {
  InvalidAccountIdError,
  normalizeAccountId,
  requireAccountId,
} from "../src/account-id.mjs";

const ACCOUNT_ID = "11111111-1111-4111-8111-111111111111";

test("normaliza UUID de account para lowercase", () => {
  assert.equal(
    normalizeAccountId(`  ${ACCOUNT_ID.toUpperCase()}  `),
    ACCOUNT_ID,
  );
  assert.equal(requireAccountId(ACCOUNT_ID), ACCOUNT_ID);
});

test("rejeita account ausente, UUID malformado e tentativa de traversal", () => {
  for (const value of [
    "",
    undefined,
    "../tokens",
    `${ACCOUNT_ID}/../../tokens`,
    "not-a-uuid",
  ]) {
    assert.equal(normalizeAccountId(value), "");
    assert.throws(() => requireAccountId(value), InvalidAccountIdError);
  }
});

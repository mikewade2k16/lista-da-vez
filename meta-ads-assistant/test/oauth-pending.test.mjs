import assert from "node:assert/strict";
import test from "node:test";

import { PendingOAuthRegistry } from "../src/oauth-pending.mjs";

const ACCOUNT_A = "11111111-1111-4111-8111-111111111111";
const ACCOUNT_B = "22222222-2222-4222-8222-222222222222";

test("fluxos PKCE pendentes permanecem isolados por account e state", () => {
  const registry = new PendingOAuthRegistry();
  const flowA = { state: "state-a" };
  const flowB = { state: "state-b" };
  registry.set(ACCOUNT_A, flowA);
  registry.set(ACCOUNT_B, flowB);

  assert.equal(registry.get(ACCOUNT_A), flowA);
  assert.equal(registry.get(ACCOUNT_B), flowB);
  assert.deepEqual(registry.findByState("state-a"), {
    accountId: ACCOUNT_A,
    flow: flowA,
  });
  assert.deepEqual(registry.findByState("state-b"), {
    accountId: ACCOUNT_B,
    flow: flowB,
  });
  assert.equal(registry.findByState("unknown"), null);

  assert.equal(registry.delete(ACCOUNT_A, flowB), false);
  assert.equal(registry.get(ACCOUNT_A), flowA);
  assert.equal(registry.delete(ACCOUNT_A, flowA), true);
  assert.equal(registry.get(ACCOUNT_A), null);
  assert.equal(registry.get(ACCOUNT_B), flowB);
});

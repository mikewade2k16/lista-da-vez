import assert from "node:assert/strict";
import test from "node:test";

import { buildQueryOptions } from "../src/agent.mjs";
import { createOmniBridgeClient } from "../src/omni-tools.mjs";

const ACCOUNT_A = "11111111-1111-4111-8111-111111111111";
const ACCOUNT_B = "22222222-2222-4222-8222-222222222222";

test("bridge Omni captura accountId por closure sem contexto global", async () => {
  const calls = [];
  const bridgeCall = async (path) => {
    calls.push(path);
    return { ok: true, data: { accounts: [], media: [] } };
  };
  const bridgeA = createOmniBridgeClient(ACCOUNT_A, bridgeCall);
  const bridgeB = createOmniBridgeClient(ACCOUNT_B, bridgeCall);

  await bridgeA.getAccounts();
  await bridgeB.getRecentPosts({ limit: 7 });
  await bridgeA.getRecentPosts({ limit: 3 });

  assert.match(calls[0], new RegExp(`accountId=${ACCOUNT_A}`));
  assert.match(calls[1], new RegExp(`accountId=${ACCOUNT_B}`));
  assert.match(calls[2], new RegExp(`accountId=${ACCOUNT_A}`));
});

test("gate final limita leituras Meta a conta de anuncios autorizada", async () => {
  let policy = { mode: "read", adAccountId: "act_123456" };
  const options = buildQueryOptions(new AbortController(), {
    accountId: ACCOUNT_A,
    getToolPolicyContext: () => policy,
  });
  const read = await options.canUseTool(
    "mcp__meta-ads__ads_get_ad_entities",
    { ad_account_id: "123456" },
  );
  const crossAccountRead = await options.canUseTool(
    "mcp__meta-ads__ads_get_ad_entities",
    { ad_account_id: "999999" },
  );
  const unscopedList = await options.canUseTool(
    "mcp__meta-ads__ads_get_ad_accounts",
    {},
  );
  const write = await options.canUseTool("mcp__meta-ads__ads_create_campaign");
  const futureOmniWrite = await options.canUseTool(
    "mcp__omni__instagram_publish_post",
  );

  assert.equal(read.behavior, "allow");
  assert.equal(crossAccountRead.behavior, "deny");
  assert.equal(unscopedList.behavior, "deny");
  assert.equal(write.behavior, "deny");
  assert.equal(futureOmniWrite.behavior, "deny");

  policy = { mode: "auth" };
  const authList = await options.canUseTool(
    "mcp__meta-ads__ads_get_ad_accounts",
    {},
  );
  assert.equal(authList.behavior, "allow");
});

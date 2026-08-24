import assert from "node:assert/strict";
import test from "node:test";

import {
  collectAssistantMessage,
  collectToolResults,
  guardReply,
  NO_REAL_DATA_REPLY,
} from "../src/agent.mjs";

const ACCOUNT = "123456789";

function state(policy = { mode: "read", adAccountId: ACCOUNT }) {
  return {
    actions: [],
    actionIndexByToolUseId: new Map(),
    lastAssistantText: "",
    toolPolicy: policy,
  };
}

function assistantTool(id, name, input = {}) {
  return {
    message: {
      content: [{ type: "tool_use", id, name, input }],
    },
  };
}

function toolResult(id, isError) {
  return {
    message: {
      content: [{ type: "tool_result", tool_use_id: id, is_error: isError }],
    },
  };
}

test("write negado nao vira action real nem libera resposta inventada", () => {
  const current = state();
  collectAssistantMessage(
    assistantTool("write-1", "mcp__meta-ads__ads_create_campaign", {
      ad_account_id: ACCOUNT,
    }),
    current,
  );

  assert.deepEqual(current.actions, []);
  assert.deepEqual(guardReply("A campanha 123456789 foi criada.", current.actions), {
    reply: NO_REAL_DATA_REPLY,
    suppressed: true,
  });
});

test("leitura com erro permanece nao confiavel", () => {
  const current = state();
  collectAssistantMessage(
    assistantTool("read-1", "mcp__meta-ads__ads_get_ad_entities", {
      ad_account_id: ACCOUNT,
    }),
    current,
  );
  assert.equal(current.actions[0]?.status, "pending");

  collectToolResults(toolResult("read-1", true), current);
  assert.equal(current.actions[0]?.status, "error");
  assert.equal(
    guardReply("A conta tem 4 campanhas.", current.actions).suppressed,
    true,
  );
});

test("somente leitura concluida com sucesso libera dados concretos", () => {
  const current = state();
  collectAssistantMessage(
    assistantTool("read-1", "mcp__meta-ads__ads_get_ad_entities", {
      ad_account_id: ACCOUNT,
    }),
    current,
  );
  collectToolResults(toolResult("read-1", false), current);

  assert.equal(current.actions[0]?.status, "ok");
  assert.deepEqual(guardReply("A conta tem 4 campanhas.", current.actions), {
    reply: "A conta tem 4 campanhas.",
    suppressed: false,
  });
});

test("leitura de outra ad account e ignorada antes do resultado", () => {
  const current = state();
  collectAssistantMessage(
    assistantTool("read-foreign", "mcp__meta-ads__ads_get_ad_entities", {
      ad_account_id: "999999999",
    }),
    current,
  );

  assert.deepEqual(current.actions, []);
});

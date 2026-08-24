import assert from "node:assert/strict";
import test from "node:test";

const ACCOUNT_ID = "11111111-1111-4111-8111-111111111111";
const SERVICE_TOKEN = "runner-test-token";

process.env.META_ADS_ASSISTANT_TOKEN = SERVICE_TOKEN;
const { createRunnerServer } = await import("../src/server.mjs");

async function withServer(operation) {
  const server = createRunnerServer();
  await new Promise((resolve, reject) => {
    server.once("error", reject);
    server.listen(0, "127.0.0.1", resolve);
  });
  const address = server.address();
  try {
    return await operation(`http://127.0.0.1:${address.port}`);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
}

function authHeaders(json = false) {
  const headers = { Authorization: `Bearer ${SERVICE_TOKEN}` };
  if (json) {
    headers["Content-Type"] = "application/json";
  }
  return headers;
}

test("health exige Bearer e accountId UUID na query", async () => {
  await withServer(async (baseURL) => {
    const unauthenticated = await fetch(
      `${baseURL}/healthz?accountId=${ACCOUNT_ID}`,
    );
    assert.equal(unauthenticated.status, 401);

    const missingAccount = await fetch(`${baseURL}/healthz`, {
      headers: authHeaders(),
    });
    assert.equal(missingAccount.status, 400);
    assert.equal((await missingAccount.json()).error, "invalid_account_id");

    const healthy = await fetch(`${baseURL}/healthz?accountId=${ACCOUNT_ID}`, {
      headers: authHeaders(),
    });
    assert.equal(healthy.status, 200);
    const payload = await healthy.json();
    assert.equal(payload.ok, true);
    assert.match(payload.metaAuth, /^(oauth|session|none)$/);
  });
});

test("run e auth rejeitam accountId ausente ou malformado antes de executar integracoes", async () => {
  await withServer(async (baseURL) => {
    const cases = [
      ["/run", { prompt: "liste campanhas" }],
      ["/auth/start", { accountId: "../outra-conta" }],
      ["/auth/complete", { accountId: "invalid", callbackUrl: "" }],
    ];
    for (const [path, body] of cases) {
      const response = await fetch(`${baseURL}${path}`, {
        method: "POST",
        headers: authHeaders(true),
        body: JSON.stringify(body),
      });
      assert.equal(response.status, 400);
      assert.equal((await response.json()).error, "invalid_account_id");
    }
  });
});

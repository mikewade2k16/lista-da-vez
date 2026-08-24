import assert from "node:assert/strict";
import test from "node:test";

import {
  AccountSessionPool,
  SessionCapacityError,
} from "../src/session-pool.mjs";

const ACCOUNT_A = "11111111-1111-4111-8111-111111111111";
const ACCOUNT_B = "22222222-2222-4222-8222-222222222222";
const ACCOUNT_C = "33333333-3333-4333-8333-333333333333";

function fakeFactory(closed) {
  return (accountId) => ({
    accountId,
    close() {
      closed.push(accountId);
    },
  });
}

test("pool entrega uma sessao diferente para cada account e faz eviccao LRU", async () => {
  let now = 100;
  const closed = [];
  const pool = new AccountSessionPool({
    maxSessions: 2,
    idleMs: 1000,
    createSession: fakeFactory(closed),
    now: () => now,
  });

  const seenA = await pool.withSession(
    ACCOUNT_A,
    async (session) => session.accountId,
  );
  now += 1;
  const seenB = await pool.withSession(
    ACCOUNT_B,
    async (session) => session.accountId,
  );
  now += 1;
  const seenC = await pool.withSession(
    ACCOUNT_C,
    async (session) => session.accountId,
  );

  assert.equal(seenA, ACCOUNT_A);
  assert.equal(seenB, ACCOUNT_B);
  assert.equal(seenC, ACCOUNT_C);
  assert.deepEqual(closed, [ACCOUNT_A]);
  assert.equal(pool.get(ACCOUNT_A), null);
  assert.equal(pool.get(ACCOUNT_B).accountId, ACCOUNT_B);
  assert.equal(pool.get(ACCOUNT_C).accountId, ACCOUNT_C);
});

test("pool nunca remove uma sessao com lease ativa", async () => {
  const closed = [];
  const pool = new AccountSessionPool({
    maxSessions: 1,
    idleMs: 1000,
    createSession: fakeFactory(closed),
  });
  let release;
  const blocker = new Promise((resolve) => {
    release = resolve;
  });
  const running = pool.withSession(ACCOUNT_A, async () => blocker);
  await Promise.resolve();

  await assert.rejects(
    () => pool.withSession(ACCOUNT_B, async () => null),
    SessionCapacityError,
  );
  assert.deepEqual(closed, []);
  release();
  await running;
});

test("pool fecha sessao que ultrapassou o TTL ocioso", async () => {
  let now = 10;
  const closed = [];
  const pool = new AccountSessionPool({
    maxSessions: 2,
    idleMs: 50,
    createSession: fakeFactory(closed),
    now: () => now,
  });

  await pool.withSession(ACCOUNT_A, async () => null);
  now = 61;
  await pool.withSession(ACCOUNT_B, async () => null);

  assert.deepEqual(closed, [ACCOUNT_A]);
  assert.equal(pool.get(ACCOUNT_A), null);
});

import { requireAccountId } from "./account-id.mjs";

export class SessionCapacityError extends Error {
  constructor(maxSessions) {
    super(`Todas as ${maxSessions} sessoes do runner estao ocupadas.`);
    this.name = "SessionCapacityError";
    this.code = "session_capacity";
  }
}

// Pool LRU tenant-scoped. Cada lease representa um turno (inclusive os que
// aguardam a fila serial da propria account), portanto uma sessao ocupada nunca
// e evictada. Entradas ociosas expiram e o limite total impede crescimento sem
// controle quando muitas accounts acessam o runner.
export class AccountSessionPool {
  constructor({ maxSessions, idleMs, createSession, now = () => Date.now() }) {
    this.maxSessions = maxSessions;
    this.idleMs = idleMs;
    this.createSession = createSession;
    this.now = now;
    this.entries = new Map();
  }

  closeEntry(accountId, entry) {
    try {
      entry.session.close?.();
    } catch {
      // Fechamento best-effort; a entrada sempre sai do pool.
    }
    this.entries.delete(accountId);
  }

  pruneExpired() {
    const cutoff = this.now() - this.idleMs;
    for (const [accountId, entry] of this.entries) {
      if (entry.leases === 0 && entry.lastUsed <= cutoff) {
        this.closeEntry(accountId, entry);
      }
    }
  }

  evictLeastRecentlyUsed() {
    let candidate = null;
    for (const pair of this.entries) {
      const entry = pair[1];
      if (entry.leases !== 0) {
        continue;
      }
      if (!candidate || entry.lastUsed < candidate[1].lastUsed) {
        candidate = pair;
      }
    }
    if (!candidate) {
      return false;
    }
    this.closeEntry(candidate[0], candidate[1]);
    return true;
  }

  acquire(rawAccountId) {
    const accountId = requireAccountId(rawAccountId);
    this.pruneExpired();
    let entry = this.entries.get(accountId);
    if (!entry) {
      if (
        this.entries.size >= this.maxSessions &&
        !this.evictLeastRecentlyUsed()
      ) {
        throw new SessionCapacityError(this.maxSessions);
      }
      entry = {
        session: this.createSession(accountId),
        lastUsed: this.now(),
        leases: 0,
      };
      this.entries.set(accountId, entry);
    }
    entry.leases += 1;
    entry.lastUsed = this.now();
    return { accountId, entry };
  }

  async withSession(accountId, operation) {
    const lease = this.acquire(accountId);
    try {
      return await operation(lease.entry.session);
    } finally {
      lease.entry.leases -= 1;
      lease.entry.lastUsed = this.now();
    }
  }

  get(rawAccountId) {
    const accountId = requireAccountId(rawAccountId);
    this.pruneExpired();
    return this.entries.get(accountId)?.session || null;
  }

  hasMetaTools(accountId) {
    return this.get(accountId)?.hasMetaTools?.() === true;
  }

  recreate(accountId) {
    const session = this.get(accountId);
    session?.recreate?.();
  }

  close(accountId) {
    const normalized = requireAccountId(accountId);
    const entry = this.entries.get(normalized);
    if (!entry || entry.leases !== 0) {
      return false;
    }
    this.closeEntry(normalized, entry);
    return true;
  }

  closeAll() {
    for (const [accountId, entry] of this.entries) {
      this.closeEntry(accountId, entry);
    }
  }

  get size() {
    return this.entries.size;
  }
}

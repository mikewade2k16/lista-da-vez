import { requireAccountId } from "./account-id.mjs";

// Registry tenant-scoped dos fluxos PKCE pendentes. O state continua sendo
// aleatorio e nunca e exposto em logs; a busca por state existe apenas para o
// listener local encaminhar o callback ao fluxo correto.
export class PendingOAuthRegistry {
  constructor() {
    this.flows = new Map();
  }

  get(accountId) {
    return this.flows.get(requireAccountId(accountId)) || null;
  }

  set(accountId, flow) {
    const normalized = requireAccountId(accountId);
    this.flows.set(normalized, flow);
    return flow;
  }

  delete(accountId, expectedFlow) {
    const normalized = requireAccountId(accountId);
    const current = this.flows.get(normalized);
    if (!current || (expectedFlow !== undefined && current !== expectedFlow)) {
      return false;
    }
    return this.flows.delete(normalized);
  }

  findByState(state) {
    if (typeof state !== "string" || state === "") {
      return null;
    }
    for (const [accountId, flow] of this.flows) {
      if (flow?.state === state) {
        return { accountId, flow };
      }
    }
    return null;
  }

  get size() {
    return this.flows.size;
  }
}

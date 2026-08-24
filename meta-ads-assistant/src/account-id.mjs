// Identidade de tenant aceita pelo runner. O Go envia core.accounts.id e o
// runner usa esse valor apenas depois de validar/normalizar na borda HTTP.

const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export class InvalidAccountIdError extends Error {
  constructor() {
    super("accountId deve ser um UUID valido.");
    this.name = "InvalidAccountIdError";
    this.code = "invalid_account_id";
  }
}

export function normalizeAccountId(value) {
  if (typeof value !== "string") {
    return "";
  }
  const normalized = value.trim().toLowerCase();
  return UUID_PATTERN.test(normalized) ? normalized : "";
}

export function requireAccountId(value) {
  const accountId = normalizeAccountId(value);
  if (accountId === "") {
    throw new InvalidAccountIdError();
  }
  return accountId;
}

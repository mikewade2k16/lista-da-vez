import type { UserRole } from "~/types";

export const ADMIN_ROLE_ITEMS = [
  { label: "AGENT", value: "AGENT" as const },
  { label: "SUPERVISOR", value: "SUPERVISOR" as const },
  { label: "VIEWER", value: "VIEWER" as const },
  { label: "ADMIN", value: "ADMIN" as const }
];

export function createTenantFormState() {
  return {
    name: "",
    whatsappInstance: "",
    evolutionApiKey: "",
    maxChannels: 1,
    maxUsers: 3,
    retentionDays: 15,
    maxUploadMb: 500
  };
}

export function createClientFormState() {
  return {
    slug: "",
    name: "",
    evolutionApiKey: "",
    maxChannels: 1,
    maxUsers: 3,
    retentionDays: 15,
    maxUploadMb: 500,
    adminName: "",
    adminEmail: "",
    adminPassword: ""
  };
}

export function createUserFormState() {
  return {
    name: "",
    email: "",
    password: "",
    role: "AGENT" as UserRole
  };
}

export function createWhatsAppFormState() {
  return {
    instanceId: "",
    instanceName: "",
    number: ""
  };
}

export function extractAdminError(error: unknown) {
  if (error && typeof error === "object" && "data" in error) {
    const data = (error as { data?: Record<string, unknown> }).data;
    if (data && typeof data.message === "string") {
      return data.message;
    }
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "Operacao falhou";
}

export type AdminFieldErrorState = Record<string, string>;

export function clearAdminFieldErrors(target: AdminFieldErrorState) {
  for (const key of Object.keys(target)) {
    delete target[key];
  }
}

export function extractAdminFieldErrors(error: unknown): AdminFieldErrorState {
  if (!error || typeof error !== "object" || !("data" in error)) {
    return {};
  }

  const data = (error as { data?: Record<string, unknown> }).data;
  if (!data || typeof data !== "object" || !("errors" in data)) {
    return {};
  }

  const errors = data.errors as Record<string, unknown> | undefined;
  const fieldErrors =
    errors && typeof errors === "object" && "fieldErrors" in errors
      ? (errors.fieldErrors as Record<string, unknown> | undefined)
      : undefined;

  if (!fieldErrors || typeof fieldErrors !== "object") {
    return {};
  }

  const result: AdminFieldErrorState = {};
  for (const [field, value] of Object.entries(fieldErrors)) {
    if (Array.isArray(value)) {
      const firstMessage = value.find((entry) => typeof entry === "string" && entry.trim());
      if (typeof firstMessage === "string") {
        result[field] = firstMessage;
      }
      continue;
    }

    if (typeof value === "string" && value.trim()) {
      result[field] = value;
    }
  }

  return result;
}

export function applyAdminFieldErrors(target: AdminFieldErrorState, error: unknown) {
  clearAdminFieldErrors(target);
  const next = extractAdminFieldErrors(error);
  Object.assign(target, next);
  return Object.keys(next).length > 0;
}

export function extractPairingCode(source: unknown): string | null {
  if (!source || typeof source !== "object") {
    return null;
  }

  const queue: unknown[] = [source];
  while (queue.length > 0) {
    const current = queue.shift();
    if (!current) {
      continue;
    }

    if (typeof current === "string") {
      const trimmed = current.trim();
      if (trimmed && trimmed.length <= 64 && /^[A-Za-z0-9\-_.@:+/]+$/.test(trimmed)) {
        return trimmed;
      }
      continue;
    }

    if (typeof current !== "object") {
      continue;
    }

    if (Array.isArray(current)) {
      queue.push(...current);
      continue;
    }

    const obj = current as Record<string, unknown>;
    if (typeof obj.pairingCode === "string") {
      return obj.pairingCode;
    }
    if (typeof obj.code === "string" && obj.code.length <= 64) {
      return obj.code;
    }

    queue.push(...Object.values(obj));
  }

  return null;
}

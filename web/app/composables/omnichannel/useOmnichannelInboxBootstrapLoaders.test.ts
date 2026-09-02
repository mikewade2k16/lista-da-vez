import { ref } from "vue";
import { describe, expect, it, vi } from "vitest";
import type { Contact, TenantUser, WhatsAppInstanceRecord, WhatsAppStatusResponse } from "~/types";
import { useOmnichannelInboxBootstrapLoaders } from "./useOmnichannelInboxBootstrapLoaders";

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => { resolve = resolvePromise; });
  return { promise, resolve };
}

describe("bootstrap loaders por conta", () => {
  it("descarta respostas atrasadas de A e nao faz B aguardar o status de A", async () => {
    let scopeIdentity = "account-a:0";
    const requests = new Map<string, ReturnType<typeof deferred<unknown>>>();
    const apiFetch = vi.fn((path: string) => {
      const key = `${scopeIdentity}:${path}`;
      const request = deferred<unknown>();
      requests.set(key, request);
      return request.promise;
    });
    const tenantMaxUploadMb = ref(0);
    const whatsappStatus = ref<WhatsAppStatusResponse | null>(null);
    const users = ref<TenantUser[]>([]);
    const contacts = ref<Contact[]>([]);
    const whatsappInstances = ref<WhatsAppInstanceRecord[]>([]);
    const whatsappInstanceAccessState = ref<"loading" | "resolved-empty" | "resolved-nonempty" | "error">("loading");
    const sortContacts = vi.fn();
    const loaders = useOmnichannelInboxBootstrapLoaders({
      tenantMaxUploadMb,
      whatsappStatus,
      users,
      contacts,
      whatsappInstances,
      whatsappInstanceAccessState,
      selectedInstanceId: ref("all"),
      loadingWhatsAppStatus: ref(false),
      loadingUsers: ref(false),
      loadingContacts: ref(false),
      apiFetch: apiFetch as never,
      sortContacts,
      getScopeIdentity: () => scopeIdentity
    });

    const aLoads = [
      loaders.loadTenantUploadLimit(), loaders.loadWhatsAppStatus(), loaders.loadUsers(),
      loaders.loadContacts(), loaders.loadAccessibleWhatsAppInstances()
    ];
    scopeIdentity = "account-b:1";
    const bLoads = [
      loaders.loadTenantUploadLimit(), loaders.loadWhatsAppStatus(), loaders.loadUsers(),
      loaders.loadContacts(), loaders.loadAccessibleWhatsAppInstances()
    ];
    const bUser = { id: "user-b", name: "B" } as TenantUser;
    const bContact = { id: "contact-b", name: "Contato B" } as Contact;
    const bInstance = { id: "instance-b", instanceName: "b" } as WhatsAppInstanceRecord;
    requests.get("account-b:1:/tenant")?.resolve({ maxUploadMb: 25 });
    requests.get("account-b:1:/tenant/whatsapp/status")?.resolve({ configured: true, message: "B" });
    requests.get("account-b:1:/users")?.resolve([bUser]);
    requests.get("account-b:1:/contacts")?.resolve([bContact]);
    requests.get("account-b:1:/tenant/whatsapp/instances/access")?.resolve({ instances: [bInstance] });
    await Promise.all(bLoads);

    expect(whatsappStatus.value?.message).toBe("B");
    expect(users.value).toEqual([bUser]);
    expect(contacts.value).toEqual([bContact]);
    expect(whatsappInstances.value).toEqual([bInstance]);
    expect(whatsappInstanceAccessState.value).toBe("resolved-nonempty");
    expect(tenantMaxUploadMb.value).toBe(25);

    requests.get("account-a:0:/tenant")?.resolve({ maxUploadMb: 99 });
    requests.get("account-a:0:/tenant/whatsapp/status")?.resolve({ configured: true, message: "A" });
    requests.get("account-a:0:/users")?.resolve([{ id: "user-a" }]);
    requests.get("account-a:0:/contacts")?.resolve([{ id: "contact-a" }]);
    requests.get("account-a:0:/tenant/whatsapp/instances/access")?.resolve({ instances: [{ id: "instance-a" }] });
    await Promise.all(aLoads);

    expect(whatsappStatus.value?.message).toBe("B");
    expect(users.value).toEqual([bUser]);
    expect(contacts.value).toEqual([bContact]);
    expect(whatsappInstances.value).toEqual([bInstance]);
    expect(tenantMaxUploadMb.value).toBe(25);
    expect(sortContacts).toHaveBeenCalledTimes(1);
  });

  it("resolve lista vazia e limpa estado anterior quando o contrato de acesso falha", async () => {
    const whatsappInstances = ref<WhatsAppInstanceRecord[]>([
      { id: "stale", instanceName: "stale" } as WhatsAppInstanceRecord
    ]);
    const whatsappInstanceAccessState = ref<"loading" | "resolved-empty" | "resolved-nonempty" | "error">("resolved-nonempty");
    const apiFetch = vi.fn();
    const loaders = useOmnichannelInboxBootstrapLoaders({
      tenantMaxUploadMb: ref(0),
      whatsappStatus: ref<WhatsAppStatusResponse | null>(null),
      users: ref<TenantUser[]>([]),
      contacts: ref<Contact[]>([]),
      whatsappInstances,
      whatsappInstanceAccessState,
      selectedInstanceId: ref("all"),
      loadingWhatsAppStatus: ref(false),
      loadingUsers: ref(false),
      loadingContacts: ref(false),
      apiFetch: apiFetch as never,
      sortContacts: vi.fn(),
      getScopeIdentity: () => "account-a:0"
    });

    apiFetch.mockResolvedValueOnce({ instances: [] });
    await loaders.loadAccessibleWhatsAppInstances();
    expect(whatsappInstances.value).toEqual([]);
    expect(whatsappInstanceAccessState.value).toBe("resolved-empty");

    for (const failure of [
      Object.assign(new Error("forbidden"), { statusCode: 403 }),
      Object.assign(new Error("server"), { statusCode: 500 }),
      new TypeError("network")
    ]) {
      whatsappInstances.value = [{ id: "stale", instanceName: "stale" } as WhatsAppInstanceRecord];
      apiFetch.mockRejectedValueOnce(failure);
      await expect(loaders.loadAccessibleWhatsAppInstances()).rejects.toBe(failure);
      expect(whatsappInstances.value).toEqual([]);
      expect(whatsappInstanceAccessState.value).toBe("error");
    }

    apiFetch.mockClear();
    await loaders.loadWhatsAppStatus({ force: true });
    expect(apiFetch).not.toHaveBeenCalled();
  });
});

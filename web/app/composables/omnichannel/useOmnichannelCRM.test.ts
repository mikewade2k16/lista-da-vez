import { beforeEach, describe, expect, it, vi } from "vitest";
import { useOmnichannelCRM, type CRMContact } from "~/composables/omnichannel/useOmnichannelCRM";

const { apiFetchMock } = vi.hoisted(() => ({
  apiFetchMock: vi.fn()
}));

vi.mock("~/composables/useApi", () => ({
  useApi: () => ({ apiFetch: apiFetchMock })
}));

function contact(id: string): CRMContact {
  return {
    id,
    name: id,
    phone: `551199999${id.length}`,
    source: "landing_page",
    relationshipStatus: "known_lead",
    tags: ["vip"]
  };
}

describe("useOmnichannelCRM E4-FE-07", () => {
  beforeEach(() => {
    apiFetchMock.mockReset();
  });

  it("aplica filtros no primeiro cursor e concatena a próxima página sem duplicar", async () => {
    apiFetchMock
      .mockResolvedValueOnce({ contacts: [contact("first"), contact("second")], hasMore: true, nextCursor: "cursor-2" })
      .mockResolvedValueOnce({ contacts: [contact("second"), contact("third")], hasMore: false, nextCursor: "" });

    const crm = useOmnichannelCRM();
    await crm.loadContacts({
      search: "Ana",
      status: "known_lead",
      channel: "WHATSAPP",
      tag: "vip",
      owner: "owner-1",
      source: "landing_page"
    });

    expect(apiFetchMock).toHaveBeenNthCalledWith(
      1,
      "/contacts/crm?limit=100&q=Ana&status=known_lead&channel=WHATSAPP&tag=vip&owner=owner-1&source=landing_page"
    );
    expect(crm.contacts.value.map((entry) => entry.id)).toEqual(["first", "second"]);
    expect(crm.hasMore.value).toBe(true);
    expect(crm.nextCursor.value).toBe("cursor-2");

    await crm.loadContacts({
      search: "Ana",
      status: "known_lead",
      channel: "WHATSAPP",
      before: crm.nextCursor.value,
      append: true
    });

    expect(apiFetchMock).toHaveBeenNthCalledWith(
      2,
      "/contacts/crm?limit=100&q=Ana&status=known_lead&channel=WHATSAPP&before=cursor-2"
    );
    expect(crm.contacts.value.map((entry) => entry.id)).toEqual(["first", "second", "third"]);
    expect(crm.hasMore.value).toBe(false);
    expect(crm.nextCursor.value).toBe("");
  });

  it("mantém o erro acionável e não trata resposta inválida como lista vazia silenciosa", async () => {
    apiFetchMock.mockRejectedValueOnce(new Error("CRM indisponível"));
    const crm = useOmnichannelCRM();

    await crm.loadContacts();

    expect(crm.contacts.value).toEqual([]);
    expect(crm.ready.value).toBe(false);
    expect(crm.error.value).toBe("CRM indisponível");
    expect(crm.loading.value).toBe(false);
    expect(crm.loadingMore.value).toBe(false);
  });

  it("desfaz merge pelo endpoint tenant-scoped do evento", async () => {
    apiFetchMock.mockResolvedValueOnce({
      eventId: "merge-1",
      sourceContactId: "source-1",
      targetContactId: "target-1",
      undoneAt: new Date().toISOString(),
      createdAt: new Date().toISOString()
    });
    const crm = useOmnichannelCRM();

    await crm.undoMerge("merge-1");

    expect(apiFetchMock).toHaveBeenCalledWith("/contacts/merges/merge-1/undo", { method: "POST" });
  });
});

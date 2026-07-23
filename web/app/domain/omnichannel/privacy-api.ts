import type { ApiRequest } from '~/domain/omnichannel/config-api'

export const CONVERSATION_PRIVACY_PERMISSION = 'omnichannel.conversations.privacy.manage'

export interface HiddenOmnichannelContact {
  contactId: string
  conversationId: string
  contactName: string
  contactPhone: string
  hiddenAt: string
  historyClearedAt: string | null
  hiddenByUserId: string
  historyClearedByUserId?: string | null
}

export type ContactAIRestrictionMode = 'allow' | 'until' | 'indefinite'

export interface ContactAIRestriction {
  contactId: string
  blocked: boolean
  mode: ContactAIRestrictionMode
  blockedUntil: string | null
  updatedAt: string | null
}

export interface ContactAIRestrictionInput {
  mode: ContactAIRestrictionMode
  blockedUntil?: string
}

export function fetchHiddenOmnichannelContacts(
  api: ApiRequest,
): Promise<HiddenOmnichannelContact[]> {
  return api('/v1/omnichannel/privacy/hidden-contacts', { dedupe: false }) as Promise<
    HiddenOmnichannelContact[]
  >
}

export function hideOmnichannelContact(
  api: ApiRequest,
  conversationId: string,
  clearHistory: boolean,
): Promise<HiddenOmnichannelContact> {
  return api(`/v1/omnichannel/conversations/${encodeURIComponent(conversationId)}/privacy/hide`, {
    method: 'POST',
    body: { clearHistory },
  }) as Promise<HiddenOmnichannelContact>
}

export function restoreOmnichannelContact(api: ApiRequest, contactId: string): Promise<void> {
  return api(`/v1/omnichannel/privacy/hidden-contacts/${encodeURIComponent(contactId)}/restore`, {
    method: 'POST',
  }) as Promise<void>
}

export function fetchContactAIRestriction(
  api: ApiRequest,
  conversationId: string,
): Promise<ContactAIRestriction> {
  return api(`/v1/omnichannel/conversations/${encodeURIComponent(conversationId)}/ai-restriction`, {
    dedupe: false,
  }) as Promise<ContactAIRestriction>
}

export function updateContactAIRestriction(
  api: ApiRequest,
  conversationId: string,
  input: ContactAIRestrictionInput,
): Promise<ContactAIRestriction> {
  return api(`/v1/omnichannel/conversations/${encodeURIComponent(conversationId)}/ai-restriction`, {
    method: 'PUT',
    body: input,
  }) as Promise<ContactAIRestriction>
}

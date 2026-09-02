import { describe, expect, it } from 'vitest'
import { isSelectableAccountId, type AccountSummary } from './account'

function account(id: string): AccountSummary {
  return {
    id,
    name: id,
    slug: id,
    organizationId: '',
    planCode: 'test',
    modules: ['omnichannel'],
    isAgency: false,
    organizationName: '',
  }
}

describe('CoreAccountSwitcher authorization source', () => {
  it('accepts only ids returned by /v2/me/accounts', () => {
    const accounts = [account('account-a'), account('account-b')]

    expect(isSelectableAccountId(accounts, ' account-b ')).toBe(true)
    expect(isSelectableAccountId(accounts, 'account-c')).toBe(false)
    expect(isSelectableAccountId(accounts, '')).toBe(false)
  })
})

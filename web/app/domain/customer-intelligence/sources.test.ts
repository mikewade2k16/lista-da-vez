import { describe, expect, it, vi } from 'vitest'
import {
  buildIntelligenceSourceWriteInput,
  createIntelligenceSourceDraft,
  fetchIntelligenceSources,
  normalizeIntelligenceSourceDescriptor,
  setIntelligenceSourceEnabled,
  validateIntelligenceSourceDraft,
  type IntelligenceSourceConfig,
} from './sources'

const ERP_DESCRIPTOR = normalizeIntelligenceSourceDescriptor({
  key: 'erp',
  label: 'ERP configurado',
  ownerPackage: 'crm/erp',
  capabilities: ['subject_evidence'],
  modes: ['scheduled', 'on_demand'],
  allowedConfigKeys: ['connectionId', 'entityTypes', 'lookbackDays'],
  allowedFields: ['preferred_name', 'order_date', 'total_amount_cents'],
  purposeKeys: ['customer_profile', 'customer_service', 'marketing'],
  configSchema: [
    {
      key: 'connectionId',
      label: 'Conexao ERP registrada',
      type: 'safe_key',
      required: true,
    },
    {
      key: 'entityTypes',
      label: 'Entidades',
      type: 'string_list',
      elementKeys: ['customer', 'order'],
    },
    {
      key: 'lookbackDays',
      label: 'Janela em dias',
      type: 'integer',
      min: 1,
      max: 3650,
    },
  ],
})

describe('customer intelligence source descriptor contracts', () => {
  it('normalizes purposes, fields, capabilities and typed config from the backend', () => {
    expect(ERP_DESCRIPTOR).toMatchObject({
      ownerPackage: 'crm/erp',
      category: 'dados do relacionamento',
      capabilities: ['subject_evidence'],
      modes: ['scheduled', 'on_demand'],
      purposeKeys: ['customer_profile', 'customer_service', 'marketing'],
      allowedFields: ['preferred_name', 'order_date', 'total_amount_cents'],
    })
    expect(ERP_DESCRIPTOR.configSchema[1]).toMatchObject({
      type: 'string_list',
      elementKeys: ['customer', 'order'],
    })
  })

  it('creates a privacy-first source with optimistic revision zero', () => {
    const draft = createIntelligenceSourceDraft(ERP_DESCRIPTOR, null)

    expect(draft).toMatchObject({
      connectionKey: 'default',
      status: 'disabled',
      mode: 'on_demand',
      purposeKey: 'customer_profile',
      fieldAllowlist: [],
      freshnessSeconds: 3600,
      retentionPolicyKey: 'customer_profile.default',
      snapshotTtlSeconds: 7_776_000,
      onExpiry: 'tombstone',
      config: {},
    })

    draft.config = {
      connectionId: 'erp.primary',
      entityTypes: ['customer'],
      lookbackDays: 30,
    }
    draft.fieldAllowlist = ['preferred_name']
    const input = buildIntelligenceSourceWriteInput(ERP_DESCRIPTOR, null, 'client-1', draft)

    expect(input).toMatchObject({
      clientAccountId: 'client-1',
      sourceKey: 'erp',
      status: 'disabled',
      expectedRevision: 0,
      fieldAllowlist: ['preferred_name'],
      config: {
        connectionId: 'erp.primary',
        entityTypes: ['customer'],
        lookbackDays: 30,
      },
    })
  })

  it('rejects arbitrary connection surfaces and values outside the descriptor', () => {
    const draft = createIntelligenceSourceDraft(ERP_DESCRIPTOR, null)
    draft.connectionKey = 'https://database.local'
    draft.fieldAllowlist = ['email']
    draft.config = {
      connectionId: 'erp.primary',
      entityTypes: ['users'],
      baseUrl: 'https://database.local',
    }

    const validation = validateIntelligenceSourceDraft(ERP_DESCRIPTOR, draft)

    expect(validation.valid).toBe(false)
    expect(validation.errors).toMatchObject({
      connectionKey: expect.any(String),
      fieldAllowlist: expect.any(String),
      config: expect.any(String),
      'config.entityTypes': expect.any(String),
    })
  })
})

describe('customer intelligence source retention contracts', () => {
  it('validates and sends an editable safe retention policy key', () => {
    const draft = createIntelligenceSourceDraft(ERP_DESCRIPTOR, null)
    draft.retentionPolicyKey = 'INVALID POLICY'
    expect(validateIntelligenceSourceDraft(ERP_DESCRIPTOR, draft).errors.retentionPolicyKey).toBe(
      'A chave da policy de retencao e invalida.',
    )

    draft.retentionPolicyKey = 'customer_profile.marketing'
    draft.config.connectionId = 'erp.primary'
    const input = buildIntelligenceSourceWriteInput(ERP_DESCRIPTOR, null, 'client-1', draft)
    expect(input.retentionPolicyKey).toBe('customer_profile.marketing')
  })

  it('normalizes a conservative closed retention policy for legacy responses', async () => {
    const api = vi.fn().mockResolvedValue([
      {
        id: 'source-1',
        clientAccountId: 'client-1',
        sourceKey: 'erp',
        connectionKey: 'default',
        status: 'enabled',
        mode: 'scheduled',
        purposeKey: 'customer_profile',
        fieldAllowlist: ['total_amount_cents'],
        freshnessSeconds: 3600,
        config: {},
        revision: 1,
      },
    ])

    const sources = await fetchIntelligenceSources(api as never, 'client-1')

    expect(sources[0]).toMatchObject({
      retentionPolicyKey: 'customer_profile.default',
      snapshotTtlSeconds: 7_776_000,
      onExpiry: 'tombstone',
    })
  })

  it('preserves the versioned retention binding when toggling a source', async () => {
    const api = vi.fn().mockResolvedValue({})
    const source: IntelligenceSourceConfig = {
      id: 'source-1',
      clientAccountId: 'client-1',
      sourceKey: 'erp',
      connectionKey: 'default',
      status: 'enabled',
      mode: 'scheduled',
      purposeKey: 'customer_profile',
      fieldAllowlist: ['total_amount_cents'],
      freshnessSeconds: 3600,
      retentionPolicyKey: 'customer_profile.short',
      retentionPolicyVersionId: 'policy-version-2',
      retentionPolicyVersion: 2,
      snapshotTtlSeconds: 604_800,
      onExpiry: 'crypto_shred',
      config: {},
      revision: 4,
      enabled: true,
    }

    await setIntelligenceSourceEnabled(api as never, source, false)

    expect(api).toHaveBeenCalledWith('/v1/customer-intelligence/sources', {
      method: 'PUT',
      body: expect.objectContaining({
        status: 'disabled',
        retentionPolicyKey: 'customer_profile.short',
        snapshotTtlSeconds: 604_800,
        onExpiry: 'crypto_shred',
        expectedRevision: 4,
      }),
    })
  })
})

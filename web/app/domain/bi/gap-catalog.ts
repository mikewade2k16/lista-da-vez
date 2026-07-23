export type BiGapPriority = 'P0' | 'P1' | 'P2'
export type BiGapStatus = 'missing' | 'unconfirmed' | 'contract'
export type BiGapDomain = 'product' | 'sales' | 'customer' | 'team' | 'stock' | 'governance'

export interface BiGapItem {
  id: string
  priority: BiGapPriority
  status: BiGapStatus
  domain: BiGapDomain
  title: string
  erpEvidence: string
  apiGap: string
  supplierRequest: string
}

export const BI_GAP_AUDIT_DATE = '23/07/2026'

export const BI_ERP_VOLUME_EVIDENCE = [
  { label: 'Produtos', value: 360_686 },
  { label: 'Clientes', value: 348_802 },
  { label: 'Linhas de pedidos', value: 775_822 },
  { label: 'Linhas canceladas', value: 44_952 },
]

export const BI_GAP_PRIORITY_LABELS: Record<BiGapPriority, string> = {
  P0: 'P0 · bloqueia substituição',
  P1: 'P1 · preserva operação',
  P2: 'P2 · governança',
}

export const BI_GAP_STATUS_LABELS: Record<BiGapStatus, string> = {
  missing: 'Não observado na API',
  unconfirmed: 'Chave não confirmada',
  contract: 'Contrato insuficiente',
}

export const BI_GAP_DOMAIN_LABELS: Record<BiGapDomain, string> = {
  product: 'Produto',
  sales: 'Vendas',
  customer: 'Cliente',
  team: 'Equipe',
  stock: 'Estoque',
  governance: 'Governança',
}

export const BI_ERP_API_GAPS: BiGapItem[] = [
  {
    id: 'item-balance-bridge',
    priority: 'P0',
    status: 'unconfirmed',
    domain: 'product',
    title: 'Ponte itemSaldoId → Item/SKU',
    erpEvidence:
      'As linhas comerciais usam SKU e o cadastro de produtos possui identificador estável.',
    apiGap: 'Nota Item, custo e Inventário usam itemSaldoId; Item usa itemId/referência.',
    supplierRequest: 'Disponibilizar itemSaldoId, itemId, SKU e empresa no mesmo recurso.',
  },
  {
    id: 'commercial-product',
    priority: 'P0',
    status: 'missing',
    domain: 'product',
    title: 'Cadastro comercial completo do produto',
    erpEvidence: 'O ERP possui SKU, nome, descrição e preço comercial para 360.686 produtos.',
    apiGap: 'A amostra de Item não confirmou SKU, nome, descrição nem preço de venda.',
    supplierRequest: 'Confirmar e documentar SKU, nome, descrição e preço comercial vigente.',
  },
  {
    id: 'order-note-key',
    priority: 'P0',
    status: 'unconfirmed',
    domain: 'sales',
    title: 'Chave oficial de Pedido/Venda → Nota',
    erpEvidence: 'Pedidos possuem order_id e identifier.',
    apiGap: 'Não foi confirmada a equivalência desses campos com numDocumento ou id da Nota.',
    supplierRequest: 'Documentar a chave estável que liga pedido/venda à respectiva Nota.',
  },
  {
    id: 'payment-method',
    priority: 'P0',
    status: 'missing',
    domain: 'sales',
    title: 'Forma e composição do pagamento',
    erpEvidence: 'As 775.822 linhas comerciais possuem payment_type.',
    apiGap: 'Forma de pagamento, parcelas e valores por meio não foram observados.',
    supplierRequest: 'Disponibilizar formas, parcelas, valores e identificador da transação.',
  },
  {
    id: 'commercial-cancellation',
    priority: 'P0',
    status: 'missing',
    domain: 'sales',
    title: 'Cancelamento comercial estruturado',
    erpEvidence: 'O ERP mantém 44.952 linhas de pedidos cancelados em dataset próprio.',
    apiGap: 'Não há dataset/status confirmado com cancelado, data, motivo e responsável.',
    supplierRequest: 'Criar endpoint ou filtro incremental confiável para cancelamentos.',
  },
  {
    id: 'note-item-product',
    priority: 'P0',
    status: 'missing',
    domain: 'sales',
    title: 'SKU ou itemId na linha da Nota',
    erpEvidence: 'Cada linha de pedido do ERP identifica o produto por SKU.',
    apiGap: 'Nota Item trouxe apenas itemSaldoId, sem SKU/itemId confirmado.',
    supplierRequest: 'Adicionar SKU e itemId em Nota Item ou fornecer relação oficial equivalente.',
  },
  {
    id: 'current-stock',
    priority: 'P0',
    status: 'missing',
    domain: 'stock',
    title: 'Saldo atual por produto e empresa',
    erpEvidence: 'A substituição precisa preservar disponibilidade comercial por loja.',
    apiGap: 'Inventário expõe movimentos, mas não confirmou saldo físico, reservado e disponível.',
    supplierRequest: 'Fornecer saldo atual, reservado e disponível por itemSaldoId, SKU e empresa.',
  },
  {
    id: 'incremental-filter',
    priority: 'P0',
    status: 'contract',
    domain: 'governance',
    title: 'Filtros incrementais confiáveis',
    erpEvidence: 'Arquivos atuais possuem lote, checksum, horário e trilha de importação.',
    apiGap: 'Não há contrato confirmado de updatedAt/cursor e detecção incremental de exclusões.',
    supplierRequest: 'Documentar updatedAt, cursor, janela de período e exclusões incrementais.',
  },
  {
    id: 'customer-contact',
    priority: 'P1',
    status: 'missing',
    domain: 'customer',
    title: 'Contato e data de cadastro do cliente',
    erpEvidence: 'O ERP possui e-mail, telefone, celular e data de cadastro.',
    apiGap: 'A Nota observada não substitui uma entidade mestre atualizada de Cliente.',
    supplierRequest: 'Disponibilizar cadastro de Cliente com contatos, consentimento e updatedAt.',
  },
  {
    id: 'customer-profile',
    priority: 'P1',
    status: 'missing',
    domain: 'customer',
    title: 'Perfil complementar do cliente',
    erpEvidence: 'Existem apelido, gênero, país, complemento e tags comerciais.',
    apiGap: 'Esses atributos não foram observados nas seis entidades.',
    supplierRequest: 'Adicionar os atributos à entidade mestre de Cliente com tipos documentados.',
  },
  {
    id: 'employee-master',
    priority: 'P1',
    status: 'missing',
    domain: 'team',
    title: 'Entidade mestre de Funcionário',
    erpEvidence: 'O ERP possui 21.476 registros com status e vínculo de loja.',
    apiGap: 'A API expõe colaborador dentro da Nota, mas não um cadastro mestre com status.',
    supplierRequest: 'Fornecer Funcionário com ID, status, perfil, loja e updatedAt.',
  },
  {
    id: 'customer-origin',
    priority: 'P1',
    status: 'missing',
    domain: 'customer',
    title: 'Loja de origem e vendedor do cliente',
    erpEvidence: 'O cadastro atual preserva store_id e employee_id.',
    apiGap: 'A relação comercial persistente do cliente com loja/vendedor não foi confirmada.',
    supplierRequest: 'Disponibilizar loja de origem e vínculo com vendedor na entidade Cliente.',
  },
  {
    id: 'return-semantics',
    priority: 'P1',
    status: 'contract',
    domain: 'sales',
    title: 'Semântica de devolução, exclusão e débito',
    erpEvidence: 'Pedidos trazem product_return, total_exclusion e total_debit.',
    apiGap: 'Há valores fiscais relacionados, mas a equivalência comercial não está documentada.',
    supplierRequest: 'Documentar valores, sinais, motivos e relação com pedido/nota original.',
  },
  {
    id: 'versioned-schema',
    priority: 'P2',
    status: 'contract',
    domain: 'governance',
    title: 'Schema/OpenAPI versionado',
    erpEvidence: 'A migração exige contrato reproduzível e auditável.',
    apiGap: 'O levantamento atual depende de amostra observada, não de schema formal versionado.',
    supplierRequest: 'Publicar OpenAPI com versão e política de breaking changes.',
  },
  {
    id: 'types-timezone',
    priority: 'P2',
    status: 'contract',
    domain: 'governance',
    title: 'Tipos decimais, datas e timezone',
    erpEvidence: 'Valores e datas precisam manter precisão e interpretação consistente.',
    apiGap: 'Valores monetários chegaram como texto e datas não têm contrato formal de timezone.',
    supplierRequest: 'Documentar decimal, ISO 8601, timezone e regras de arredondamento.',
  },
  {
    id: 'soft-delete',
    priority: 'P2',
    status: 'contract',
    domain: 'governance',
    title: 'Exclusão lógica consistente',
    erpEvidence: 'A sincronização substituta precisa detectar remoções sem recarga total.',
    apiGap: 'Excluído/data de exclusão não foram confirmados em todas as entidades.',
    supplierRequest: 'Padronizar excluido, deletedAt e filtro incremental por exclusão.',
  },
  {
    id: 'change-log',
    priority: 'P2',
    status: 'contract',
    domain: 'governance',
    title: 'Changelog ou cursor de sincronização',
    erpEvidence: 'A trilha atual identifica arquivo, lote, linha e horário de importação.',
    apiGap: 'Não há request ID/cursor confirmado para auditoria ponta a ponta.',
    supplierRequest: 'Fornecer cursor monotônico, request ID e política de retenção.',
  },
  {
    id: 'limits-errors',
    priority: 'P2',
    status: 'contract',
    domain: 'governance',
    title: 'Limites e erros documentados',
    erpEvidence: 'A operação precisa dimensionar carga e recuperação de falhas.',
    apiGap: 'Rate limit, limites máximos e catálogo formal de erros não foram confirmados.',
    supplierRequest: 'Documentar rate limit, paginação máxima, timeouts e códigos de erro.',
  },
]

function normalizeGapSearch(value: string) {
  return value
    .normalize('NFD')
    .replace(/[\u0300-\u036f]/g, '')
    .trim()
    .toLowerCase()
}

export function filterBiGaps(
  gaps: BiGapItem[],
  filters: { search?: string; priority?: string; domain?: string },
) {
  const search = normalizeGapSearch(filters.search || '')
  return gaps.filter((gap) => {
    if (filters.priority && gap.priority !== filters.priority) return false
    if (filters.domain && gap.domain !== filters.domain) return false
    if (!search) return true

    return normalizeGapSearch(
      `${gap.title} ${gap.erpEvidence} ${gap.apiGap} ${gap.supplierRequest}`,
    ).includes(search)
  })
}

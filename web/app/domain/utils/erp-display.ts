export type ExportScope = 'page' | 'filtered' | 'all'

export type ErpGridColumn = {
  id: string
  label: string
  width: string
  align: string
  locked?: boolean
  sortable?: boolean
}

export type ErpTab = {
  id: string
  label: string
  icon: string
}

export type ErpSpecificSearch = {
  label: string
  placeholder: string
}

export type ErpBancoCard = {
  table: string
  label: string
  desc: string
  badge: string
}

export type ErpBancoSection = {
  title: string
  text: string
  note: string
  cards: ErpBancoCard[]
}

export type ErpRecord = Record<string, unknown>

export type ErpImportedFile = ErpRecord & {
  importedAt?: string | null
  recordCount?: number | null
  sourceName?: string | null
}

export type ErpRun = ErpRecord & {
  dataType?: string | null
  errorMessage?: string | null
  filesImported?: number | null
  filesSkipped?: number | null
  finishedAt?: string | null
  id?: string | null
  rowsImported?: number | null
  rowsRead?: number | null
  startedAt?: string | null
  status?: string | null
  triggeredBy?: string | null
}

export type ErpOrderStats = ErpRecord & {
  avgAmountCents?: number | null
  dataType?: string | null
  orderCount?: number | null
  pa?: number | null
  totalAmountCents?: number | null
}

export const ERP_TABS: ErpTab[] = [
  { id: 'produtos', label: 'Produtos', icon: 'inventory_2' },
  { id: 'pedidos', label: 'Compras', icon: 'receipt_long' },
  { id: 'clientes', label: 'Clientes', icon: 'groups' },
  { id: 'cancelados', label: 'Cancelados', icon: 'event_busy' },
  { id: 'funcionarios', label: 'Funcionarios', icon: 'badge' },
  { id: 'crm', label: 'CRM', icon: 'person_search' },
  { id: 'banco', label: 'Banco', icon: 'storage' },
  { id: 'sincronizacao', label: 'Sincronizacao', icon: 'sync' },
]

export const ERP_BANCO_TABS: ErpTab[] = [
  { id: 'geral', label: 'Visao geral', icon: 'dashboard' },
  { id: 'produtos', label: 'Produtos', icon: 'inventory_2' },
  { id: 'clientes', label: 'Clientes', icon: 'groups' },
  { id: 'pedidos', label: 'Compras', icon: 'receipt_long' },
  { id: 'cancelados', label: 'Cancelados', icon: 'event_busy' },
  { id: 'funcionarios', label: 'Funcionarios', icon: 'badge' },
  { id: 'outbox', label: 'Outbox', icon: 'send' },
]

export const ERP_PRODUCT_COLUMNS: ErpGridColumn[] = [
  { id: 'sku', label: 'SKU', width: '120px', align: 'left', locked: true, sortable: true },
  { id: 'identifier', label: 'Identificador', width: '140px', align: 'left', sortable: false },
  {
    id: 'name',
    label: 'Produto',
    width: 'minmax(320px, 2.2fr)',
    align: 'left',
    locked: true,
    sortable: true,
  },
  {
    id: 'description',
    label: 'Descricao',
    width: 'minmax(260px, 1.5fr)',
    align: 'left',
    sortable: false,
  },
  {
    id: 'supplierReference',
    label: 'Ref. fornecedor',
    width: '150px',
    align: 'left',
    sortable: false,
  },
  { id: 'brandName', label: 'Marca', width: '140px', align: 'left', sortable: false },
  { id: 'seasonName', label: 'Colecao', width: '140px', align: 'left', sortable: false },
  { id: 'category1', label: 'Categoria', width: '150px', align: 'left', sortable: false },
  { id: 'category2', label: 'Subcategoria', width: '170px', align: 'left', sortable: false },
  { id: 'category3', label: 'Linha', width: '150px', align: 'left', sortable: false },
  { id: 'size', label: 'Tam.', width: '90px', align: 'center', sortable: false },
  { id: 'color', label: 'Cor', width: '110px', align: 'left', sortable: false },
  { id: 'unit', label: 'Un.', width: '80px', align: 'center', sortable: false },
  { id: 'priceRaw', label: 'Preco', width: '120px', align: 'right', locked: true, sortable: true },
  { id: 'sourceUpdatedAt', label: 'Atualizado', width: '160px', align: 'left', sortable: true },
]

export const ERP_RECORDS_COLUMNS_BY_TAB: Record<string, ErpGridColumn[]> = {
  clientes: [
    {
      id: 'name',
      label: 'Nome',
      width: 'minmax(240px, 1.8fr)',
      align: 'left',
      locked: true,
      sortable: true,
    },
    { id: 'nickname', label: 'Apelido', width: '160px', align: 'left', sortable: false },
    { id: 'cpf', label: 'CPF', width: '150px', align: 'left', sortable: true },
    { id: 'email', label: 'Email', width: 'minmax(230px, 1.5fr)', align: 'left', sortable: false },
    { id: 'phone', label: 'Telefone', width: '150px', align: 'left', sortable: false },
    { id: 'mobile', label: 'Celular', width: '150px', align: 'left', sortable: false },
    { id: 'gender', label: 'Genero', width: '110px', align: 'center', sortable: false },
    { id: 'birthday_raw', label: 'Nascimento', width: '140px', align: 'left', sortable: false },
    {
      id: 'street',
      label: 'Endereco',
      width: 'minmax(220px, 1.4fr)',
      align: 'left',
      sortable: false,
    },
    { id: 'number', label: 'Numero', width: '100px', align: 'left', sortable: false },
    { id: 'complement', label: 'Complemento', width: '150px', align: 'left', sortable: false },
    { id: 'neighborhood', label: 'Bairro', width: '160px', align: 'left', sortable: false },
    { id: 'city', label: 'Cidade', width: '170px', align: 'left', sortable: true },
    { id: 'uf', label: 'UF', width: '90px', align: 'center', sortable: false },
    { id: 'country', label: 'Pais', width: '100px', align: 'center', sortable: false },
    { id: 'zipcode', label: 'CEP', width: '130px', align: 'left', sortable: false },
    { id: 'employee_id', label: 'Funcionario', width: '130px', align: 'left', sortable: false },
    { id: 'store_id_raw', label: 'Store ID ERP', width: '150px', align: 'left', sortable: false },
    { id: 'registered_at_raw', label: 'Cadastro', width: '170px', align: 'left', sortable: true },
    { id: 'original_id', label: 'ID original', width: '140px', align: 'left', sortable: false },
    { id: 'identifier', label: 'Identificador', width: '140px', align: 'left', sortable: false },
    { id: 'tags', label: 'Tags', width: 'minmax(180px, 1fr)', align: 'left', sortable: false },
  ],
  funcionarios: [
    {
      id: 'name',
      label: 'Nome',
      width: 'minmax(240px, 1.8fr)',
      align: 'left',
      locked: true,
      sortable: true,
    },
    { id: 'store_id_raw', label: 'Store ID ERP', width: '150px', align: 'left', sortable: false },
    { id: 'original_id', label: 'ID original', width: '150px', align: 'left', sortable: false },
    {
      id: 'street',
      label: 'Endereco',
      width: 'minmax(220px, 1.3fr)',
      align: 'left',
      sortable: false,
    },
    { id: 'complement', label: 'Complemento', width: '150px', align: 'left', sortable: false },
    { id: 'city', label: 'Cidade', width: '170px', align: 'left', sortable: false },
    { id: 'uf', label: 'UF', width: '90px', align: 'center', sortable: false },
    { id: 'zipcode', label: 'CEP', width: '130px', align: 'left', sortable: false },
    { id: 'is_active_raw', label: 'Ativo', width: '110px', align: 'center', sortable: false },
  ],
  pedidos: [
    {
      id: 'order_id',
      label: 'Compra',
      width: '160px',
      align: 'left',
      locked: true,
      sortable: false,
    },
    { id: 'identifier', label: 'Identificador', width: '140px', align: 'left', sortable: false },
    { id: 'store_id_raw', label: 'Store ID ERP', width: '150px', align: 'left', sortable: false },
    { id: 'customer_id', label: 'Cliente', width: '130px', align: 'left', sortable: true },
    { id: 'order_date_raw', label: 'Data', width: '140px', align: 'left', sortable: true },
    {
      id: 'total_amount_raw',
      label: 'Total compra',
      width: '130px',
      align: 'right',
      sortable: true,
    },
    {
      id: 'product_return_raw',
      label: 'Devolucao',
      width: '120px',
      align: 'right',
      sortable: false,
    },
    { id: 'sku', label: 'SKUs', width: '220px', align: 'left', sortable: false },
    { id: 'amount_raw', label: 'Valor itens', width: '120px', align: 'right', sortable: false },
    { id: 'quantity_raw', label: 'Qtd total', width: '100px', align: 'right', sortable: false },
    { id: 'employee_id', label: 'Funcionario', width: '130px', align: 'left', sortable: false },
    { id: 'payment_type', label: 'Pagamento', width: '140px', align: 'left', sortable: false },
    {
      id: 'total_exclusion_raw',
      label: 'Exclusao',
      width: '120px',
      align: 'right',
      sortable: false,
    },
    { id: 'total_debit_raw', label: 'Debito', width: '120px', align: 'right', sortable: false },
  ],
  cancelados: [
    {
      id: 'order_id',
      label: 'Compra',
      width: '160px',
      align: 'left',
      locked: true,
      sortable: false,
    },
    { id: 'identifier', label: 'Identificador', width: '140px', align: 'left', sortable: false },
    { id: 'store_id_raw', label: 'Store ID ERP', width: '150px', align: 'left', sortable: false },
    { id: 'customer_id', label: 'Cliente', width: '130px', align: 'left', sortable: true },
    { id: 'order_date_raw', label: 'Data', width: '140px', align: 'left', sortable: true },
    {
      id: 'total_amount_raw',
      label: 'Total compra',
      width: '130px',
      align: 'right',
      sortable: true,
    },
    {
      id: 'product_return_raw',
      label: 'Devolucao',
      width: '120px',
      align: 'right',
      sortable: false,
    },
    { id: 'sku', label: 'SKUs', width: '220px', align: 'left', sortable: false },
    { id: 'amount_raw', label: 'Valor itens', width: '120px', align: 'right', sortable: false },
    { id: 'quantity_raw', label: 'Qtd total', width: '100px', align: 'right', sortable: false },
    { id: 'employee_id', label: 'Funcionario', width: '130px', align: 'left', sortable: false },
    { id: 'payment_type', label: 'Pagamento', width: '140px', align: 'left', sortable: false },
    {
      id: 'total_exclusion_raw',
      label: 'Exclusao',
      width: '120px',
      align: 'right',
      sortable: false,
    },
    { id: 'total_debit_raw', label: 'Debito', width: '120px', align: 'right', sortable: false },
  ],
}

export const ERP_PAGE_SIZE_OPTIONS = [25, 50, 100, 200]

export const ERP_RECORDS_DATA_TYPE_BY_TAB: Record<string, string> = {
  clientes: 'customer',
  funcionarios: 'employee',
  pedidos: 'order',
  cancelados: 'ordercanceled',
}

export const ERP_RECORDS_LABEL_BY_TAB: Record<string, string> = {
  clientes: 'clientes',
  funcionarios: 'funcionarios',
  pedidos: 'compras',
  cancelados: 'cancelados',
}

export const ERP_RECORDS_BOOTSTRAP_LABEL_BY_TAB: Record<string, string> = {
  clientes: 'Bootstrap clientes ERP',
  funcionarios: 'Bootstrap funcionarios ERP',
  pedidos: 'Bootstrap compras ERP',
  cancelados: 'Bootstrap cancelados ERP',
}

export const ERP_RECORDS_SPECIFIC_SEARCH_BY_TAB: Record<string, ErpSpecificSearch> = {
  clientes: { label: 'CPF (comeca com)', placeholder: 'Ex: 123.456.789-00' },
  funcionarios: { label: 'ID funcionario (comeca com)', placeholder: 'Ex: 315' },
  pedidos: { label: 'Compra (comeca com)', placeholder: 'Ex: 315578' },
  cancelados: { label: 'Compra cancelada (comeca com)', placeholder: 'Ex: 315578' },
}

export const ERP_RECORDS_GENERAL_SEARCH_PLACEHOLDER_BY_TAB: Record<string, string> = {
  clientes: 'Busca geral (nome, email, telefone, cidade, store ID, tags...)',
  funcionarios: 'Busca geral (nome, store ID, cidade, UF, endereco, status...)',
  pedidos: 'Busca geral (compra, store ID, cliente, SKU, valor, funcionario...)',
  cancelados: 'Busca geral (compra cancelada, store ID, cliente, SKU, valor...)',
}

export const ERP_BANCO_SECTION_BY_TAB: Record<string, ErpBancoSection> = {
  geral: {
    title: 'Estrutura geral do modulo ERP',
    text: 'O desenho separa controle de execucao, espelho raw e projecao de leitura. Isso garante trilha auditavel sem perder performance nas consultas do painel.',
    note: 'Sub-lojas entram por linha em store_cnpj nas tabelas raw. A projecao atual prioriza SKU por tenant/loja e pode receber filtro por CNPJ na proxima camada.',
    cards: [
      {
        table: 'erp_sync_runs',
        label: 'Runs de sincronizacao',
        desc: 'Controle de cada execucao, status e contadores por tipo.',
        badge: 'controle',
      },
      {
        table: 'erp_sync_files',
        label: 'Lotes processados',
        desc: 'Checksum por arquivo para idempotencia e reprocessamento seguro.',
        badge: 'controle',
      },
      {
        table: 'erp_item_raw',
        label: 'Raw de produtos',
        desc: 'Historico bruto linha a linha vindo do consolidado markdown.',
        badge: 'raw',
      },
      {
        table: 'erp_item_current',
        label: 'Catalogo atual',
        desc: 'Projecao deduplicada por SKU para consultas de produtos.',
        badge: 'projecao',
      },
    ],
  },
  produtos: {
    title: 'Banco da frente de Produtos',
    text: 'Produtos possuem pipeline completo: gravacao raw e atualizacao de projecao atual com upsert por SKU.',
    note: 'A tabela erp_item_current alimenta a grade principal e ja respeita paginacao/busca administrativa.',
    cards: [
      {
        table: 'erp_item_raw',
        label: 'Itens raw',
        desc: 'Espelho integral das linhas de item importadas por lote.',
        badge: 'raw',
      },
      {
        table: 'erp_item_current',
        label: 'Itens atuais',
        desc: 'Camada otimizada para leitura no painel, 1 registro por SKU.',
        badge: 'projecao',
      },
      {
        table: 'erp_sync_files',
        label: 'Arquivos de item',
        desc: 'Metadados e checksum dos lotes que atualizaram produtos.',
        badge: 'controle',
      },
    ],
  },
  clientes: {
    title: 'Banco da frente de Clientes',
    text: 'Clientes usam tabela raw dedicada com todos os campos de origem para auditoria e busca administrativa.',
    note: 'A leitura da aba Clientes vem de erp_customer_raw via endpoint paginado do modulo ERP.',
    cards: [
      {
        table: 'erp_customer_raw',
        label: 'Clientes raw',
        desc: 'Nome, CPF, email, contato e identificador por linha de origem.',
        badge: 'raw',
      },
      {
        table: 'erp_sync_files',
        label: 'Lotes de customer',
        desc: 'Controle de importacao e deduplicacao por checksum.',
        badge: 'controle',
      },
    ],
  },
  pedidos: {
    title: 'Banco da frente de Compras',
    text: 'Compras ativas ficam em tabela raw propria com valores brutos e campos normalizados de apoio.',
    note: 'A aba Compras consulta erp_order_raw e preserva referencia de lote, linha e tipo de pagamento.',
    cards: [
      {
        table: 'erp_order_raw',
        label: 'Compras raw',
        desc: 'Compra, cliente, SKU, valores e metadados de origem.',
        badge: 'raw',
      },
      {
        table: 'erp_sync_files',
        label: 'Lotes de order',
        desc: 'Historico de arquivos importados para compras.',
        badge: 'controle',
      },
    ],
  },
  cancelados: {
    title: 'Banco da frente de Cancelados',
    text: 'Cancelados seguem o mesmo contrato de compras, em tabela separada para governanca e filtros dedicados.',
    note: 'Separar order e ordercanceled evita ambiguidades em indicadores e trilhas de reconciliacao.',
    cards: [
      {
        table: 'erp_order_canceled_raw',
        label: 'Compras canceladas raw',
        desc: 'Mesma base de campos de order, isolada para cancelamentos.',
        badge: 'raw',
      },
      {
        table: 'erp_sync_files',
        label: 'Lotes de ordercanceled',
        desc: 'Controle de lotes importados para cancelados.',
        badge: 'controle',
      },
    ],
  },
  funcionarios: {
    title: 'Banco da frente de Funcionarios',
    text: 'Funcionarios entram em tabela raw especifica com dados cadastrais e status de atividade.',
    note: 'A consulta da aba Funcionarios usa erp_employee_raw com paginacao e busca textual.',
    cards: [
      {
        table: 'erp_employee_raw',
        label: 'Funcionarios raw',
        desc: 'ID original, nome, cidade, UF e indicador de ativo.',
        badge: 'raw',
      },
      {
        table: 'erp_sync_files',
        label: 'Lotes de employee',
        desc: 'Trilha de importacoes da frente de funcionarios.',
        badge: 'controle',
      },
    ],
  },
  outbox: {
    title: 'Banco da frente de Outbox',
    text: 'Outbox prepara integracao incremental com outros bancos/servicos sem acoplar na importacao principal.',
    note: 'Quando ativado, processa eventos pendentes com retries e controle de disponibilidade.',
    cards: [
      {
        table: 'erp_export_outbox',
        label: 'Outbox de exportacao',
        desc: 'Fila de eventos ERP para sincronizacoes futuras.',
        badge: 'outbox',
      },
      {
        table: 'erp_sync_runs',
        label: 'Relacionamento com runs',
        desc: 'Permite rastrear de qual ciclo de importacao partiu o evento.',
        badge: 'controle',
      },
    ],
  },
}

export function formatCurrency(cents: number | null | undefined) {
  const n = Number(cents || 0)
  if (!n) return 'R$ 0,00'
  return new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' }).format(n / 100)
}

export function castRow(v: unknown): Record<string, unknown> {
  return v as Record<string, unknown>
}

export function formatDateTime(value?: string | null) {
  const normalized = String(value || '').trim()
  if (!normalized) {
    return '-'
  }

  const parsed = new Date(normalized)
  if (Number.isNaN(parsed.getTime())) {
    return normalized
  }

  const datePart = parsed
    .toLocaleDateString('pt-BR', {
      day: 'numeric',
      month: 'short',
      year: 'numeric',
    })
    .replace(/\. de /g, ' ')
    .replace(/\.$/, '')

  const timePart = parsed.toLocaleTimeString('pt-BR', {
    hour: '2-digit',
    minute: '2-digit',
  })

  return `${datePart} as ${timePart}`
}

export function formatNumber(value: number | null | undefined) {
  const n = Number(value || 0)
  return n.toLocaleString('pt-BR')
}

export function formatSourceFileName(sourceName?: string | null): string {
  if (!sourceName) return '-'
  const match = sourceName.match(/^(\d{4})(\d{2})(\d{2})(\d{2})(\d{2})(\d{2})/)
  if (!match) return sourceName
  const [, year, month, day, hour, minute] = match
  const parsed = new Date(`${year}-${month}-${day}T${hour}:${minute}:00`)
  if (Number.isNaN(parsed.getTime())) return sourceName
  return formatDateTime(parsed.toISOString())
}

export function formatPrice(rawValue?: string, cents?: number | null) {
  const numericCents = Number.isFinite(Number(cents)) ? Number(cents) : Number(rawValue || 0)
  if (!numericCents) {
    return '-'
  }

  return new Intl.NumberFormat('pt-BR', {
    style: 'currency',
    currency: 'BRL',
  }).format(numericCents / 100)
}

export function productRowKey(row: Record<string, unknown>) {
  return `${String(row.sku)}-${String(row.identifier)}`
}

export function recordsRowKey(row: Record<string, unknown>, index: number) {
  return String(row.id || row.order_id || row.original_id || row.identifier || row.cpf || index)
}

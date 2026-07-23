export type BiApiFieldType = 'string' | 'number' | 'boolean' | 'null-observed'

export interface BiApiField {
  key: string
  type: BiApiFieldType
}

export interface BiApiFieldGroup {
  id: string
  label: string
  fields: BiApiField[]
}

export interface BiApiEntity {
  id: string
  label: string
  endpoint: string
  icon: 'package' | 'image' | 'coins' | 'receipt' | 'list' | 'warehouse'
  description: string
  availableInformation: string[]
  fieldGroups: BiApiFieldGroup[]
  relation?: string
  queryRule: string
  performance: string
  tone: 'default' | 'attention' | 'sensitive'
}

export interface BiApiSchemaRow {
  id: string
  entityId: string
  entity: string
  endpoint: string
  groupId: string
  group: string
  field: string
  fieldLabel: string
  type: BiApiFieldType
  typeLabel: string
}

export interface BiApiSchemaFilters {
  search?: string
  entityId?: string
  groupId?: string
  type?: string
}

const field = (key: string, type: BiApiFieldType): BiApiField => ({ key, type })

const group = (id: string, label: string, fields: BiApiField[]): BiApiFieldGroup => ({
  id,
  label,
  fields,
})

export const BI_API_ENTITIES: BiApiEntity[] = [
  {
    id: 'item',
    label: 'Item',
    endpoint: '/item/find',
    icon: 'package',
    description:
      'Cadastro e classificação comercial do produto, com atributos de joias e relógios.',
    availableInformation: [
      'Referência, fornecedor e unidade',
      'Departamento, classe, tipo e subtipo',
      'Marca, coleção, estilo e linha fundamental',
      'Material, cor, tamanho, formato e pedras',
      'Características técnicas de relógios e acabamentos',
      'Não trouxe preço na amostra; valores vivem nas entidades de saldo e nota',
    ],
    fieldGroups: [
      group('identification', 'Identificação', [
        field('id', 'number'),
        field('itemId', 'number'),
        field('referencia', 'string'),
        field('fornecedor', 'string'),
        field('un', 'string'),
        field('obs', 'string'),
      ]),
      group('classification', 'Classificação comercial', [
        field('departamento', 'string'),
        field('departamentoId', 'number'),
        field('classe', 'string'),
        field('classeId', 'number'),
        field('tipo', 'string'),
        field('tipoId', 'number'),
        field('subtipo', 'string'),
        field('subTipoId', 'number'),
        field('colecao', 'null-observed'),
        field('colecaoId', 'null-observed'),
        field('marca', 'string'),
        field('marcaId', 'number'),
        field('estilo', 'string'),
        field('estiloId', 'number'),
        field('fundamental', 'string'),
        field('fundamentalId', 'number'),
      ]),
      group('attributes', 'Atributos do produto', [
        field('material', 'string'),
        field('materialId', 'number'),
        field('cor', 'string'),
        field('corId', 'number'),
        field('tamanho', 'string'),
        field('formato', 'string'),
        field('formatoId', 'number'),
        field('pedras', 'string'),
        field('acabamentos', 'string'),
        field('pesoBruto', 'null-observed'),
        field('pesoLiquido', 'null-observed'),
      ]),
      group('watch', 'Atributos técnicos', [
        field('movimento', 'null-observed'),
        field('movimentoId', 'null-observed'),
        field('vidro', 'null-observed'),
        field('vidroId', 'null-observed'),
        field('visor', 'null-observed'),
        field('visorId', 'null-observed'),
        field('mostradores', 'null-observed'),
        field('funcoes', 'null-observed'),
        field('resistenciaAgua', 'null-observed'),
        field('coresPulseiras', 'null-observed'),
        field('materiaisPulseiras', 'null-observed'),
        field('preenchimento', 'null-observed'),
        field('preenchimentoId', 'null-observed'),
        field('tipoFabricacao', 'null-observed'),
        field('tipoFabricacaoId', 'null-observed'),
        field('moeda', 'null-observed'),
        field('moedaId', 'null-observed'),
      ]),
      group('audit', 'Controle da origem', [
        field('created', 'string'),
        field('modified', 'string'),
      ]),
    ],
    queryRule: 'Consulta paginada. Usar busca e filtros antes de ampliar a página.',
    performance: 'Amostra de 1 registro respondeu em aproximadamente 0,6 s.',
    tone: 'default',
  },
  {
    id: 'image-item',
    label: 'Imagem do item',
    endpoint: '/imagemItem/find',
    icon: 'image',
    description: 'Arquivos de imagem associados ao cadastro de um item.',
    availableInformation: [
      'Nome do arquivo de imagem',
      'Item ao qual a imagem pertence',
      'Ordem de exibição quando informada',
    ],
    fieldGroups: [
      group('image', 'Arquivo e vínculo', [
        field('id', 'number'),
        field('itemId', 'number'),
        field('filename', 'string'),
        field('ordem', 'null-observed'),
      ]),
    ],
    relation: '`itemId` liga a imagem ao cadastro de Item.',
    queryRule: 'Preferir consulta por itemId; não assumir que filename é uma URL pública.',
    performance: 'Amostra de 1 registro respondeu em aproximadamente 0,6 s.',
    tone: 'default',
  },
  {
    id: 'purchase-price',
    label: 'Saldo e preço de compra',
    endpoint: '/itemSaldoPrecoCompra/find',
    icon: 'coins',
    description: 'Historico de custo, entrada e preco medio associado ao saldo do item.',
    availableInformation: [
      'Preço de custo e respectiva moeda',
      'Preço de entrada e respectiva moeda',
      'Preço médio e respectiva moeda',
      'Empresa e data de referência',
      'Vínculos com saldo do item e item da nota',
    ],
    fieldGroups: [
      group('relation', 'Identificação e vínculos', [
        field('id', 'number'),
        field('itemSaldoId', 'number'),
        field('notaItemId', 'null-observed'),
        field('empresaId', 'number'),
        field('data', 'string'),
      ]),
      group('prices', 'Valores de compra', [
        field('precoCusto', 'string'),
        field('precoCustoMoeda', 'string'),
        field('precoEntrada', 'string'),
        field('precoEntradaMoeda', 'string'),
        field('precoMedio', 'string'),
        field('precoMedioMoeda', 'string'),
      ]),
    ],
    relation: '`itemSaldoId` conecta preços, Nota Item e Inventário.',
    queryRule: 'Valores monetários chegam como texto e exigem conversão decimal controlada.',
    performance: 'Amostra de 1 registro respondeu em aproximadamente 0,9 s.',
    tone: 'default',
  },
  {
    id: 'invoice',
    label: 'Nota',
    endpoint: '/nota/find',
    icon: 'receipt',
    description:
      'Cabecalho fiscal e comercial da nota, incluindo empresa, vendedor, cliente e totais.',
    availableInformation: [
      'Documento, série, tipo e datas da nota',
      'Empresa de origem e colaborador responsável',
      'Cliente, documento, nascimento e endereço',
      'Frete, impostos, descontos, acréscimos e totais',
      'Status de exclusão e observações',
    ],
    fieldGroups: [
      group('document', 'Documento e datas', [
        field('id', 'number'),
        field('numDocumento', 'string'),
        field('serie', 'string'),
        field('tipoNota', 'string'),
        field('tipoNotaSigla', 'string'),
        field('dataEmissao', 'string'),
        field('dataSaidaEntrada', 'string'),
        field('origemDestino', 'string'),
        field('origemDestinoId', 'number'),
        field('origemDestinoSigla', 'string'),
        field('excluido', 'boolean'),
        field('obs', 'string'),
        field('informacoesComplementares', 'null-observed'),
      ]),
      group('company', 'Empresa e colaborador', [
        field('empresaId', 'number'),
        field('empresaCnpj', 'string'),
        field('empresaFantasia', 'string'),
        field('empresaRazaoSocial', 'string'),
        field('empresaSigla', 'string'),
        field('colaboradorId', 'number'),
        field('colaboradorCpfCnpj', 'string'),
        field('colaboradorNome', 'string'),
      ]),
      group('customer', 'Cliente e endereço (PII)', [
        field('pessoaNomeRazaoSocial', 'string'),
        field('pessoaCpfCnpj', 'string'),
        field('pessoaRgIe', 'string'),
        field('pessoaDatNascimento', 'string'),
        field('pessoaLogradouro', 'string'),
        field('pessoaNumero', 'string'),
        field('pessoaBairro', 'string'),
        field('pessoaCep', 'string'),
        field('pessoaCidade', 'string'),
        field('pessoaUf', 'string'),
      ]),
      group('taxes', 'Valores e impostos', [
        field('valorTotal', 'string'),
        field('valorTotalItens', 'string'),
        field('valorDesconto', 'string'),
        field('valorAcrescimo', 'string'),
        field('valorFrete', 'string'),
        field('valorSeguro', 'string'),
        field('valorOutrasDespesas', 'string'),
        field('valorOutrosCustos', 'string'),
        field('valorVendaAutorizacao', 'string'),
        field('valorCotacaoMoeda', 'null-observed'),
        field('baseIcms', 'string'),
        field('baseIcmsSt', 'string'),
        field('valorIcms', 'string'),
        field('valorIcmsSt', 'string'),
        field('valorIpi', 'string'),
        field('valorPis', 'string'),
        field('valorCofins', 'string'),
        field('valorConhecimentoFrete', 'string'),
        field('tipoFrete', 'null-observed'),
      ]),
      group('audit', 'Controle da origem', [
        field('created', 'string'),
        field('modified', 'string'),
      ]),
    ],
    relation: '`id` é referenciado por `notaId` em Nota Item.',
    queryRule:
      'Fonte cara e sensível. Exigir período/filtro e projetar somente os campos necessários.',
    performance: 'Amostra de 1 registro respondeu em aproximadamente 6,2 s.',
    tone: 'sensitive',
  },
  {
    id: 'invoice-item',
    label: 'Item da nota',
    endpoint: '/notaItem/find',
    icon: 'list',
    description: 'Linhas vendidas ou devolvidas da nota, com quantidade, precos e colaborador.',
    availableInformation: [
      'Nota e saldo do item relacionados',
      'Quantidade vendida e devolvida',
      'Preço unitário, total, custo, entrada e médio',
      'Desconto, acréscimo e valor de devolução',
      'Colaborador e operação de estoque',
    ],
    fieldGroups: [
      group('relation', 'Identificação e vínculos', [
        field('id', 'number'),
        field('notaId', 'number'),
        field('itemSaldoId', 'number'),
        field('estoqueOperacao', 'string'),
        field('excluido', 'boolean'),
      ]),
      group('collaborator', 'Colaborador', [
        field('colaboradorId', 'number'),
        field('colaboradorCpfCnpj', 'string'),
        field('colaboradorNome', 'string'),
      ]),
      group('quantity', 'Quantidades', [
        field('quantidade', 'string'),
        field('quantidadeDevolvida', 'number'),
      ]),
      group('values', 'Preços e valores', [
        field('precoUnitario', 'string'),
        field('precoTotal', 'string'),
        field('precoCusto', 'string'),
        field('precoCustoMoeda', 'string'),
        field('precoEntrada', 'string'),
        field('precoEntradaMoeda', 'string'),
        field('precoMedio', 'string'),
        field('precoMedioMoeda', 'string'),
        field('valorDesconto', 'string'),
        field('valorAcrescimo', 'string'),
        field('valorDevolucao', 'string'),
      ]),
      group('audit', 'Controle da origem', [
        field('created', 'null-observed'),
        field('modified', 'null-observed'),
      ]),
    ],
    relation: '`notaId` liga a Nota; `itemSaldoId` liga saldo, preços e Inventário.',
    queryRule: 'Consultar por nota, saldo, período ou outro filtro seletivo.',
    performance: 'Amostra de 1 registro respondeu em aproximadamente 2,6 s.',
    tone: 'sensitive',
  },
  {
    id: 'inventory',
    label: 'Inventário',
    endpoint: '/inventario/find',
    icon: 'warehouse',
    description: 'Movimentos e ajustes de quantidade por saldo do item e empresa.',
    availableInformation: [
      'Saldo do item relacionado',
      'Empresa responsável pelo movimento',
      'Data, quantidade e tipo do inventário',
      'Sigla do tipo de movimento',
    ],
    fieldGroups: [
      group('movement', 'Movimento', [
        field('id', 'number'),
        field('itemSaldoId', 'number'),
        field('data', 'string'),
        field('quantidade', 'string'),
        field('tipoInventario', 'string'),
        field('tipoInventarioSigla', 'string'),
      ]),
      group('company', 'Empresa', [
        field('empresaId', 'number'),
        field('empresaCnpj', 'string'),
        field('empresaFantasia', 'string'),
        field('empresaRazaoSocial', 'string'),
        field('empresaSigla', 'string'),
      ]),
      group('audit', 'Controle da origem', [field('created', 'string')]),
    ],
    relation: '`itemSaldoId` conecta o movimento aos itens de nota e preços de compra.',
    queryRule:
      'Filtro seletivo obrigatório. A consulta aberta não é permitida, mesmo com limitação de resposta.',
    performance:
      'Busca aberta com limit 1 excedeu 35 s; filtrada por itemSaldoId respondeu em aproximadamente 0,4 s.',
    tone: 'attention',
  },
]

const FIELD_LABELS: Record<string, string> = {
  id: 'ID do registro',
  itemId: 'ID do item',
  itemSaldoId: 'ID do saldo do item',
  notaId: 'ID da nota',
  notaItemId: 'ID do item da nota',
  empresaId: 'ID da empresa',
  colaboradorId: 'ID do colaborador',
  departamentoId: 'ID do departamento',
  classeId: 'ID da classe',
  tipoId: 'ID do tipo',
  subTipoId: 'ID do subtipo',
  colecaoId: 'ID da coleção',
  marcaId: 'ID da marca',
  estiloId: 'ID do estilo',
  fundamentalId: 'ID da linha fundamental',
  materialId: 'ID do material',
  corId: 'ID da cor',
  formatoId: 'ID do formato',
  movimentoId: 'ID do movimento',
  vidroId: 'ID do vidro',
  visorId: 'ID do visor',
  preenchimentoId: 'ID do preenchimento',
  tipoFabricacaoId: 'ID do tipo de fabricação',
  moedaId: 'ID da moeda',
  numDocumento: 'Numero do documento',
  pessoaDatNascimento: 'Data de nascimento',
  pessoaCpfCnpj: 'CPF/CNPJ do cliente',
  pessoaRgIe: 'RG/inscricao estadual',
  colaboradorCpfCnpj: 'CPF/CNPJ do colaborador',
  empresaCnpj: 'CNPJ da empresa',
  empresaFantasia: 'Nome fantasia',
  empresaRazaoSocial: 'Razão social',
  precoCusto: 'Preço de custo',
  precoEntrada: 'Preço de entrada',
  precoMedio: 'Preço médio',
  precoUnitario: 'Preço unitário',
  precoTotal: 'Preço total',
  valorTotal: 'Valor total',
  valorTotalItens: 'Valor total dos itens',
  created: 'Criado em',
  modified: 'Alterado em',
}

export function biApiFieldLabel(key: string) {
  const override = FIELD_LABELS[key]
  if (override) return override

  const spaced = key.replace(/([a-z0-9])([A-Z])/g, '$1 $2').replace(/[_-]+/g, ' ')
  return spaced.charAt(0).toUpperCase() + spaced.slice(1)
}

export function biApiFieldTypeLabel(type: BiApiFieldType) {
  if (type === 'number') return 'Número'
  if (type === 'boolean') return 'Sim/não'
  if (type === 'null-observed') return 'Opcional · tipo a confirmar'
  return 'Texto'
}

export function biApiEntityFieldCount(entity: BiApiEntity) {
  return entity.fieldGroups.reduce((total, current) => total + current.fields.length, 0)
}

export function biApiSchemaRows(entities: BiApiEntity[] = BI_API_ENTITIES): BiApiSchemaRow[] {
  return entities.flatMap((entity) =>
    entity.fieldGroups.flatMap((fieldGroup) =>
      fieldGroup.fields.map((item) => ({
        id: `${entity.id}:${fieldGroup.id}:${item.key}`,
        entityId: entity.id,
        entity: entity.label,
        endpoint: entity.endpoint,
        groupId: fieldGroup.id,
        group: fieldGroup.label,
        field: item.key,
        fieldLabel: biApiFieldLabel(item.key),
        type: item.type,
        typeLabel: biApiFieldTypeLabel(item.type),
      })),
    ),
  )
}

function normalizeSchemaSearch(value: unknown) {
  return String(value || '')
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLocaleLowerCase('pt-BR')
    .trim()
}

export function filterBiApiSchemaRows(
  rows: BiApiSchemaRow[],
  filters: BiApiSchemaFilters,
): BiApiSchemaRow[] {
  const search = normalizeSchemaSearch(filters.search)
  const entityId = String(filters.entityId || '').trim()
  const groupId = String(filters.groupId || '').trim()
  const type = String(filters.type || '').trim()

  return rows.filter((row) => {
    if (entityId && entityId !== 'all' && row.entityId !== entityId) return false
    if (groupId && groupId !== 'all' && row.groupId !== groupId) return false
    if (type && type !== 'all' && row.type !== type) return false
    if (!search) return true

    return normalizeSchemaSearch(
      [row.entity, row.endpoint, row.group, row.field, row.fieldLabel, row.typeLabel].join(' '),
    ).includes(search)
  })
}

export type BiIntelligenceSourceId = 'perola' | 'erp' | 'queue'
export type BiIntelligenceReadiness = 'data-ready' | 'safe-query' | 'mapping-gap'

export interface BiIntelligenceSource {
  id: BiIntelligenceSourceId
  label: string
  shortLabel: string
  description: string
}

export interface BiIntelligenceOpportunity {
  id: string
  title: string
  question: string
  outcome: string
  sources: BiIntelligenceSourceId[]
  readiness: BiIntelligenceReadiness
  ingredients: string[]
  dimensions: string[]
  guardrail?: string
}

export interface BiSourceComparison {
  domain: string
  perola: string
  erp: string
  queue: string
}

export const BI_INTELLIGENCE_SOURCES: BiIntelligenceSource[] = [
  {
    id: 'perola',
    label: 'BI Pérola',
    shortLabel: 'BI',
    description: 'Documento fiscal, custos, atributos de produto, imagens e inventário.',
  },
  {
    id: 'erp',
    label: 'ERP interno',
    shortLabel: 'ERP',
    description: 'Cadastro comercial, clientes contatáveis, pedidos, pagamentos e cancelamentos.',
  },
  {
    id: 'queue',
    label: 'Fila de atendimento',
    shortLabel: 'Fila',
    description: 'Jornada na loja, conversão, tempos, motivos, campanhas e qualidade operacional.',
  },
]

export const BI_INTELLIGENCE_READINESS: Record<
  BiIntelligenceReadiness,
  { label: string; description: string }
> = {
  'data-ready': {
    label: 'Ingredientes disponíveis',
    description: 'Os dados já existem no PostgreSQL ou em contrato confirmado.',
  },
  'safe-query': {
    label: 'Consulta BI controlada',
    description: 'Exige endpoint por período, filtro e paginação antes de calcular.',
  },
  'mapping-gap': {
    label: 'Chave de ligação pendente',
    description: 'Os dados existem, mas falta confirmar a chave segura entre as fontes.',
  },
}

export const BI_INTELLIGENCE_OPPORTUNITIES: BiIntelligenceOpportunity[] = [
  {
    id: 'commercial-performance',
    title: 'Desempenho comercial completo',
    question: 'Quem vende, converte e usa bem a fila?',
    outcome:
      'Compara venda, ticket, PA, atingimento de meta, atendimentos, conversão e cancelamentos por loja e consultor.',
    sources: ['erp', 'queue'],
    readiness: 'data-ready',
    ingredients: [
      'Pedidos, unidades, faturamento, ticket e PA',
      'Metas, progresso e comissões calculadas',
      'Atendimentos, conversões e cancelamentos da fila',
    ],
    dimensions: ['Período', 'Loja', 'Consultor'],
  },
  {
    id: 'customer-segmentation',
    title: 'Segmentação e recompra',
    question: 'Quem compra, com que frequência e como podemos contatar?',
    outcome:
      'Forma segmentos por recência, frequência e valor, além de aniversário, cidade, gênero, tags e canal de contato.',
    sources: ['erp'],
    readiness: 'data-ready',
    ingredients: [
      'Cliente com CPF, e-mail, telefone e celular',
      'Data de cadastro, nascimento, gênero, cidade e tags',
      'Pedidos, produtos, valores e vendedor relacionados',
    ],
    dimensions: ['Cliente', 'Cidade', 'Loja', 'Consultor', 'Produto'],
    guardrail: 'CPF e contatos são PII: segmentar no backend e retornar somente o necessário.',
  },
  {
    id: 'payment-cancellation',
    title: 'Pagamento, devolução e cancelamento',
    question: 'Onde perdemos receita depois da venda?',
    outcome:
      'Mostra preferência de pagamento, pedidos cancelados, devoluções, exclusões e débitos por loja, vendedor e período.',
    sources: ['erp'],
    readiness: 'data-ready',
    ingredients: [
      'Forma de pagamento por pedido',
      'Base separada de pedidos cancelados',
      'Valores de devolução, exclusão e débito',
    ],
    dimensions: ['Pagamento', 'Loja', 'Consultor', 'Período'],
  },
  {
    id: 'service-quality',
    title: 'Qualidade da jornada na loja',
    question: 'O que aconteceu antes de vender ou perder a venda?',
    outcome:
      'Explica conversão com tempo de espera, duração, motivo da visita, origem do cliente, produto procurado e qualidade do preenchimento.',
    sources: ['queue'],
    readiness: 'data-ready',
    ingredients: [
      'Desfecho, valor informado e cliente novo ou existente',
      'Espera, duração, furo de fila e pausas',
      'Produtos vistos, fechados e não encontrados',
      'Motivos, origem, profissão, campanhas e observações',
    ],
    dimensions: ['Loja', 'Consultor', 'Horário', 'Motivo', 'Origem'],
  },
  {
    id: 'fiscal-margin',
    title: 'Margem e carga fiscal',
    question: 'Quanto sobra de cada venda depois de custo, desconto e imposto?',
    outcome:
      'Calcula margem bruta estimada e peso fiscal por nota, empresa, consultor e período usando os valores da própria Pérola.',
    sources: ['perola'],
    readiness: 'safe-query',
    ingredients: [
      'Preço total, custo, entrada e médio do item da nota',
      'Quantidade, desconto, acréscimo e devolução',
      'ICMS, ICMS-ST, IPI, PIS, COFINS, frete e seguro',
      'Empresa, colaborador e datas da nota',
    ],
    dimensions: ['Empresa', 'Consultor', 'Nota', 'Período'],
    guardrail: 'Valores chegam como texto e devem ser convertidos com decimal controlado.',
  },
  {
    id: 'product-mix',
    title: 'Profundidade e cobertura do mix',
    question: 'Como o catálogo se distribui e onde existem lacunas?',
    outcome:
      'Lê variedade por marca, coleção, material, cor, tipo, pedra, tamanho e atributos técnicos, com cobertura de imagens.',
    sources: ['perola'],
    readiness: 'safe-query',
    ingredients: [
      'Classificação completa do cadastro de Item',
      'Atributos de joias, relógios e acabamentos',
      'Imagem associada e ordem de exibição',
    ],
    dimensions: ['Marca', 'Coleção', 'Tipo', 'Material', 'Cor'],
  },
  {
    id: 'stock-movement',
    title: 'Movimento e cobertura de estoque',
    question: 'Quais itens giram, ficam parados ou correm risco de ruptura?',
    outcome:
      'Cruza movimento de inventário com vendas para estimar giro, dias sem movimento e cobertura.',
    sources: ['perola', 'erp'],
    readiness: 'mapping-gap',
    ingredients: [
      'Movimentos e quantidades por itemSaldoId',
      'Pedidos, quantidades e datas do ERP',
      'Cadastro e atributos do Item',
    ],
    dimensions: ['Item', 'Loja', 'Tipo de movimento', 'Período'],
    guardrail:
      'Falta confirmar a ponte itemSaldoId → Item/SKU; não é seguro atribuir giro ao produto antes disso.',
  },
  {
    id: 'product-profitability',
    title: 'Rentabilidade por produto e atributo',
    question: 'Quais marcas e características vendem com melhor margem?',
    outcome:
      'Combina volume vendido do ERP com custo fiscal e atributos detalhados da Pérola para formar margem por produto.',
    sources: ['perola', 'erp'],
    readiness: 'mapping-gap',
    ingredients: [
      'SKU, produto e quantidade vendida no ERP',
      'Custo e preço da linha fiscal',
      'Marca, coleção, material, cor, tipo e pedras',
    ],
    dimensions: ['SKU', 'Marca', 'Coleção', 'Material', 'Loja'],
    guardrail:
      'Nota Item não trouxe SKU/itemId; precisamos de Item Saldo ou outra chave oficial antes do cruzamento.',
  },
  {
    id: 'fiscal-reconciliation',
    title: 'Conciliação comercial × fiscal',
    question: 'ERP e nota fiscal contam a mesma venda?',
    outcome:
      'Aponta pedidos ausentes, valores divergentes, cancelamentos não refletidos e diferenças de vendedor ou empresa.',
    sources: ['perola', 'erp'],
    readiness: 'mapping-gap',
    ingredients: [
      'Pedido, chave NFe, data e total do ERP',
      'Número, série, data, empresa e total da Nota',
      'Cancelados, devoluções, descontos e acréscimos',
    ],
    dimensions: ['Documento', 'Empresa', 'Loja', 'Consultor', 'Período'],
    guardrail:
      'É necessário validar se orderId/identifier correspondem a numDocumento ou a outra chave da Pérola.',
  },
  {
    id: 'customer-journey',
    title: 'Jornada 360 do cliente',
    question: 'Da entrada na fila até a recompra, onde ganhamos ou perdemos o cliente?',
    outcome:
      'Une origem e motivo da visita, atendimento, venda ERP, nota fiscal e histórico de recompra em uma única jornada.',
    sources: ['perola', 'erp', 'queue'],
    readiness: 'mapping-gap',
    ingredients: [
      'Contato e histórico comercial do cliente no ERP',
      'Identidade fiscal e notas da Pérola',
      'Motivo, origem, campanha, produtos e desfecho da Fila',
    ],
    dimensions: ['Cliente', 'Campanha', 'Loja', 'Consultor', 'Período'],
    guardrail:
      'Priorizar purchaseCode/documento; CPF e telefone só podem ser usados com normalização e proteção de PII.',
  },
  {
    id: 'campaign-profitability',
    title: 'Rentabilidade de campanhas',
    question: 'Qual campanha gera venda real e margem, não apenas atendimento?',
    outcome:
      'Relaciona campanha e origem capturadas na fila com pedido ERP, nota, impostos, custo e margem.',
    sources: ['perola', 'erp', 'queue'],
    readiness: 'mapping-gap',
    ingredients: [
      'Campanha, bônus e origem do atendimento',
      'Pedido, itens e faturamento do ERP',
      'Custo, desconto, imposto e devolução fiscal',
    ],
    dimensions: ['Campanha', 'Origem', 'Loja', 'Produto', 'Consultor'],
  },
]

export const BI_SOURCE_COMPARISON: BiSourceComparison[] = [
  {
    domain: 'Produto',
    perola: 'Atributos detalhados, classificação técnica e imagens.',
    erp: 'SKU, nome, descrição, categorias, preço comercial e fornecedor.',
    queue: 'Produto visto, procurado, fechado e não encontrado.',
  },
  {
    domain: 'Cliente',
    perola: 'Identidade fiscal, nascimento e endereço da nota.',
    erp: 'E-mail, telefone, celular, gênero, tags e data de cadastro.',
    queue: 'Origem, motivo, profissão, contato informado e contexto da visita.',
  },
  {
    domain: 'Venda',
    perola: 'Nota, impostos, descontos, devolução e custos da linha.',
    erp: 'Pedido, pagamento, itens, total e cancelamento separado.',
    queue: 'Desfecho declarado, valor, campanha e código da compra.',
  },
  {
    domain: 'Equipe',
    perola: 'Colaborador associado à nota e ao item.',
    erp: 'Funcionário, loja atribuída, vendas, meta, PA e ticket.',
    queue: 'Consultor autenticado, fila, conversão, espera, duração e qualidade.',
  },
  {
    domain: 'Estoque',
    perola: 'Movimentos por itemSaldoId e preço de compra.',
    erp: 'Não há saldo ou movimento de estoque no contrato atual.',
    queue: 'Sinal de demanda por produtos procurados e não encontrados.',
  },
]

export const ERP_DATA_NOT_OBSERVED_IN_PEROLA = [
  'Telefone, celular e e-mail do cliente',
  'Apelido, gênero, país, complemento, tags e data de cadastro',
  'SKU, nome e descrição comercial do produto',
  'Forma de pagamento do pedido',
  'Base explícita e separada de pedidos cancelados',
  'Valores de exclusão e débito',
  'Vínculo operacional do funcionário com loja e perfil',
  'Métricas prontas de pedidos, faturamento, ticket, PA, meta e comissão',
]

export function biIntelligenceSource(sourceId: BiIntelligenceSourceId) {
  return BI_INTELLIGENCE_SOURCES.find((source) => source.id === sourceId)
}

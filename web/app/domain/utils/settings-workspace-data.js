export const settingsTabs = [
  { id: 'operacao', label: 'Operacao', icon: 'tune' },
  { id: 'modal', label: 'Modal', icon: 'edit_note' },
  { id: 'produtos', label: 'Produtos', icon: 'inventory_2' },
  { id: 'consultores', label: 'Consultores', icon: 'group' },
  { id: 'metas-crm', label: 'Metas CRM', icon: 'flag' },
  { id: 'gamificacao', label: 'Gamificacao', icon: 'emoji_events' },
  { id: 'motivos', label: 'Motivos', icon: 'fact_check' },
  { id: 'cancelamento', label: 'Cancelamento', icon: 'undo' },
  { id: 'parada', label: 'Parada', icon: 'pause' },
  { id: 'pausas', label: 'Pausas', icon: 'pause_circle' },
  { id: 'motivos-perda', label: 'Perdas', icon: 'trending_down' },
  { id: 'motivos-fora-da-vez', label: 'Fora da vez', icon: 'bolt' },
  { id: 'origens', label: 'Origens', icon: 'share_location' },
  { id: 'profissoes', label: 'Profissoes', icon: 'badge' },
  { id: 'alertas', label: 'Alertas', icon: 'notifications_active' },
]

export const hiddenSettingsTabs = new Set(['parada'])

export const fieldSelectionOptions = [
  { value: 'single', label: 'Escolha unica' },
  { value: 'multiple', label: 'Multiplas escolhas' },
]

export const fieldDetailModeOptions = [
  { value: 'off', label: 'Sem descricao' },
  { value: 'shared', label: 'Uma descricao para a selecao' },
  { value: 'per-item', label: 'Uma descricao por opcao' },
]

export const reasonInputModeOptions = [
  { value: 'text', label: 'Texto livre' },
  { value: 'select', label: 'Apenas lista' },
  { value: 'select-with-other', label: 'Lista com Outro' },
]

export const modalFinishFlowOptions = [
  { value: 'legacy', label: 'Modal atual' },
  { value: 'erp-reconciliation', label: 'Modal conciliacao ERP' },
]

export const reasonInputSectionConfigs = {
  cancelamento: {
    addPlaceholder: 'Adicionar novo motivo de cancelamento',
    description:
      'Opcoes exibidas quando o campo estiver configurado como lista ou lista com outro.',
    group: 'cancel-reason',
    itemsKey: 'cancelReasonOptions',
    labelDefault: 'Motivo do cancelamento',
    labelKey: 'cancelReasonLabel',
    modeKey: 'cancelReasonInputMode',
    otherLabelDefault: 'Detalhe do cancelamento',
    otherLabelKey: 'cancelReasonOtherLabel',
    otherPlaceholderDefault: 'Explique por que o atendimento foi cancelado',
    otherPlaceholderKey: 'cancelReasonOtherPlaceholder',
    placeholderDefault: 'Informe ou selecione o motivo do cancelamento',
    placeholderKey: 'cancelReasonPlaceholder',
    testid: 'settings-cancel-reasons',
    text: 'Define como a justificativa aparece quando o atendimento ainda esta dentro da janela de cancelamento.',
    title: 'Campo de cancelamento',
    optionTitle: 'Motivos de cancelamento',
  },
  parada: {
    addPlaceholder: 'Adicionar novo motivo de parada',
    description:
      'Opcoes exibidas quando a parada estiver configurada como lista ou lista com outro.',
    group: 'stop-reason',
    itemsKey: 'stopReasonOptions',
    labelDefault: 'Motivo da parada',
    labelKey: 'stopReasonLabel',
    modeKey: 'stopReasonInputMode',
    otherLabelDefault: 'Detalhe da parada',
    otherLabelKey: 'stopReasonOtherLabel',
    otherPlaceholderDefault: 'Explique por que o atendimento foi parado',
    otherPlaceholderKey: 'stopReasonOtherPlaceholder',
    placeholderDefault: 'Informe ou selecione o motivo da parada',
    placeholderKey: 'stopReasonPlaceholder',
    testid: 'settings-stop-reasons',
    text: 'A parada sempre exige justificativa. Aqui voce escolhe apenas como ela sera coletada e exibida.',
    title: 'Campo de parada',
    optionTitle: 'Motivos de parada',
  },
}

export const optionTabConfigs = {
  motivos: {
    addPlaceholder: 'Adicionar nova opcao',
    description: 'Opcoes exibidas no modal de fechamento.',
    detailDefault: 'shared',
    detailKey: 'visitReasonDetailMode',
    group: 'visit-reason',
    itemsKey: 'visitReasonOptions',
    selectionDefault: 'multiple',
    selectionKey: 'visitReasonSelectionMode',
    testid: 'settings-motivos',
    title: 'Motivo da visita',
  },
  pausas: {
    addPlaceholder: 'Adicionar novo motivo de pausa',
    description: 'Opcoes exibidas ao pausar consultor no painel de operacao.',
    group: 'pause-reason',
    itemsKey: 'pauseReasonOptions',
    testid: 'settings-pausas',
    title: 'Motivos de pausa',
  },
  'motivos-fora-da-vez': {
    addPlaceholder: 'Adicionar novo motivo fora da vez',
    description: 'Opcoes obrigatorias exibidas quando o atendimento for encerrado fora da vez.',
    group: 'queue-jump-reason',
    itemsKey: 'queueJumpReasonOptions',
    testid: 'settings-fora-da-vez',
    title: 'Motivo fora da vez',
  },
  'motivos-perda': {
    addPlaceholder: 'Adicionar novo motivo da perda',
    description: 'Opcoes exibidas quando o atendimento termina sem venda.',
    detailDefault: 'off',
    detailKey: 'lossReasonDetailMode',
    group: 'loss-reason',
    itemsKey: 'lossReasonOptions',
    selectionDefault: 'single',
    selectionKey: 'lossReasonSelectionMode',
    testid: 'settings-motivos-perda',
    title: 'Motivo da perda',
  },
  origens: {
    addPlaceholder: 'Adicionar nova opcao',
    description: 'Opcoes exibidas no modal de fechamento.',
    detailDefault: 'shared',
    detailKey: 'customerSourceDetailMode',
    group: 'customer-source',
    itemsKey: 'customerSourceOptions',
    selectionDefault: 'single',
    selectionKey: 'customerSourceSelectionMode',
    testid: 'settings-origens',
    title: 'Origem do cliente',
  },
  profissoes: {
    addPlaceholder: 'Adicionar nova profissao',
    description:
      'Lista usada no modal. Se nao existir, tambem pode cadastrar na hora no fechamento.',
    group: 'profession',
    itemsKey: 'professionOptions',
    testid: 'settings-profissoes',
    title: 'Profissoes',
  },
}

function withFieldJustification(field, baseKey) {
  const normalizedBaseKey = String(baseKey || '').trim()
  if (!normalizedBaseKey) return field
  const configPrefix = `${normalizedBaseKey.charAt(0).toLowerCase()}${normalizedBaseKey.slice(1)}`

  return {
    ...field,
    justificationMinCharsKey: `${configPrefix}JustificationMinChars`,
    justificationRequiredKey: `require${normalizedBaseKey}Justification`,
  }
}

export const modalFieldSections = [
  {
    id: 'customer',
    title: 'Dados do cliente',
    description: 'Campos basicos do passo 2 para identificar e qualificar o cliente.',
    defaultOpen: true,
    fields: [
      withFieldJustification(
        {
          id: 'customer-name',
          label: 'Nome do cliente',
          labelKey: 'customerNameLabel',
          description: 'Campo de texto exibido no topo da secao de cliente.',
          showKey: 'showCustomerNameField',
          requiredKey: 'requireCustomerNameField',
          requiredDefault: true,
          legacyRequiredKey: 'requireCustomerNamePhone',
        },
        'CustomerName',
      ),
      withFieldJustification(
        {
          id: 'customer-phone',
          label: 'Telefone',
          labelKey: 'customerPhoneLabel',
          description: 'Usado para contato e reaproveitamento do atendimento.',
          showKey: 'showCustomerPhoneField',
          requiredKey: 'requireCustomerPhoneField',
          requiredDefault: true,
          legacyRequiredKey: 'requireCustomerNamePhone',
        },
        'CustomerPhone',
      ),
      withFieldJustification(
        {
          id: 'customer-email',
          label: 'E-mail',
          labelKey: 'customerEmailLabel',
          description: 'Captura complementar para relacionamento.',
          showKey: 'showEmailField',
          requiredKey: 'requireEmailField',
          requiredDefault: false,
        },
        'Email',
      ),
      withFieldJustification(
        {
          id: 'customer-profession',
          label: 'Profissao',
          labelKey: 'customerProfessionLabel',
          description: 'Usa o catalogo configurado na aba de profissoes.',
          showKey: 'showProfessionField',
          requiredKey: 'requireProfessionField',
          requiredDefault: false,
        },
        'Profession',
      ),
      withFieldJustification(
        {
          id: 'existing-customer',
          label: 'Ja era cliente',
          labelKey: 'existingCustomerLabel',
          description: 'Vai para o passo 2 para apoiar a busca automatica de cadastro do cliente.',
          showKey: 'showExistingCustomerField',
        },
        'ExistingCustomer',
      ),
      withFieldJustification(
        {
          id: 'notes',
          label: 'Observacoes',
          labelKey: 'notesLabel',
          description: 'Campo livre para contexto adicional do atendimento.',
          showKey: 'showNotesField',
          requiredKey: 'requireNotesField',
          requiredDefault: false,
        },
        'Notes',
      ),
    ],
  },
  {
    id: 'journey',
    title: 'Produtos e jornada',
    description: 'Campos principais do atendimento e da origem do cliente.',
    defaultOpen: true,
    fields: [
      withFieldJustification(
        {
          id: 'product-closed',
          label: 'Compra / Reserva',
          labelKey: 'productClosedLabel',
          description: 'Aparece primeiro no passo 1 quando o desfecho for compra ou reserva.',
          showKey: 'showProductClosedField',
          requiredKey: 'requireProductClosedField',
          requiredDefault: true,
          legacyRequiredKey: 'requireProduct',
        },
        'ProductClosed',
      ),
      withFieldJustification(
        {
          id: 'purchase-code',
          label: 'Codigo da compra',
          labelKey: 'purchaseCodeLabel',
          description:
            'No fluxo ERP, aparece apenas para compra e guarda a referencia para conciliacao no dia seguinte.',
          showKey: 'showPurchaseCodeField',
          requiredKey: 'requirePurchaseCodeField',
          requiredDefault: true,
        },
        'PurchaseCode',
      ),
      withFieldJustification(
        {
          id: 'product-seen',
          label: 'Interesses do cliente',
          labelKey: 'productSeenLabel',
          description: 'Aparece no passo 1 para mapear os interesses vistos ou desejados.',
          showKey: 'showProductSeenField',
          requiredKey: 'requireProductSeenField',
          requiredDefault: true,
          legacyRequiredKey: 'requireProduct',
        },
        'ProductSeen',
      ),
      withFieldJustification(
        {
          id: 'product-seen-notes',
          label: 'Observacao dos interesses',
          labelKey: 'productSeenNotesLabel',
          description:
            'Campo complementar para contexto, referencia ou justificativa quando nao houver item.',
          showKey: 'showProductSeenNotesField',
          requiredKey: 'requireProductSeenNotesField',
          requiredDefault: false,
        },
        'ProductSeenNotes',
      ),
      withFieldJustification(
        {
          id: 'visit-reason',
          label: 'Motivo da visita',
          labelKey: 'visitReasonLabel',
          description: 'Ajuda a entender a intencao do cliente na chegada.',
          showKey: 'showVisitReasonField',
          requiredKey: 'requireVisitReason',
          requiredDefault: true,
        },
        'VisitReason',
      ),
      withFieldJustification(
        {
          id: 'customer-source',
          label: 'Origem do cliente',
          labelKey: 'customerSourceLabel',
          description: 'Relaciona o atendimento ao canal de entrada.',
          showKey: 'showCustomerSourceField',
          requiredKey: 'requireCustomerSource',
          requiredDefault: true,
        },
        'CustomerSource',
      ),
    ],
  },
  {
    id: 'conditional',
    title: 'Campos condicionais',
    description: 'Campos que so entram em cenarios especificos de encerramento.',
    defaultOpen: false,
    fields: [
      withFieldJustification(
        {
          id: 'queue-jump-reason',
          label: 'Motivo fora da vez',
          labelKey: 'queueJumpReasonLabel',
          description: 'Exibido quando o atendimento comeca fora da fila.',
          showKey: 'showQueueJumpReasonField',
          requiredKey: 'requireQueueJumpReasonField',
          requiredDefault: true,
        },
        'QueueJumpReason',
      ),
      withFieldJustification(
        {
          id: 'loss-reason',
          label: 'Motivo da perda',
          labelKey: 'lossReasonLabel',
          description: 'Exibido quando o desfecho for nao compra.',
          showKey: 'showLossReasonField',
          requiredKey: 'requireLossReasonField',
          requiredDefault: true,
        },
        'LossReason',
      ),
    ],
  },
]

export const modalTextSections = [
  {
    id: 'general',
    title: 'Textos gerais',
    description: 'Titulos base do modal e da secao de cliente.',
    defaultOpen: true,
    fields: [
      { key: 'title', label: 'Titulo do modal' },
      { key: 'customerSectionLabel', label: 'Label da secao de cliente' },
    ],
  },
  {
    id: 'products',
    title: 'Textos de produto',
    description: 'Copys exibidas nos blocos de produto visto e produto fechado.',
    defaultOpen: false,
    fields: [
      { key: 'productSeenLabel', label: 'Label interesses do cliente' },
      { key: 'productSeenPlaceholder', label: 'Placeholder interesses do cliente' },
      { key: 'productClosedLabel', label: 'Label fechamento (opcional)' },
      { key: 'productClosedPlaceholder', label: 'Placeholder compra / reserva' },
      { key: 'purchaseCodePlaceholder', label: 'Placeholder codigo da compra' },
    ],
  },
  {
    id: 'support',
    title: 'Textos de apoio',
    description: 'Textos auxiliares de observacoes, perda e fora da vez.',
    defaultOpen: false,
    fields: [
      { key: 'notesLabel', label: 'Label observacoes' },
      { key: 'notesPlaceholder', label: 'Placeholder observacoes' },
      { key: 'queueJumpReasonLabel', label: 'Label motivo fora da vez' },
      { key: 'queueJumpReasonPlaceholder', label: 'Placeholder motivo fora da vez' },
      { key: 'lossReasonLabel', label: 'Label motivo da perda' },
      { key: 'lossReasonPlaceholder', label: 'Placeholder motivo da perda' },
      { key: 'cancelReasonLabel', label: 'Label motivo do cancelamento' },
      { key: 'cancelReasonPlaceholder', label: 'Placeholder motivo do cancelamento' },
      { key: 'stopReasonLabel', label: 'Label motivo da parada' },
      { key: 'stopReasonPlaceholder', label: 'Placeholder motivo da parada' },
    ],
  },
]

import { cloneValue } from "../utils/object";

export const DEFAULT_OPERATION_TEMPLATE_ID = "joalheria-padrao";

export const DEFAULT_FIELD_JUSTIFICATION_CONFIG = {
  requireCustomerNameJustification: false,
  customerNameJustificationMinChars: 20,
  requireCustomerPhoneJustification: false,
  customerPhoneJustificationMinChars: 20,
  requireEmailJustification: false,
  emailJustificationMinChars: 20,
  requireProfessionJustification: false,
  professionJustificationMinChars: 20,
  requireExistingCustomerJustification: false,
  existingCustomerJustificationMinChars: 20,
  requireNotesJustification: false,
  notesJustificationMinChars: 20,
  requireProductSeenJustification: false,
  productSeenJustificationMinChars: 20,
  requireProductSeenNotesJustification: false,
  productSeenNotesJustificationMinChars: 20,
  requireProductClosedJustification: false,
  productClosedJustificationMinChars: 20,
  requirePurchaseCodeJustification: false,
  purchaseCodeJustificationMinChars: 20,
  requireVisitReasonJustification: false,
  visitReasonJustificationMinChars: 20,
  requireCustomerSourceJustification: false,
  customerSourceJustificationMinChars: 20,
  requireQueueJumpReasonJustification: false,
  queueJumpReasonJustificationMinChars: 20,
  requireLossReasonJustification: false,
  lossReasonJustificationMinChars: 20
};

const BASE_CUSTOMER_SOURCES = [
  { id: "instagram", label: "Instagram" },
  { id: "trafego-pago", label: "Trafego pago" },
  { id: "google", label: "Google" },
  { id: "whatsapp", label: "WhatsApp" },
  { id: "site", label: "Site" },
  { id: "indicacao", label: "Indicacao de amigo" },
  { id: "cliente-recorrente", label: "Cliente recorrente" },
  { id: "vitrine", label: "Vitrine ou passagem na frente" },
  { id: "evento-parceria", label: "Evento ou parceria" },
  { id: "outro", label: "Outro" }
];

export const DEFAULT_QUEUE_JUMP_REASON_OPTIONS = [
  { id: "cliente-fixo", label: "Cliente fixo" },
  { id: "troca", label: "Troca" },
  { id: "retirada", label: "Retirada" },
  { id: "cliente-chamado-consultor", label: "Cliente chamado pelo consultor" },
  { id: "atendimento-agendado", label: "Atendimento agendado" }
];

export const DEFAULT_PAUSE_REASON_OPTIONS = [
  { id: "almoco", label: "Almoco" },
  { id: "atendimento-externo", label: "Atendimento externo" },
  { id: "suporte-interno", label: "Suporte interno" },
  { id: "treinamento", label: "Treinamento" },
  { id: "reuniao", label: "Reuniao" }
];

export const DEFAULT_LOSS_REASON_OPTIONS = [
  { id: "preco", label: "Preco" },
  { id: "vai-pensar", label: "Vai pensar" },
  { id: "nao-encontrou-o-que-queria", label: "Nao encontrou o que queria" },
  { id: "tamanho-indisponivel", label: "Tamanho indisponivel" },
  { id: "comparando-precos", label: "Comparando precos" },
  { id: "volta-depois", label: "Volta depois" },
  { id: "so-pesquisando", label: "So pesquisando" }
];

export const operationTemplates = [
  {
    id: "joalheria-padrao",
    label: "Joalheria padrao",
    description: "Equilibrio entre qualidade de atendimento, captura de lead e disciplina de fila.",
    settings: {
      maxConcurrentServices: 10,
      maxConcurrentServicesPerConsultant: 1,
      serviceCancelWindowSeconds: 30,
      timingFastCloseMinutes: 5,
      timingLongServiceMinutes: 25,
      timingLowSaleAmount: 1200
    },
    modalConfig: {
      finishFlowMode: "legacy",
      ...DEFAULT_FIELD_JUSTIFICATION_CONFIG,
      showCustomerNameField: true,
      showCustomerPhoneField: true,
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      showProductSeenField: true,
      showProductSeenNotesField: true,
      showProductClosedField: true,
      showPurchaseCodeField: true,
      showVisitReasonField: true,
      showCustomerSourceField: true,
      showExistingCustomerField: true,
      showQueueJumpReasonField: true,
      showLossReasonField: true,
      showCancelReasonField: true,
      showStopReasonField: true,
      allowProductSeenNone: true,
      visitReasonSelectionMode: "multiple",
      visitReasonDetailMode: "shared",
      customerSourceSelectionMode: "single",
      customerSourceDetailMode: "shared",
      cancelReasonInputMode: "text",
      stopReasonInputMode: "text",
      requireCustomerNameField: true,
      requireCustomerPhoneField: true,
      requireEmailField: false,
      requireProfessionField: false,
      requireNotesField: false,
      requireProduct: true,
      requireProductSeenField: true,
      requireProductSeenNotesField: false,
      requireProductClosedField: true,
      requirePurchaseCodeField: true,
      requireVisitReason: true,
      requireCustomerSource: true,
      requireCustomerNamePhone: true,
      requireProductSeenNotesWhenNone: true,
      productSeenNotesMinChars: 20,
      requireQueueJumpReasonField: true,
      requireLossReasonField: true,
      requireCancelReasonField: false,
      requireStopReasonField: false
    },
    visitReasonOptions: [
      { id: "aniversario-casamento", label: "Aniversario de casamento" },
      { id: "pedido-noivado", label: "Pedido de namoro ou noivado" },
      { id: "casamento", label: "Casamento" },
      { id: "aniversario", label: "Aniversario" },
      { id: "quinze-anos", label: "15 anos" },
      { id: "formatura", label: "Formatura" },
      { id: "evento", label: "Evento especial" },
      { id: "promocao", label: "Promocao ou conquista" },
      { id: "presente", label: "Presente" },
      { id: "auto-presente", label: "Auto presente" },
      { id: "retirada", label: "Retirada de reserva" },
      { id: "consulta", label: "Consulta ou pesquisa de preco" },
      { id: "data-especial", label: "Outra data especial" }
    ],
    customerSourceOptions: BASE_CUSTOMER_SOURCES
  },
  {
    id: "joalheria-relacionamento",
    label: "Joalheria relacionamento",
    description: "Mais foco em relacao de longo prazo e coleta completa de dados do cliente.",
    settings: {
      maxConcurrentServices: 8,
      maxConcurrentServicesPerConsultant: 1,
      serviceCancelWindowSeconds: 30,
      timingFastCloseMinutes: 7,
      timingLongServiceMinutes: 35,
      timingLowSaleAmount: 1500
    },
    modalConfig: {
      finishFlowMode: "legacy",
      ...DEFAULT_FIELD_JUSTIFICATION_CONFIG,
      showCustomerNameField: true,
      showCustomerPhoneField: true,
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      showProductSeenField: true,
      showProductSeenNotesField: true,
      showProductClosedField: true,
      showPurchaseCodeField: true,
      showVisitReasonField: true,
      showCustomerSourceField: true,
      showExistingCustomerField: true,
      showQueueJumpReasonField: true,
      showLossReasonField: true,
      showCancelReasonField: true,
      showStopReasonField: true,
      allowProductSeenNone: true,
      visitReasonSelectionMode: "multiple",
      visitReasonDetailMode: "shared",
      customerSourceSelectionMode: "single",
      customerSourceDetailMode: "shared",
      cancelReasonInputMode: "text",
      stopReasonInputMode: "text",
      requireCustomerNameField: true,
      requireCustomerPhoneField: true,
      requireEmailField: false,
      requireProfessionField: false,
      requireNotesField: false,
      requireProduct: true,
      requireProductSeenField: true,
      requireProductSeenNotesField: false,
      requireProductClosedField: true,
      requirePurchaseCodeField: true,
      requireVisitReason: true,
      requireCustomerSource: true,
      requireCustomerNamePhone: true,
      requireProductSeenNotesWhenNone: true,
      productSeenNotesMinChars: 20,
      requireQueueJumpReasonField: true,
      requireLossReasonField: true,
      requireCancelReasonField: false,
      requireStopReasonField: false
    },
    visitReasonOptions: [
      { id: "aniversario-casamento", label: "Aniversario de casamento" },
      { id: "noivado", label: "Noivado" },
      { id: "casamento", label: "Casamento" },
      { id: "presente", label: "Presente" },
      { id: "evento-corporativo", label: "Evento corporativo" },
      { id: "cliente-recorrente", label: "Relacionamento com cliente recorrente" },
      { id: "retirada", label: "Retirada de reserva" },
      { id: "consulta", label: "Consulta ou pesquisa de preco" },
      { id: "data-especial", label: "Outra data especial" }
    ],
    customerSourceOptions: BASE_CUSTOMER_SOURCES
  },
  {
    id: "joalheria-fluxo-rapido",
    label: "Joalheria fluxo rapido",
    description: "Operacao de alto fluxo com fechamento mais objetivo e formulario mais leve.",
    settings: {
      maxConcurrentServices: 12,
      maxConcurrentServicesPerConsultant: 1,
      serviceCancelWindowSeconds: 30,
      timingFastCloseMinutes: 3,
      timingLongServiceMinutes: 18,
      timingLowSaleAmount: 900
    },
    modalConfig: {
      finishFlowMode: "legacy",
      ...DEFAULT_FIELD_JUSTIFICATION_CONFIG,
      showCustomerNameField: true,
      showCustomerPhoneField: true,
      showEmailField: false,
      showProfessionField: false,
      showNotesField: false,
      showProductSeenField: true,
      showProductSeenNotesField: true,
      showProductClosedField: true,
      showPurchaseCodeField: true,
      showVisitReasonField: true,
      showCustomerSourceField: true,
      showExistingCustomerField: true,
      showQueueJumpReasonField: true,
      showLossReasonField: true,
      showCancelReasonField: true,
      showStopReasonField: true,
      allowProductSeenNone: true,
      visitReasonSelectionMode: "multiple",
      visitReasonDetailMode: "off",
      customerSourceSelectionMode: "single",
      customerSourceDetailMode: "off",
      cancelReasonInputMode: "text",
      stopReasonInputMode: "text",
      requireCustomerNameField: true,
      requireCustomerPhoneField: true,
      requireEmailField: false,
      requireProfessionField: false,
      requireNotesField: false,
      requireProduct: true,
      requireProductSeenField: true,
      requireProductSeenNotesField: false,
      requireProductClosedField: true,
      requirePurchaseCodeField: true,
      requireVisitReason: true,
      requireCustomerSource: false,
      requireCustomerNamePhone: true,
      requireProductSeenNotesWhenNone: true,
      productSeenNotesMinChars: 20,
      requireQueueJumpReasonField: true,
      requireLossReasonField: true,
      requireCancelReasonField: false,
      requireStopReasonField: false
    },
    visitReasonOptions: [
      { id: "presente", label: "Presente" },
      { id: "auto-presente", label: "Auto presente" },
      { id: "promocao", label: "Promocao ou conquista" },
      { id: "aniversario", label: "Aniversario" },
      { id: "troca", label: "Troca" },
      { id: "retirada", label: "Retirada de reserva" },
      { id: "consulta", label: "Consulta ou pesquisa de preco" }
    ],
    customerSourceOptions: BASE_CUSTOMER_SOURCES
  }
];

export function getOperationTemplateById(templateId) {
  return operationTemplates.find((template) => template.id === templateId) || null;
}

export function cloneOperationTemplates() {
  return cloneValue(operationTemplates);
}

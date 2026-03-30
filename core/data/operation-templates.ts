import { cloneValue } from "../utils/object";

export const DEFAULT_OPERATION_TEMPLATE_ID = "joalheria-padrao";

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

export const operationTemplates = [
  {
    id: "joalheria-padrao",
    label: "Joalheria padrao",
    description: "Equilibrio entre qualidade de atendimento, captura de lead e disciplina de fila.",
    settings: {
      maxConcurrentServices: 10,
      timingFastCloseMinutes: 5,
      timingLongServiceMinutes: 25,
      timingLowSaleAmount: 1200
    },
    modalConfig: {
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      showVisitReasonDetails: true,
      showCustomerSourceDetails: true,
      requireProduct: true,
      requireVisitReason: true,
      requireCustomerSource: true,
      requireCustomerNamePhone: true
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
      { id: "retirada", label: "Retirada de reserva", outcomes: ["compra", "nao-compra"] },
      { id: "consulta", label: "Consulta ou pesquisa de preco", outcomes: ["nao-compra"] },
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
      timingFastCloseMinutes: 7,
      timingLongServiceMinutes: 35,
      timingLowSaleAmount: 1500
    },
    modalConfig: {
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      showVisitReasonDetails: true,
      showCustomerSourceDetails: true,
      requireProduct: true,
      requireVisitReason: true,
      requireCustomerSource: true,
      requireCustomerNamePhone: true
    },
    visitReasonOptions: [
      { id: "aniversario-casamento", label: "Aniversario de casamento" },
      { id: "noivado", label: "Noivado" },
      { id: "casamento", label: "Casamento" },
      { id: "presente", label: "Presente" },
      { id: "evento-corporativo", label: "Evento corporativo" },
      { id: "cliente-recorrente", label: "Relacionamento com cliente recorrente" },
      { id: "retirada", label: "Retirada de reserva", outcomes: ["compra", "nao-compra"] },
      { id: "consulta", label: "Consulta ou pesquisa de preco", outcomes: ["nao-compra"] },
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
      timingFastCloseMinutes: 3,
      timingLongServiceMinutes: 18,
      timingLowSaleAmount: 900
    },
    modalConfig: {
      showEmailField: false,
      showProfessionField: false,
      showNotesField: false,
      showVisitReasonDetails: false,
      showCustomerSourceDetails: false,
      requireProduct: true,
      requireVisitReason: true,
      requireCustomerSource: false,
      requireCustomerNamePhone: true
    },
    visitReasonOptions: [
      { id: "presente", label: "Presente" },
      { id: "auto-presente", label: "Auto presente" },
      { id: "promocao", label: "Promocao ou conquista" },
      { id: "aniversario", label: "Aniversario" },
      { id: "troca", label: "Troca", outcomes: ["compra", "nao-compra"] },
      { id: "retirada", label: "Retirada de reserva", outcomes: ["compra", "nao-compra"] },
      { id: "consulta", label: "Consulta ou pesquisa de preco", outcomes: ["nao-compra"] }
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

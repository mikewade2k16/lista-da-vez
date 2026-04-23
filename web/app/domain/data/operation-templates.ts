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

export const DEFAULT_QUEUE_JUMP_REASON_OPTIONS = [
  { id: "cliente-fixo", label: "Cliente fixo" },
  { id: "troca", label: "Troca" },
  { id: "retirada", label: "Retirada" },
  { id: "cliente-chamado-consultor", label: "Cliente chamado pelo consultor" },
  { id: "atendimento-agendado", label: "Atendimento agendado" }
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
      timingFastCloseMinutes: 5,
      timingLongServiceMinutes: 25,
      timingLowSaleAmount: 1200
    },
    modalConfig: {
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      visitReasonSelectionMode: "multiple",
      visitReasonDetailMode: "shared",
      customerSourceSelectionMode: "single",
      customerSourceDetailMode: "shared",
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
      timingFastCloseMinutes: 7,
      timingLongServiceMinutes: 35,
      timingLowSaleAmount: 1500
    },
    modalConfig: {
      showEmailField: true,
      showProfessionField: true,
      showNotesField: true,
      visitReasonSelectionMode: "multiple",
      visitReasonDetailMode: "shared",
      customerSourceSelectionMode: "single",
      customerSourceDetailMode: "shared",
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
      timingFastCloseMinutes: 3,
      timingLongServiceMinutes: 18,
      timingLowSaleAmount: 900
    },
    modalConfig: {
      showEmailField: false,
      showProfessionField: false,
      showNotesField: false,
      visitReasonSelectionMode: "multiple",
      visitReasonDetailMode: "off",
      customerSourceSelectionMode: "single",
      customerSourceDetailMode: "off",
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

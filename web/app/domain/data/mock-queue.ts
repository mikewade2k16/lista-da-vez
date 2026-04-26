import {
  cloneOperationTemplates,
  DEFAULT_LOSS_REASON_OPTIONS,
  DEFAULT_PAUSE_REASON_OPTIONS,
  DEFAULT_QUEUE_JUMP_REASON_OPTIONS,
  DEFAULT_OPERATION_TEMPLATE_ID,
  getOperationTemplateById
} from "./operation-templates";
import { DEFAULT_PROFESSION_OPTIONS } from "./profession-options";
import { normalizeCampaign } from "../utils/campaigns";
import { DEFAULT_REPORT_FILTERS } from "../utils/reports";

const defaultTemplate = getOperationTemplateById(DEFAULT_OPERATION_TEMPLATE_ID);

function buildConsultant(id, name, color, monthlyGoal, commissionRate) {
  const parts = String(name || "").trim().split(/\s+/).filter(Boolean);
  const first = parts[0]?.charAt(0) || "C";
  const second = parts[1]?.charAt(0) || parts[0]?.charAt(1) || "O";

  return {
    id,
    name,
    role: "Atendimento",
    initials: `${first}${second}`.toUpperCase(),
    color,
    monthlyGoal,
    commissionRate
  };
}

function createStoreSnapshot(roster) {
  const now = Date.now();
  const consultantCurrentStatus = Object.fromEntries(
    roster.map((consultant) => [
      consultant.id,
      {
        status: "available",
        startedAt: now
      }
    ])
  );

  return {
    selectedConsultantId: roster[0]?.id || null,
    consultantSimulationAdditionalSales: 0,
    waitingList: [],
    activeServices: [],
    roster,
    consultantActivitySessions: [],
    consultantCurrentStatus,
    pausedEmployees: [],
    serviceHistory: []
  };
}

const stores = [
  { id: "loja-pj-riomar", name: "Pérola Riomar", code: "PJ-RIO", city: "Aracaju" },
  { id: "loja-pj-jardins", name: "Pérola Jardins", code: "PJ-JAR", city: "Aracaju" },
  { id: "loja-pj-treze", name: "Pérola Treze", code: "PJ-TRE", city: "Aracaju" },
  { id: "loja-pj-garcia", name: "Pérola Garcia", code: "PJ-GAR", city: "Aracaju" }
];
const snapshots = {
  "loja-pj-riomar": createStoreSnapshot([
    buildConsultant("thalles", "Thalles", "#168aad", 140000, 0.025),
    buildConsultant("erik", "Erik", "#7a6ff0", 180000, 0.03),
    buildConsultant("camila", "Camila", "#d17a96", 150000, 0.028),
    buildConsultant("mariana", "Mariana", "#e09f3e", 130000, 0.024),
    buildConsultant("hiro", "Hiro", "#355070", 165000, 0.029),
    buildConsultant("nathalia", "Nathalia", "#d90429", 155000, 0.027)
  ]),
  "loja-pj-jardins": createStoreSnapshot([
    buildConsultant("aline", "Aline", "#168aad", 120000, 0.024),
    buildConsultant("joao", "Joao", "#7a6ff0", 115000, 0.022),
    buildConsultant("bruna", "Bruna", "#d17a96", 135000, 0.026),
    buildConsultant("leo", "Leo", "#e09f3e", 128000, 0.025)
  ]),
  "loja-pj-treze": createStoreSnapshot([
    buildConsultant("rafa", "Rafaela", "#355070", 210000, 0.032),
    buildConsultant("caio", "Caio", "#d90429", 195000, 0.03),
    buildConsultant("patricia", "Patricia", "#23a26d", 205000, 0.031),
    buildConsultant("vitor", "Vitor", "#4361ee", 188000, 0.029)
  ]),
  "loja-pj-garcia": createStoreSnapshot([
    buildConsultant("marcia", "Marcia", "#355070", 210000, 0.032),
    buildConsultant("amanda", "Amanda", "#d90429", 195000, 0.03),
    buildConsultant("Joana", "Joana", "#23a26d", 205000, 0.031),
    buildConsultant("vitoria", "Vitoria", "#4361ee", 188000, 0.029)
  ])
};
const activeStoreId = stores[0].id;
const activeSnapshot = snapshots[activeStoreId];

export const mockQueueState = {
  configSchemaVersion: 4,
  brandName: "Omni",
  pageTitle: "Fila de atendimento",
  profiles: [
    { id: "perfil-platform-admin", name: "Admin Plataforma", role: "platform_admin" },
    { id: "perfil-proprietario", name: "Proprietario Grupo", role: "owner" },
    { id: "perfil-marketing", name: "Marketing Grupo", role: "marketing" },
    { id: "perfil-gerente", name: "Gerente Loja", role: "manager" },
    { id: "perfil-consultor", name: "Consultor Loja", role: "consultant" }
  ],
  activeProfileId: "perfil-platform-admin",
  stores,
  activeStoreId,
  storeSnapshots: snapshots,
  activeWorkspace: "operacao",
  selectedConsultantId: activeSnapshot.selectedConsultantId,
  consultantSimulationAdditionalSales: activeSnapshot.consultantSimulationAdditionalSales,
  operationTemplates: cloneOperationTemplates(),
  selectedOperationTemplateId: DEFAULT_OPERATION_TEMPLATE_ID,
  reportFilters: { ...DEFAULT_REPORT_FILTERS },
  campaigns: [
    normalizeCampaign({
      id: "campanha-prata-instagram",
      name: "Prata Instagram",
      description: "Campanha comercial do grupo para prata com foco em Instagram e apoio de vitrine/site.",
      campaignType: "comercial",
      startsAt: "2026-03-01",
      endsAt: "2026-04-30",
      targetOutcome: "compra-reserva",
      sourceIds: ["instagram"],
      productCodes: ["COL-PRATA-004", "BRI-PRATA-009", "PUL-PRATA-010"],
      bonusFixed: 0,
      bonusRate: 0,
      isActive: true
    }),
    normalizeCampaign({
      id: "campanha-ticket-premium",
      name: "Ticket premium",
      description: "Bonus para compra ou reserva acima de R$ 5.000.",
      campaignType: "interna",
      targetOutcome: "compra-reserva",
      minSaleAmount: 5000,
      bonusFixed: 50,
      bonusRate: 0.005,
      isActive: true
    }),
    normalizeCampaign({
      id: "campanha-recuperacao-fora-da-vez",
      name: "Recuperacao fora da vez",
      description: "Premia atendimento fora da vez que converte cliente novo.",
      campaignType: "interna",
      targetOutcome: "compra-reserva",
      queueJumpOnly: true,
      existingCustomerFilter: "no",
      bonusFixed: 40,
      bonusRate: 0.003,
      isActive: true
    })
  ],
  waitingList: activeSnapshot.waitingList,
  activeServices: activeSnapshot.activeServices,
  finishModalDraft: null,
  visitReasonOptions: (defaultTemplate?.visitReasonOptions || []).map((item) => ({ ...item })),
  customerSourceOptions: (defaultTemplate?.customerSourceOptions || []).map((item) => ({ ...item })),
  pauseReasonOptions: DEFAULT_PAUSE_REASON_OPTIONS.map((item) => ({ ...item })),
  queueJumpReasonOptions: DEFAULT_QUEUE_JUMP_REASON_OPTIONS.map((item) => ({ ...item })),
  lossReasonOptions: DEFAULT_LOSS_REASON_OPTIONS.map((item) => ({ ...item })),
  professionOptions: DEFAULT_PROFESSION_OPTIONS.map((item) => ({ ...item })),
  productCatalog: [
    { id: "produto-1", name: "Anel Solitario Ouro 18k", code: "ANE-OURO-001", category: "Aneis", basePrice: 3900 },
    { id: "produto-2", name: "Alianca Slim Diamantada", code: "ALI-OURO-002", category: "Aliancas", basePrice: 2200 },
    { id: "produto-3", name: "Brinco Gota Safira", code: "BRI-PEDRA-003", category: "Brincos", basePrice: 1750 },
    { id: "produto-4", name: "Colar Riviera Prata", code: "COL-PRATA-004", category: "Colares", basePrice: 1480 },
    { id: "produto-5", name: "Pulseira Cartier Ouro", code: "PUL-OURO-005", category: "Pulseiras", basePrice: 2850 },
    { id: "produto-6", name: "Relogio Classico Feminino", code: "REL-CLASS-006", category: "Relogios", basePrice: 4200 },
    { id: "produto-7", name: "Anel Formatura Esmeralda", code: "ANE-FORM-007", category: "Aneis", basePrice: 2600 },
    { id: "produto-8", name: "Escapulario Ouro Branco", code: "COL-OURO-008", category: "Colares", basePrice: 1950 },
    { id: "produto-9", name: "Brinco Argola Premium", code: "BRI-PRATA-009", category: "Brincos", basePrice: 1320 },
    { id: "produto-10", name: "Pulseira Tennis Zirconia", code: "PUL-PRATA-010", category: "Pulseiras", basePrice: 1680 }
  ],
  modalConfig: {
    title: "Fechar atendimento",
    productSeenLabel: "Interesses do cliente",
    productSeenPlaceholder: "Busque e selecione interesses",
    productClosedLabel: "",
    productClosedPlaceholder: "Busque e selecione o produto fechado",
    notesLabel: "Observações",
    notesPlaceholder: "Detalhes adicionais do atendimento",
    queueJumpReasonLabel: "Motivo do atendimento fora da vez",
    queueJumpReasonPlaceholder: "Busque e selecione o motivo fora da vez",
    lossReasonLabel: "Motivo da perda",
    lossReasonPlaceholder: "Busque e selecione o motivo da perda",
    customerSectionLabel: "Dados do cliente",
    customerNameLabel: "Nome do cliente",
    customerPhoneLabel: "Telefone",
    customerEmailLabel: "E-mail",
    customerProfessionLabel: "Profissão",
    existingCustomerLabel: "Já era cliente",
    productSeenNotesLabel: "Observação dos interesses",
    productSeenNotesPlaceholder: "Descreva referência, pedido específico, contexto do cliente ou justificativa quando não houver interesse identificado.",
    visitReasonLabel: "Motivo da visita",
    customerSourceLabel: "Origem do cliente",
    showCustomerNameField: true,
    showCustomerPhoneField: true,
    showEmailField: true,
    showProfessionField: true,
    showNotesField: true,
    showProductSeenField: true,
    showProductSeenNotesField: true,
    showProductClosedField: true,
    showVisitReasonField: true,
    showCustomerSourceField: true,
    showExistingCustomerField: true,
    showQueueJumpReasonField: true,
    showLossReasonField: true,
    allowProductSeenNone: true,
    visitReasonSelectionMode: "multiple",
    visitReasonDetailMode: "shared",
    lossReasonSelectionMode: "single",
    lossReasonDetailMode: "off",
    customerSourceSelectionMode: "single",
    customerSourceDetailMode: "shared",
    requireCustomerNameField: true,
    requireCustomerPhoneField: true,
    requireEmailField: false,
    requireProfessionField: false,
    requireNotesField: false,
    requireProduct: true,
    requireProductSeenField: true,
    requireProductSeenNotesField: false,
    requireProductClosedField: true,
    requireVisitReason: true,
    requireCustomerSource: true,
    requireCustomerNamePhone: true,
    requireProductSeenNotesWhenNone: true,
    productSeenNotesMinChars: 20,
    requireQueueJumpReasonField: true,
    requireLossReasonField: true
  },
  settings: {
    maxConcurrentServices: Number(defaultTemplate?.settings?.maxConcurrentServices || 10),
    timingFastCloseMinutes: Number(defaultTemplate?.settings?.timingFastCloseMinutes || 5),
    timingLongServiceMinutes: Number(defaultTemplate?.settings?.timingLongServiceMinutes || 25),
    timingLowSaleAmount: Number(defaultTemplate?.settings?.timingLowSaleAmount || 1200),
    testModeEnabled: true,
    autoFillFinishModal: true
  },
  serviceHistory: activeSnapshot.serviceHistory,
  roster: activeSnapshot.roster,
  consultantActivitySessions: activeSnapshot.consultantActivitySessions,
  consultantCurrentStatus: activeSnapshot.consultantCurrentStatus,
  pausedEmployees: activeSnapshot.pausedEmployees
};

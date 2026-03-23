import {
  cloneOperationTemplates,
  DEFAULT_OPERATION_TEMPLATE_ID,
  getOperationTemplateById
} from "./operation-templates.js";
import { DEFAULT_PROFESSION_OPTIONS } from "./profession-options.js";
import { normalizeCampaign } from "../utils/campaigns.js";
import { DEFAULT_REPORT_FILTERS } from "../utils/reports.js";

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
    { id: "perfil-admin", name: "Admin Omni", role: "admin" },
    { id: "perfil-gerente", name: "Gerente Loja", role: "manager" },
    { id: "perfil-consultor", name: "Consultor Loja", role: "consultant" }
  ],
  activeProfileId: "perfil-admin",
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
      id: "campanha-ticket-premium",
      name: "Ticket premium",
      description: "Bonus para compra ou reserva acima de R$ 5.000.",
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
  professionOptions: DEFAULT_PROFESSION_OPTIONS.map((item) => ({ ...item })),
  productCatalog: [
    { id: "produto-1", name: "Anel Solitario Ouro 18k", category: "Aneis", basePrice: 3900 },
    { id: "produto-2", name: "Alianca Slim Diamantada", category: "Aliancas", basePrice: 2200 },
    { id: "produto-3", name: "Brinco Gota Safira", category: "Brincos", basePrice: 1750 },
    { id: "produto-4", name: "Colar Riviera Prata", category: "Colares", basePrice: 1480 },
    { id: "produto-5", name: "Pulseira Cartier Ouro", category: "Pulseiras", basePrice: 2850 },
    { id: "produto-6", name: "Relogio Classico Feminino", category: "Relogios", basePrice: 4200 },
    { id: "produto-7", name: "Anel Formatura Esmeralda", category: "Aneis", basePrice: 2600 },
    { id: "produto-8", name: "Escapulario Ouro Branco", category: "Colares", basePrice: 1950 },
    { id: "produto-9", name: "Brinco Argola Premium", category: "Brincos", basePrice: 1320 },
    { id: "produto-10", name: "Pulseira Tennis Zirconia", category: "Pulseiras", basePrice: 1680 }
  ],
  modalConfig: {
    title: "Fechar atendimento",
    productSeenLabel: "Produto visto pelo cliente",
    productSeenPlaceholder: "Busque e selecione um produto",
    productClosedLabel: "Produto reservado/comprado",
    productClosedPlaceholder: "Busque e selecione o produto fechado",
    notesLabel: "Observacoes",
    notesPlaceholder: "Detalhes adicionais do atendimento",
    queueJumpReasonLabel: "Motivo do atendimento fora da vez",
    queueJumpReasonPlaceholder: "Cliente fixo, troca, retirada, cliente chamado pelo consultor...",
    customerSectionLabel: "Dados do cliente",
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

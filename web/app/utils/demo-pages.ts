const fallbackPage = {
  title: "Pagina simples",
  eyebrow: "Workspace",
  description: "Visao operacional com dados de exemplo para validar a navegacao do sistema.",
  status: "Mockup local",
  statusMeta: "Dados estaticos",
  metrics: [
    { label: "Itens ativos", value: "24", tone: "info" },
    { label: "Pendencias", value: "7", tone: "warning" },
    { label: "Resolvidos", value: "18", tone: "success" }
  ],
  rows: [
    { title: "Revisar fluxo principal", meta: "Hoje, 10:30", status: "Em andamento", tone: "info" },
    { title: "Validar cadastro", meta: "Hoje, 11:15", status: "Pendente", tone: "warning" },
    { title: "Publicar ajuste", meta: "Ontem, 17:40", status: "Concluido", tone: "success" }
  ],
  asideTitle: "Fila da area",
  asideItems: ["Entrada recebida", "Triagem interna", "Retorno agendado"]
};

export const DEMO_PAGES = {
  omnichannel: {
    title: "Omnichannel",
    eyebrow: "Atendimento",
    description: "Central unificada para conversas vindas de WhatsApp, site, loja e canais internos.",
    status: "6 canais ativos",
    statusMeta: "Atualizado ha 4 min",
    metrics: [
      { label: "Conversas abertas", value: "128", tone: "info" },
      { label: "Sem dono", value: "14", tone: "warning" },
      { label: "SLA dentro", value: "92%", tone: "success" }
    ],
    rows: [
      { title: "Lead do showroom aguardando consultor", meta: "WhatsApp - 2 min", status: "Novo", tone: "info" },
      { title: "Cliente retornou sobre proposta", meta: "Site - 8 min", status: "Prioridade", tone: "warning" },
      { title: "Atendimento transferido para loja", meta: "Interno - 16 min", status: "Em rota", tone: "success" }
    ],
    asideTitle: "Canais",
    asideItems: ["WhatsApp", "Chat do site", "Telefone", "Presencial"]
  },
  tasks: {
    title: "Tasks",
    eyebrow: "Produtividade",
    description: "Quadro simples de tarefas para acompanhar responsaveis, prioridades e proximas entregas.",
    status: "Sprint aberta",
    statusMeta: "Ciclo comercial atual",
    metrics: [
      { label: "Para hoje", value: "18", tone: "warning" },
      { label: "Em execucao", value: "9", tone: "info" },
      { label: "Finalizadas", value: "31", tone: "success" }
    ],
    rows: [
      { title: "Conferir fila sem responsavel", meta: "Responsavel: Marina", status: "Alta", tone: "warning" },
      { title: "Atualizar script de abordagem", meta: "Responsavel: Leo", status: "Em execucao", tone: "info" },
      { title: "Fechar retorno de proposta", meta: "Responsavel: Ana", status: "Concluida", tone: "success" }
    ],
    asideTitle: "Prioridades",
    asideItems: ["Sem responsavel", "SLA em risco", "Retorno prometido"]
  },
  tracking: {
    title: "Tracking",
    eyebrow: "Jornada",
    description: "Linha de acompanhamento para leads, atendimentos e etapas comerciais entre loja e backoffice.",
    status: "Pipeline online",
    statusMeta: "3 integracoes simuladas",
    metrics: [
      { label: "Em contato", value: "46", tone: "info" },
      { label: "Atrasados", value: "5", tone: "warning" },
      { label: "Convertidos", value: "17", tone: "success" }
    ],
    rows: [
      { title: "Cliente chegou na etapa de proposta", meta: "Lead #3912", status: "Proposta", tone: "info" },
      { title: "Follow-up excedeu prazo combinado", meta: "Lead #3881", status: "Atrasado", tone: "warning" },
      { title: "Venda confirmada pela unidade", meta: "Lead #3850", status: "Convertido", tone: "success" }
    ],
    asideTitle: "Etapas",
    asideItems: ["Entrada", "Qualificacao", "Proposta", "Fechamento"]
  },
  finance: {
    title: "Finance",
    eyebrow: "Financeiro",
    description: "Resumo de cobrancas, previsoes e conciliacoes conectadas ao funil comercial.",
    status: "Fechamento parcial",
    statusMeta: "Maio/2026",
    metrics: [
      { label: "Previsto", value: "R$ 84k", tone: "info" },
      { label: "Pendente", value: "R$ 12k", tone: "warning" },
      { label: "Conciliado", value: "R$ 61k", tone: "success" }
    ],
    rows: [
      { title: "Comissao aguardando conferencia", meta: "Loja Centro", status: "Pendente", tone: "warning" },
      { title: "Recebimento confirmado", meta: "Pedido #2207", status: "Pago", tone: "success" },
      { title: "Forecast atualizado pelo gerente", meta: "Meta semanal", status: "Revisado", tone: "info" }
    ],
    asideTitle: "Visoes",
    asideItems: ["Comissoes", "Recebimentos", "Forecast", "Conciliacao"]
  },
  monitoramento: {
    title: "Monitoramento",
    eyebrow: "Operacao",
    description: "Painel de saude para filas, integracoes, alertas e eventos importantes do ambiente.",
    status: "Ambiente estavel",
    statusMeta: "Ultima checagem ha 1 min",
    metrics: [
      { label: "Servicos OK", value: "12", tone: "success" },
      { label: "Alertas", value: "3", tone: "warning" },
      { label: "Eventos/h", value: "248", tone: "info" }
    ],
    rows: [
      { title: "Webhook de operacao respondendo", meta: "Latencia 84ms", status: "OK", tone: "success" },
      { title: "Fila com espera acima do normal", meta: "Loja Norte", status: "Atencao", tone: "warning" },
      { title: "Snapshot recebido do backend", meta: "Operacao realtime", status: "Sincronizado", tone: "info" }
    ],
    asideTitle: "Monitores",
    asideItems: ["API", "Realtime", "ERP", "Alertas"]
  },
  "tools/qr-code": {
    title: "QR Code",
    eyebrow: "Tools",
    description: "Gerador de QR Codes para campanhas, check-in de loja e links rapidos de atendimento.",
    status: "Templates prontos",
    statusMeta: "4 modelos",
    metrics: [
      { label: "Gerados hoje", value: "42", tone: "info" },
      { label: "Campanhas", value: "8", tone: "success" },
      { label: "Expirando", value: "2", tone: "warning" }
    ],
    rows: [
      { title: "QR para entrada da fila", meta: "Loja Centro", status: "Publicado", tone: "success" },
      { title: "QR de campanha primavera", meta: "Marketing", status: "Rascunho", tone: "info" },
      { title: "QR antigo aguardando troca", meta: "Loja Sul", status: "Expira hoje", tone: "warning" }
    ],
    asideTitle: "Modelos",
    asideItems: ["Check-in", "Campanha", "Pesquisa", "Atendimento"]
  },
  "tools/encurtador-de-link": {
    title: "Encurtador de Link",
    eyebrow: "Tools",
    description: "Links curtos para campanhas, retornos e acompanhamento de origem dos atendimentos.",
    status: "Domino ativo",
    statusMeta: "fila.link",
    metrics: [
      { label: "Cliques hoje", value: "1.204", tone: "info" },
      { label: "Links ativos", value: "87", tone: "success" },
      { label: "Sem tag", value: "6", tone: "warning" }
    ],
    rows: [
      { title: "fila.link/retorno", meta: "Retorno pos-visita", status: "Ativo", tone: "success" },
      { title: "fila.link/promo-maio", meta: "Campanha mensal", status: "Monitorar", tone: "info" },
      { title: "fila.link/old-catalog", meta: "Catalogo antigo", status: "Revisar", tone: "warning" }
    ],
    asideTitle: "Origens",
    asideItems: ["WhatsApp", "Instagram", "Email", "Site"]
  },
  "tools/scripts": {
    title: "Scripts",
    eyebrow: "Tools",
    description: "Biblioteca de scripts comerciais, respostas rapidas e padroes de abordagem por canal.",
    status: "24 scripts",
    statusMeta: "6 revisados este mes",
    metrics: [
      { label: "Em uso", value: "19", tone: "success" },
      { label: "Rascunhos", value: "5", tone: "info" },
      { label: "Revisao", value: "3", tone: "warning" }
    ],
    rows: [
      { title: "Primeiro contato via WhatsApp", meta: "Canal digital", status: "Ativo", tone: "success" },
      { title: "Retorno de proposta", meta: "Vendas", status: "Em revisao", tone: "warning" },
      { title: "Convite para visita", meta: "Loja fisica", status: "Rascunho", tone: "info" }
    ],
    asideTitle: "Categorias",
    asideItems: ["Abordagem", "Retorno", "Pos-venda", "Objecoes"]
  },
  "team/equipe": {
    title: "Equipe",
    eyebrow: "Team",
    description: "Visao compacta de equipe, papeis, disponibilidade e cobertura por unidade.",
    status: "32 pessoas",
    statusMeta: "5 lojas",
    metrics: [
      { label: "Online", value: "21", tone: "success" },
      { label: "Em pausa", value: "4", tone: "warning" },
      { label: "Off", value: "7", tone: "info" }
    ],
    rows: [
      { title: "Equipe Centro completa", meta: "8 consultores", status: "Coberta", tone: "success" },
      { title: "Loja Norte com baixa cobertura", meta: "2 consultores", status: "Atencao", tone: "warning" },
      { title: "Treinamento de onboarding", meta: "3 convidados", status: "Agendado", tone: "info" }
    ],
    asideTitle: "Recortes",
    asideItems: ["Consultores", "Gerentes", "Marketing", "Backoffice"]
  },
  "team/escalas": {
    title: "Escalas",
    eyebrow: "Team",
    description: "Planejamento de turnos, folgas e reforcos para horarios com maior demanda.",
    status: "Semana aberta",
    statusMeta: "04 a 10 de maio",
    metrics: [
      { label: "Turnos", value: "54", tone: "info" },
      { label: "Lacunas", value: "3", tone: "warning" },
      { label: "Confirmados", value: "47", tone: "success" }
    ],
    rows: [
      { title: "Reforco no sabado a tarde", meta: "Loja Shopping", status: "Pendente", tone: "warning" },
      { title: "Turno extra confirmado", meta: "Loja Centro", status: "Confirmado", tone: "success" },
      { title: "Troca solicitada", meta: "Ana por Bruno", status: "Analise", tone: "info" }
    ],
    asideTitle: "Janelas",
    asideItems: ["Manha", "Tarde", "Noite", "Fim de semana"]
  },
  "site/paginas": {
    title: "Paginas",
    eyebrow: "Site",
    description: "Controle de paginas publicas, links de campanha e estados de publicacao.",
    status: "12 publicadas",
    statusMeta: "2 em revisao",
    metrics: [
      { label: "Publicadas", value: "12", tone: "success" },
      { label: "Rascunhos", value: "4", tone: "info" },
      { label: "Com erro", value: "1", tone: "warning" }
    ],
    rows: [
      { title: "Pagina de campanha mensal", meta: "/maio", status: "Publicada", tone: "success" },
      { title: "Landing de captacao", meta: "/visite-a-loja", status: "Revisao", tone: "info" },
      { title: "Pagina antiga sem analytics", meta: "/catalogo-2025", status: "Ajustar", tone: "warning" }
    ],
    asideTitle: "Tipos",
    asideItems: ["Landing", "Campanha", "Institucional", "Formulario"]
  },
  "site/formularios": {
    title: "Formularios",
    eyebrow: "Site",
    description: "Entradas de leads e pesquisas conectadas aos fluxos de atendimento.",
    status: "7 formularios",
    statusMeta: "3 com automacao",
    metrics: [
      { label: "Envios hoje", value: "86", tone: "info" },
      { label: "Convertidos", value: "23", tone: "success" },
      { label: "Incompletos", value: "9", tone: "warning" }
    ],
    rows: [
      { title: "Agendar visita", meta: "Site principal", status: "Ativo", tone: "success" },
      { title: "Pesquisa pos-atendimento", meta: "NPS", status: "Monitorar", tone: "info" },
      { title: "Interesse em produto", meta: "Campanha antiga", status: "Revisar", tone: "warning" }
    ],
    asideTitle: "Capturas",
    asideItems: ["Lead", "Pesquisa", "Agendamento", "Contato"]
  },
  "manage/auditoria": {
    title: "Auditoria",
    eyebrow: "Manage",
    description: "Eventos administrativos importantes para rastrear ajustes, acessos e alteracoes sensiveis.",
    status: "Log ativo",
    statusMeta: "Retencao de 90 dias",
    metrics: [
      { label: "Eventos hoje", value: "312", tone: "info" },
      { label: "Criticos", value: "2", tone: "warning" },
      { label: "Revisados", value: "44", tone: "success" }
    ],
    rows: [
      { title: "Regra de alerta alterada", meta: "Admin da plataforma", status: "Critico", tone: "warning" },
      { title: "Usuario convidado", meta: "Gestao de usuarios", status: "Registrado", tone: "info" },
      { title: "Configuracao salva", meta: "Tenant atual", status: "Conferido", tone: "success" }
    ],
    asideTitle: "Filtros",
    asideItems: ["Acesso", "Configuracao", "Usuarios", "Alertas"]
  },
  "manage/integracoes": {
    title: "Integracoes",
    eyebrow: "Manage",
    description: "Catalogo de conexoes internas, webhooks e sincronizacoes externas do sistema.",
    status: "9 conectores",
    statusMeta: "7 saudaveis",
    metrics: [
      { label: "Ativas", value: "7", tone: "success" },
      { label: "Fila", value: "143", tone: "info" },
      { label: "Falhas", value: "4", tone: "warning" }
    ],
    rows: [
      { title: "ERP FTP sincronizado", meta: "Ultimo lote processado", status: "OK", tone: "success" },
      { title: "Webhook de campanhas", meta: "Retentativas em fila", status: "Acompanhar", tone: "warning" },
      { title: "Analytics consolidado", meta: "Job horario", status: "Rodando", tone: "info" }
    ],
    asideTitle: "Conectores",
    asideItems: ["ERP", "WhatsApp", "Analytics", "Webhooks"]
  }
};

export function getDemoPage(key) {
  return DEMO_PAGES[String(key || "").trim()] || fallbackPage;
}

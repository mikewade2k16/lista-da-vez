export const SIDEBAR_NAV_SECTIONS = [
  {
    id: "service",
    label: "Atendimento",
    items: [
      { id: "omnichannel", label: "Omnichannel", icon: "messages", path: "/omnichannel" },
      { id: "tasks", label: "Tasks", icon: "tasks", path: "/tasks" },
      { id: "tracking", label: "Tracking", icon: "tracking", path: "/tracking" },
      { id: "fila", label: "Fila", icon: "queue", path: "/operacao", workspaceId: "operacao" }
    ]
  },
  {
    id: "tools",
    label: "Tools",
    items: [
      {
        id: "tools-menu",
        label: "Tools",
        icon: "tools",
        children: [
          { id: "qr-code", label: "QR Code", icon: "qr", path: "/tools/qr-code" },
          { id: "link-shortener", label: "Encurtador de Link", icon: "link", path: "/tools/encurtador-de-link" },
          { id: "scripts", label: "Scripts", icon: "script", path: "/tools/scripts" }
        ]
      }
    ]
  },
  {
    id: "team-site",
    label: "Operacao comercial",
    items: [
      {
        id: "team-menu",
        label: "Team",
        icon: "team",
        children: [
          { id: "consultor", label: "Consultor", icon: "user", path: "/consultor", workspaceId: "consultor" },
          { id: "team-equipe", label: "Equipe", icon: "team", path: "/team/equipe" },
          { id: "team-escalas", label: "Escalas", icon: "calendar", path: "/team/escalas" }
        ]
      },
      {
        id: "site-menu",
        label: "Site",
        icon: "site",
        children: [
          { id: "campanhas", label: "Campanhas", icon: "megaphone", path: "/campanhas", workspaceId: "campanhas" },
          { id: "site-paginas", label: "Paginas", icon: "page", path: "/site/paginas" },
          { id: "site-formularios", label: "Formularios", icon: "forms", path: "/site/formularios" }
        ]
      }
    ]
  },
  {
    id: "indicators",
    label: "Indicadores",
    items: [
      {
        id: "indicadores-menu",
        label: "Indicadores",
        icon: "indicators",
        children: [
          { id: "ranking", label: "Ranking", icon: "ranking", path: "/ranking", workspaceId: "ranking" },
          { id: "dados", label: "Dados", icon: "chart", path: "/dados", workspaceId: "dados" },
          { id: "inteligencia", label: "Inteligencia", icon: "brain", path: "/inteligencia", workspaceId: "inteligencia" },
          { id: "relatorios", label: "Relatorios", icon: "reports", path: "/relatorios", workspaceId: "relatorios" }
        ]
      },
      { id: "finance", label: "Finance", icon: "finance", path: "/finance" },
      { id: "monitoramento", label: "Monitoramento", icon: "monitoring", path: "/monitoramento" }
    ]
  },
  {
    id: "manage",
    label: "Manage",
    items: [
      {
        id: "manage-menu",
        label: "Manage",
        icon: "manage",
        children: [
          { id: "clientes", label: "Clientes", icon: "building", path: "/clientes", workspaceId: "clientes" },
          { id: "usuarios", label: "Usuarios", icon: "users", path: "/usuarios", workspaceId: "usuarios" },
          { id: "erp", label: "ERP", icon: "boxes", path: "/erp", workspaceId: "erp" },
          { id: "multiloja", label: "Multi-loja", icon: "stores", path: "/multiloja", workspaceId: "multiloja" },
          { id: "configuracoes", label: "Config", icon: "settings", path: "/configuracoes", workspaceId: "configuracoes" },
          { id: "alertas", label: "Alertas", icon: "alert", path: "/alertas", workspaceId: "alertas" },
          { id: "feedback", label: "Feedback", icon: "feedback", path: "/feedback", workspaceId: "feedback" },
          { id: "banco", label: "Banco", icon: "database", path: "/banco", workspaceId: "banco" },
          { id: "manage-auditoria", label: "Auditoria", icon: "audit", path: "/manage/auditoria" },
          { id: "manage-integracoes", label: "Integracoes", icon: "integration", path: "/manage/integracoes" }
        ]
      }
    ]
  }
];

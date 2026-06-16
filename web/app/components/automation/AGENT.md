# AGENT.md — web/app/components/automation/

Modulo de automacao WhatsApp/IA (Omni). Frontend do modulo automation.

## Componentes

| Arquivo                        | Responsabilidade                                                                                                          |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------- |
| `AutomationWorkspace.vue`      | Orquestrador: barra de abas (Status / Comportamento / Fontes / Conhecimento / Modelos / Contexto), cabecalho, erro global |
| `AutomationStatusCard.vue`     | Abas "Status": grid 2 colunas — card WhatsApp (QR/conectado/botao) + card Robo (toggle liga/desliga)                      |
| `AutomationBehaviorCard.vue`   | Aba "Comportamento": form de persona (nome + textarea prompt) com emit v-model                                            |
| `AutomationSourcesCard.vue`    | Aba "Fontes": toggle catalogEnabled + lista editavel de siteUrls. Consome `useAutomationSources`                          |
| `AutomationKnowledgeCard.vue`  | Aba "Conhecimento": documentos manuais (CRUD). Consome `useKnowledgeDocs`                                                 |
| `AutomationModelsCard.vue`     | Aba "Modelos": escolha de modelo por funcao (chat/vision/audio/classifier). Consome `useAutomationModels`                 |
| `AutomationContextPreview.vue` | Aba "Contexto": preview do systemMessage montado (secoes colapsaveis). Expoe `refresh()`                                  |

## Composables

| Arquivo                                 | Responsabilidade                                                                        |
| --------------------------------------- | --------------------------------------------------------------------------------------- |
| `~/composables/useAutomation.ts`        | Status WhatsApp, connect/disconnect, toggle enabled, persona (load/save). Polling de QR |
| `~/composables/useAutomationModels.ts`  | Catalogo de modelos + selecao por role. Endpoints `/v1/automation/models`               |
| `~/composables/useAutomationSources.ts` | Fontes de conhecimento (M5-front). Endpoints `/v1/automation/sources`                   |
| `~/composables/useKnowledgeDocs.ts`     | Documentos manuais de conhecimento. Endpoints `/v1/automation/knowledge-docs`           |

## Contratos de API (M5-front)

```
GET  /v1/automation/sources  →  { catalogEnabled: boolean, siteUrls: string[] }
PUT  /v1/automation/sources  ←  { catalogEnabled: boolean, siteUrls: string[] }
```

Degrade limpo: se GET falhar (backend em paralelo ainda nao no ar), mostra defaults
`catalogEnabled=false / siteUrls=[]` sem quebrar a tela. Erro de PUT e exibido na UI.

## Layout

- **Abas horizontais** com keyboard navigation (ArrowLeft/ArrowRight).
- Aba "Status" sempre acessivel no topo (primeira aba); mostra indicador visual (ponto laranja/verde) quando QR pendente ou conectado.
- Mobile-first: abas fazem scroll horizontal em telas pequenas; painel de conteudo ocupa toda a largura.
- Componentes de detalhe (KnowledgeCard, ModelsCard, ContextPreview) sao reutilizados sem reescrever.

## Regras

- Nunca hex hardcoded: usar tokens `--primary`, `--success`, `--danger`, `--text-muted`, `--line-soft` etc.
- Sem emojis. Sem console.log.
- Cada arquivo < 450 linhas.
- AutomationWorkspace e o unico ponto de montagem: importa subcomponentes, nao duplica logica.

# AGENT.md — web/app/components/omnichannel/

Front do modulo **omnichannel** (inbox de atendimento WhatsApp). Rota `/omnichannel`.

## Direcao vigente — 2026-07-20

O inbox agora atende a arquitetura hibrida descrita em
`docs/omnichannel/PLANO_TECNICO_EVOLUCAO.md`: Go/PostgreSQL sao autoritativos; n8n apenas
orquestra a inteligencia e nunca envia ao canal. Para editar este front, usar as skills
Codex `$principios-engenharia` e `$omnichannel-hibrido` e ler tambem o AGENT do backend.

O backend ja existe e cobre leitura, envio/outbox, realtime, acoes, configuracao, filas,
triagem e CRM base. Os avisos de F1 abaixo sao historico do metodo de port, nao descrevem o
runtime atual. A evolucao imediata do front e o contato CRM 360°, atribuicao, handoff/SLA e
capabilities por WhatsApp oficial/Instagram sem duplicar telas por canal.

> ## LEIA ANTES DE EDITAR: este codigo e um PORT VERBATIM
>
> Os arquivos deste diretorio (e de `~/composables/omnichannel/`) foram copiados
> **byte a byte** do painel legado (`web-reference/app/**/omnichannel/**`) na fase
> **F1**. O front legado **e a especificacao** — ele e codigo maduro, em producao,
> que ninguem aqui escreveu. Reescrever enquanto se porta e trocar dois problemas
> por quatro (decisao **D-B** do canonico).
>
> Portanto: **nao reformatar, nao "arrumar" import/nome/estilo, nao splitar
> arquivo grande.** O refactor tem fase propria (**F14**). Quem mexer aqui por
> gosto de estilo quebra a rastreabilidade contra o `web-reference/`.
>
> Canonico: [`docs/omnichannel/PLANO_ATENDIMENTO.md`](../../../../docs/omnichannel/PLANO_ATENDIMENTO.md) ·
> Spec da fase: [`docs/omnichannel/specs/OMNI-F1.md`](../../../../docs/omnichannel/specs/OMNI-F1.md)

## Estado do port F1 — historico

A tela entrou inicialmente sem backend para preservar o port verbatim. Esse estagio foi
superado: o modulo Go e o schema `messaging.*` estao implementados. Qualquer badge atual deve
descrever apenas a condicao real do ambiente (por exemplo piloto Evolution/mock), nunca
"SEM BACKEND". Contratos pendentes ficam em `docs/omnichannel/ESTADO.md`.

## Estrutura

| Caminho                                     | Papel                                                                                               |
| ------------------------------------------- | --------------------------------------------------------------------------------------------------- |
| `OmnichannelInboxModule.vue`                | Orquestrador do inbox: switcher, alertas, `UDashboardGroup` com as 3 colunas                        |
| `OmnichannelInboxLoading.vue`               | Skeleton de carregamento. Usa `USkeleton` **sem bloco `<script>`** — auto-import do Nuxt, nao e bug |
| `inbox/InboxConversationsSidebar.vue`       | Coluna esquerda: lista de conversas, filtros, busca                                                 |
| `inbox/InboxChatPanel.vue`                  | Coluna central: cabecalho, corpo, composer, acoes de mensagem                                       |
| `inbox/InboxDetailsSidebar.vue`             | Coluna direita: detalhes do contato, atribuicao, notas                                              |
| `inbox/OmnichannelWhatsAppSessionModal.vue` | Modal de conexao do WhatsApp (QR, status, limpar historico)                                         |
| `inbox/types.ts`                            | Tipos locais dos componentes do inbox                                                               |
| `~/composables/omnichannel/*`               | 49 composables (logica do inbox)                                                                    |
| `~/pages/omnichannel/index.vue`             | A pagina. `layout: 'dashboard'` + `workspaceId: 'omnichannel'`                                      |

**Preservar a subpasta `inbox/`**: os imports do legado dependem dela.

## `config/` — telas de config (F10) — CODIGO DA CASA, NAO VERBATIM

A pasta `config/` **nao** e port verbatim: nasceu no design system da casa (spec
[`OMNI-F10.md`](../../../../docs/omnichannel/specs/OMNI-F10.md)). Aqui o teto de ~450
linhas/arquivo, tokens (`rgb(var(--...))`), classes `settings-collapse` e o feedback de
formulario **valem** — ao contrario do inbox verbatim. Nao misturar com o inbox.

Aberta por um drawer (precedente do calendario), com deep-link `?config=<aba>`. Ponto de
entrada = botao "Configurar atendimento" em `~/pages/omnichannel/index.vue` (gate
`isPlatformAdmin || <perm>.manage`).

| Caminho                                                                                 | Papel                                                                                     |
| --------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| `config/OmnichannelConfigDrawer.vue`                                                    | Host: abas + `?config=<aba>` + gate por permissao (platform_admin incluso)                |
| `config/ConfigNumbers.vue` + `ConfigNumberCard/Credentials/Capabilities/Connection.vue` | Numeros/providers/credencial `{set,last4}`/QR; UI degrada por numero via `Capabilities()` |
| `config/ConfigDepartments.vue` · `ConfigQueues.vue` · `ConfigQueueMembers.vue`          | Setores, filas e membros (diff incremental add/remove)                                    |
| `config/ConfigRoutingRules.vue`                                                         | Regras + reordenacao de prioridade (`PUT /routing-rules/order`)                           |
| `config/ConfigAiAgent.vue` + `ConfigAiAgentCard/Versions/Simulator.vue`                 | Editor do agente, publish/rollback e simulador (traco IA sugere / motor decide)           |
| `~/domain/omnichannel/config-api.ts` + `config-types.ts`                                | Client tipado (sem `any`) das rotas F2/F4/F8/F9                                           |

Consome rotas **ja existentes**: instancias/sessao/credencial sob `/v1/omnichannel/tenant/whatsapp/*`,
setores/filas/regras sob `/v1/omnichannel/settings/*`, agente sob `/v1/omnichannel/agents/*`.
Credencial de numero e keyed por **`instanceName`** (nao id). **needsWiring**: nao existe
`GET .../instances/{id}/capabilities` nem `provider` na view de gestao — a tela degrada
(capability desconhecida = ausente) ate o endpoint nascer na fase dona (F4).

## Por que fica em `web/app/` e nao em layer

Dentro de um layer o `~` resolve para `web/app`, e os imports dos arquivos
verbatim quebrariam (`web/layers/finance/AGENT.md:48-53`). O calendario e o
precedente: tambem nao e layer. Virar layer e **F14**.

## Costura (adaptadores temporarios — alvo F14)

O modulo verbatim importa APIs que o Omni nao tem com esse nome. A costura
traduz, sem tocar nos arquivos portados:

| Arquivo                                    | O que faz                                                                                    |
| ------------------------------------------ | -------------------------------------------------------------------------------------------- |
| `~/composables/useApi.ts`                  | `apiFetch` (prefixo `/v1/omnichannel`) + `ApiClientError`. Delega ao `createApiRequest`      |
| `~/composables/useAdminSession.ts`         | Os 8 membros de sessao que o modulo usa, derivados de `useAuthStore` + `useCoreAccountStore` |
| `~/composables/usePageBootstrapLoading.ts` | Loading de bootstrap da pagina (1 uso)                                                       |
| `~/stores/session-simulation.ts`           | Simulacao numerica do legado DESLIGADA; `hasModule` le os modulos da conta ativa             |
| `~/types/index.ts`                         | Barrel dos 36 tipos do legado (fan-in de 47 arquivos)                                        |

`~/stores/ui.ts` **ja existia** no Omni e serve — **nao foi tocado**.

### Regras que a costura protege (nao violar ao mexer aqui)

- **`X-Account-Id` nunca na mao.** O provider global injeta
  (`plugins/account-id-bridge.client.ts` → `createApiRequest`). Montar o header
  aqui cria uma segunda fonte de conta — o bug de `project_account_source_divergence`.
- **`tenantSlug` e exibicao, nunca escopo.**
- **Quem troca de conta e o switcher do shell**, nao o switcher interno do modulo
  (por isso `canSimulate = false`).

## Os 5 arquivos que NAO sao verbatim (repontados na F1)

| Arquivo                                  | O que mudou                                                                                                                                               |
| ---------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `useOmnichannelInboxRealtime.ts`         | socket.io **nao e dependencia do web** e o Go fala WS nativo com ticket → transporte **inerte**; a logica dos 3 eventos esta preservada. **F5 reescreve** |
| `useInboxChatGifAssets.ts`               | `/api/gif/*` (Nitro) → `/v1/omnichannel/gif/*` via `apiFetch`                                                                                             |
| `useAvatarProxy.ts`                      | `/api/avatar/whatsapp` (Nitro) → `/v1/omnichannel/avatar`, URL absoluta                                                                                   |
| `useInboxChatMediaActions.ts`            | URL literal do BFF → `apiFetch` (fetch cru nao recebe o `X-Account-Id`)                                                                                   |
| `useOmnichannelInboxOutboundPipeline.ts` | Removido o bypass que falava direto com o `:4000`                                                                                                         |

## Dividas conscientes (registradas, alvo F14)

- **Arquivos > 450 linhas** (o maior chega a ~1.3k). Violacao **consciente** do teto
  da casa: o teto vale para codigo novo, nao para o port. ESLint acusa `max-lines`
  — **e esperado**.
- **Modulo em `web/app/` em vez de layer.**
- **Os 5 adaptadores de costura.**

Ver `docs/LEGADO.md`.

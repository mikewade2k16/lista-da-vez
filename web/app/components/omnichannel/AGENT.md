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

### MVP de automação — `/omnichannel/automacao`

`automation/` é código novo da casa e não faz parte do port verbatim. A rota é administrativa,
usa `workspaceId: 'omnichannel'` e o link no inbox só aparece para `platform_admin` ou
`omnichannel.settings.manage`, igual ao gate real da API.

- `OmnichannelAutomationMvp.vue` mantém na tela somente Visão geral e Intervenções; toda
  configuração fica no drawer para preservar espaço operacional;
- o cabeçalho próprio foi removido: a rota usa o wrapper padrão `page-workspace`; cliente é um
  `AppSelectField` e recebe exclusivamente o catálogo filtrado pelo módulo `omnichannel` no Go;
- `AutomationProfileConfig.vue` vincula cliente, número e agente e edita os gates configuráveis;
- `AutomationAiConfigDrawer.vue` reutiliza o mesmo shell `OmniEntityDrawer` e as classes de
  configuração do Calendário, com tabs Atendimento, WhatsApp e IA. A tab WhatsApp reutiliza
  `ConfigNumbers` e o fluxo real de conexão/QR; prompt, provider, modelo e chave continuam
  persistindo nas entidades do agente Omnichannel — nunca em `calendar.*`; o select consulta
  `/agents/{id}/models` sem expor a chave;
- cada agente é um `settings-collapse`, com switch de ativação no próprio cabeçalho. Dentro dele,
  a ordem e as classes são as mesmas de `calendar/config/ConfigAi.vue`: Prompt, Provedor e modelo,
  Escopo por cliente e Chaves de API; todos iniciam fechados. `ConfigAiAgentProviderKeys.vue`
  apresenta os slots Gemini, GLM e OpenAI com status mascarado, e salvar/limpar um provider nunca
  sobrescreve os demais. No MVP existe apenas `Salvar configurações`: o Go persiste e ativa no
  mesmo commit, enquanto o versionamento fica interno para auditoria/rollback. Se houver um draft
  legado, o editor o recupera uma vez e o salvamento o promove; depois reidrata sempre a versão
  ativa. Transcrição de voz não aparece no MVP;
- cada número também tem switch no cabeçalho. Desativar libera o teto imediatamente; excluir usa
  `DELETE .../instances/{id}` e o Go bloqueia quando existe histórico. Ao criar, o card da conexão
  abre para tornar o botão de QR evidente. Somente `platform_admin` vê/edita o teto da conta;
- `AutomationInterventionList.vue` apresenta três projeções do backend: `IA atendendo`, com
  `Parar IA`; `Atendimento humano`, com `Passar para IA agora/nas próximas`; e `IA parada`, com
  preview/contagem do inbound sem resposta e `IA responder agora`.
  O grupo humano é importado explicitamente para não depender do nome gerado pelo auto-import do
  Nuxt e oferece `Encerrar conversa`, que usa o mesmo `PATCH .../status` autoritativo do inbox sem
  solicitar resposta da IA. Enquanto o estado for `human_active`, a FSM mantém a IA bloqueada.
  A resposta imediata agenda o dispatch autoritativo no Go; o card nunca possui composer nem envia
  diretamente pelo provider. Sem mensagem pendente, `Retomar nas próximas` conserva o fluxo de
  reabertura no próximo inbound;
- o comando manual aparece como `Forçar IA a responder`: ele solicita uma resposta mesmo após
  baixa confiança, máximo de respostas ou handoff sugerido. O card mostra o motivo específico e,
  quando disponível, confiança obtida/mínima ou máximo de respostas. A UI não promete contornar
  configuração ausente, falha técnica, limite mensal ou lease inválida;
- `ConfigAiAgentAdvancedSettings.vue` diferencia a confiança mínima para **responder** da confiança
  mínima para **encerrar**, explica debounce, janela de contexto, máximo de respostas e os dois
  toggles de handoff. Em máximo de respostas, `0` é exibido e persistido como `sem limite`;
  conversões com fallback truthy (`Number(valor) || 6`) são proibidas porque corrompem esse estado.
  `AutomationProfileConfig.vue` rotula a confiança de encerramento explicitamente;
- `AutomationHiddenContacts.vue` fica na aba `Pessoas ocultas`, visível somente com a permissão
  explícita `omnichannel.conversations.privacy.manage`. A ação de ocultar parte do detalhe da
  conversa e pergunta separadamente se o histórico anterior deve permanecer oculto; restaurar
  nunca fabrica mensagens no browser. Inbox e automação consomem a mesma supressão do Go;
- o inbox também expõe `Ocultos` ao lado de `Conversas`/`Contatos`, com leitura e restauração pela
  mesma API account-scoped. A recuperação não pode ficar descoberta somente dentro da Automação;
- `~/composables/omnichannel/useOmnichannelAutomationMvp.ts` concentra carregamento, save e polling;
- `~/domain/omnichannel/automation-api.ts` é o contrato tipado, sempre via `createApiRequest`.

O painel nunca oferece toggle para `validGenerationRequired`: a lease de `ai_generation` é uma
invariante do Go. O contexto estratégico é somente leitura da fonte do Calendário; não duplicar
formulário nem persistência. Realtime futuro deve apenas invalidar a leitura e refazer o GET.

### E5 — handoff humano no inbox

`InboxChatHeader.vue` expõe somente as ações autorizadas pelo backend: `Assumir` quando a
conversa está sem responsável e `Liberar` quando o usuário atual é o responsável. Os botões
chamam `POST /conversations/{id}/take` (com chave de idempotência) e
`POST /conversations/{id}/release` através de `useApi`; não escrevem estado de conversa no
browser nem enviam mensagens diretamente ao provider. O Go continua responsável pela FSM,
permissões, lock, cancelamento de dispatch da IA e reconciliação da resposta.

`InboxDetailsSidebar.vue` lê `GET /conversations/{id}/handoffs` e `/sla` ao trocar a conversa,
mostra apenas o snapshot sanitizado (motivo, resumo, nomes dos campos coletados e eventos SLA)
e oferece `PATCH /conversations/{id}/queue` para transferência quando o backend autoriza a
leitura das filas. Falha de permissão para listar filas não esconde o histórico do handoff nem
fabrica uma opção de transferência.

As áreas de `InboxDetailsSidebar.vue` usam `InboxDetailsSection.vue` com seções nativas
expansíveis. Somente `Visibilidade e IA` nasce aberta; o conteúdo dos demais blocos mantém altura
intrínseca e scroll no corpo da sidebar, sem flex-shrink que corte selects, botões ou textos.

## `config/` — telas de config (F10) — CODIGO DA CASA, NAO VERBATIM

A pasta `config/` **nao** e port verbatim: nasceu no design system da casa (spec
[`OMNI-F10.md`](../../../../docs/omnichannel/specs/OMNI-F10.md)). Aqui o teto de ~450
linhas/arquivo, tokens (`rgb(var(--...))`), classes `settings-collapse` e o feedback de
formulario **valem** — ao contrario do inbox verbatim. Nao misturar com o inbox.

Aberta por um drawer (precedente do calendario), com deep-link `?config=<aba>`. Ponto de
entrada = botao "Configurar atendimento" em `~/pages/omnichannel/index.vue` (gate
`isPlatformAdmin || <perm>.manage`).

| Caminho                                                                                 | Papel                                                                                         |
| --------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `config/OmnichannelConfigDrawer.vue`                                                    | Host: abas + `?config=<aba>` + gate por permissao (platform_admin incluso)                    |
| `config/ConfigChannelClientBindings.vue`                                                | Vínculo histórico canal→cliente, policy off/shadow/on, exceções e reparo assistido            |
| `config/ConfigNumbers.vue` + `ConfigNumberCard/Credentials/Capabilities/Connection.vue` | Numeros/providers/credencial `{set,last4}`/QR; UI degrada por numero via `Capabilities()`     |
| `config/ConfigDepartments.vue` · `ConfigQueues.vue` · `ConfigQueueMembers.vue`          | Setores, filas e membros (diff incremental add/remove)                                        |
| `config/ConfigRoutingRules.vue`                                                         | Regras + reordenacao de prioridade (`PUT /routing-rules/order`)                               |
| `config/ConfigAiCredentials.vue`                                                        | Cofre account-scoped: cria, nomeia, rotaciona e importa chaves legadas sem leitura do segredo |
| `config/ConfigAiAgent.vue` + `ConfigAiAgentCard/Versions/Simulator/MediaSettings.vue`   | Editor versionado com chave/modelo por resposta, áudio, imagem, vídeo e documento             |
| `~/domain/omnichannel/config-api.ts` + `config-types.ts` + `instagram-api.ts`           | Client tipado (sem `any`) das rotas F2/F4/F8/F9/E8                                            |

Consome rotas **ja existentes**: instancias/sessao/credencial sob `/v1/omnichannel/tenant/whatsapp/*`,
setores/filas/regras e cofre sob `/v1/omnichannel/settings/*`, agente sob `/v1/omnichannel/agents/*`.
Credencial de numero e keyed por **`instanceName`** (nao id). A view de gestão inclui `provider`;
`GET .../instances/{id}/capabilities` existe e a tela degrada para ausente somente se a consulta falhar.

`ConfigAiCredentials` e `ConfigAiRoleModelSelect` aceitam `credentialBasePath` opcional para
reuso pelo Assistente 360 via a fachada neutra `/v1/assistant/ai-credentials`; sem a prop, as
telas nativas continuam nas rotas Omnichannel acima. A prop opcional `accountId` existe somente
para o drawer cross-account da Operacao capturar o cliente operacional ja autorizado; consumidores
nativos nao a passam e continuam usando o provider global do shell. Em ambos os casos o navegador
recebe apenas metadados mascarados, nunca o segredo. No catalogo neutro, credenciais da agencia
canonica chegam com `ownedByAccount=false`, `readOnly=true` e `ownerName`; a tela identifica a
origem, permite selecionar/consultar modelos e nao oferece renomear, rotacionar ou excluir.

### E7/E8 — canais Meta no painel

`ConfigInstagram.vue` e `instagram-api.ts` usam somente rotas account-scoped do Go. Tokens são
write-only e nunca aparecem no estado do navegador. A aba lista comentários/menções, mostra o
rascunho da IA e só chama `decide` para aprovar/ignorar; não há envio Graph pelo frontend. O provider
Cloud usa a configuração por número existente e segue a mesma regra: template/janela são decididos
no Go, não pelo n8n ou pela UI.

### E6 — tools, conhecimento, evidências e aprovações

`config/ConfigAiToolsKnowledge.vue` usa `useOmnichannelToolsKnowledge` e o client tipado de
`~/domain/omnichannel/config-api.ts`. Binding, base, documento, chunks e vínculo de conhecimento
continuam escritos somente pelo Go. `ConfigAiToolRuns.vue` exibe apenas `inputMasked`/
`outputMasked` e metadados de execução para `omnichannel.audit.view`; propostas mutáveis ficam
separadas e só podem ser aprovadas/rejeitadas por `omnichannel.agents.manage`. O navegador nunca
recebe argumentos cifrados, credenciais nem executa provider. Aprovar somente muda o estado no Go;
o retry assinado do brain é o caminho de execução.

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

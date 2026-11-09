# Layer `finance` (front do modulo financeiro)

Portado do `web-reference` (Fase 14 do roadmap). Planilhas mensais de entradas/saidas
com autosave, efetivacao por linha, ajustes (+/-), contas fixas com composicao,
categorias e recorrencias de clientes (grupos por loja).

## Estado atual: API Go real (sem mock)

A fonte de dados e o **back Go real** `back/internal/modules/finance/` (schema
`finance.*`, migration `0187_finance_module.sql`, rotas `/v1/finance/*`). O mock BFF
(`web/server/`) foi **removido** em AC-12 (2026-07-02); o badge "MOCK" saiu de
`/finance`.

- Os composables usam `createApiRequest(runtimeConfig, () => auth.accessToken)`; o
  `X-Account-Id` entra sozinho pelo provider global (`account-id-bridge.client.ts`) —
  nao passar manualmente. Escopo de cliente (`coreTenantId`) via `useCoreAccountStore`.
- Dados persistem no banco real (sobrevivem a restart). Contrato JSON identico ao do
  mock (camelCase 1:1). Ver `docs/finance/PLANO_MODULO_FINANCE.md`, ADR 0002,
  `docs/LEGADO.md` #6 (RESOLVIDO) e `back/internal/modules/finance/AGENT.md`.
- **Realtime**: ainda nao ha broadcast; a tela re-busca sob demanda (pendencia).

## Estrutura

- `pages/finance.vue` — **port fiel** de `web-reference/app/pages/admin/finance.vue`:
  layout (`UDashboardGroup` + `UDashboardSidebar` redimensionavel + `UDashboardPanel`),
  cabecalho, tabelas entradas/saidas e cartao de saldo reproduzidos identicos, com as
  mesmas classes/estilos `finances-page__*`. A logica vem dos composables (usa o sheet
  editor direto e `provide(FINANCE_CONFIG_KEY)` para o painel). Rota `/finance`
  (placeholder demo em `app/pages/finance.vue` removido).
  > O layout usa componentes `UDashboard*` — no Nuxt UI **v4 o "Pro" foi unificado**
  > no `@nuxt/ui`, entao existem no pacote community instalado (4.8.0). `UDashboardGroup`
  > leva `!static !inset-auto !h-auto !w-full` para ficar em fluxo dentro da pagina.
- `components/finance/`
  - `FinanceConfigPanel.vue` — slideover de configuracao (injeta `FINANCE_CONFIG_KEY`),
    port fiel do slideover de referencia.
  - `FinanceLineCard.vue`, `FinanceRecurringGroupCard.vue` — cartoes de linha/grupo (port fiel).
- `composables/`
  - `useFinancesManager.ts` / `useFinancesConfigManager.ts` — camada de dados (`createApiRequest` -> `/v1/finance/*`).
  - `useFinanceSheetEditor.ts` — draft + autosave + linhas + efetivacao + recorrencia.
  - `useFinanceConfigEditor.ts` — categorias/contas fixas/recorrencias + autosave.
    Exporta `FINANCE_CONFIG_KEY` (usado pelo painel de config).
- `utils/finance-helpers.ts` — helpers puros (formatacao, datas, snapshot, recorrencia).
- `utils/finance-ids.ts` — UUIDs e ids deterministicos de recorrencia.
- `types/finances.ts` — contrato (espelha o schema-alvo `finance.*`).

## Padroes do layer

- `~` = app (`web/app`). Arquivos do layer referenciam os proprios via caminho
  relativo (`../types/...`, `../../core/stores/account`). Componentes compartilhados usam imports explícitos de `#components`, com os
  nomes registrados pelo Nuxt, sem depender de aliases inexistentes.
- Nuxt UI aqui e o **community `@nuxt/ui` v4** — no v4 o "Pro" foi unificado, entao
  os `UDashboard*` existem no pacote community (port fiel do web-reference mantido).
- `OmniMoneyInput` e reaproveitado do layer `tasks` (nao duplicar).

## Nav / gating

Gate por **modulo + workspace** (AC-12). Registro:

- `web/app/utils/workspaces.ts` -> workspace `finance` (`{ id:'finance', icon:'payments', path:'/finance' }`).
- `web/app/domain/utils/permissions.ts` -> `WORKSPACE_ACCESS_DEFINITIONS` (sem
  viewPermission fixa), `ROLE_WORKSPACES.platform_admin`/`.owner` incluem `finance`, e
  `MODULE_WORKSPACE_PERMISSION_PREFIXES.finance = 'finance.'` (papeis custom com
  qualquer permissao `finance.*` enxergam o workspace — padrao `isPlatformAdmin || has(...)`).
- `web/layers/queue/nav.config.ts` -> item `finance` com `workspaceId:'finance'` +
  `moduleId:'finance'`. Some para contas sem o modulo; `platform_admin` ve via bypass.
- `web/app/middleware/module-enabled.global.ts` -> `{ prefix:'/finance', moduleId:'finance' }`
  em `MODULE_PATH_GUARDS` (rota direta tambem gated).
- `pages/finance.vue` -> `definePageMeta workspaceId: 'finance'`.

Back: gating por prefixo `/v1/finance` em `moduleGatingRules()` (`app.go`); contas sem
o modulo habilitado recebem 403 `module_disabled`; `platform_admin` tem bypass.

## Pendencias

- Realtime/WebSocket (hoje re-busca sob demanda).

## Correções de 2026-11-01

- Editor separado em `useFinanceSheetSources` (configuração/recorrência),
  `useFinanceLineEditor` (linhas/ajustes), `useFinanceEffectiveEditor` (efetivação)
  e `useFinanceAutosave` (gravação). Normalização da API fica em utilitário próprio.
- IDs de recorrências incluem a planilha; IDs legados já salvos são preservados.
- Configuração só inicializa planilha nova ou é reaplicada pela ação explícita
  "Aplicar configurações atuais nesta planilha", com confirmação. Abrir, recarregar
  ou editar configurações não altera valores de planilhas históricas.
- `createFinanceSaveQueue` captura account, planilha e payload por trabalho,
  serializa gravações e retém falhas para tentativa posterior. O header capturado
  é exceção intencional ao provider global: gravações pendentes mantêm a conta
  original mesmo após troca do switcher. Respostas antigas não reidratam outra conta.
- Navegação interna aguarda flush; falha impede a saída. Fechar/recarregar com
  pendências aciona o aviso do navegador. Não há promessa de salvar após forçar
  fechamento do navegador ou desligar a máquina.
- Status de gravação distingue pendente, salvando, salvo e erro, com nova tentativa.
- Leitores ficam sem controles de mutação; a API continua sendo a autoridade.
  Planilhas fechadas bloqueiam edição até reabertura explícita via PATCH de status.
- Categorias e estilos foram extraídos para manter arquivos menores. Blocos de
  configuração começam recolhidos. Campos/popovers são componentes reais registrados.
- Testes em `finance-save-queue.test.ts`, `useFinanceSheetSources.test.ts` e
  `useFinanceSheetEditor.test.ts` (seleção e inicialização de nova planilha).

## Roteiro de QA

Funcionalidades, preparação, passos e resultados esperados: `docs/finance/ROTEIRO_TESTES.md`. Resultados históricos: `docs/finance/TESTES_FUNCIONAIS_FINANCE.md`.

- Exclusão de ajuste zera o total anterior antes de recalcular a lista normalizada, evitando promover o último valor removido a ajuste legado. Regressões em `useFinanceLineEditor.test.ts`.
- Categoria de linha manual tem ação explícita `Limpar categoria`; linhas fixas preservam o vínculo à configuração.
- Navegação mobile oferece seletor de planilha/criação; ações do cabeçalho quebram linha em telas estreitas.

Ajuste posterior de layout: seletor de status limitado a9rem, sem expansão para100%; ações permanecem na mesma linha no desktop e permitem quebra apenas abaixo de1024px. Conferido no navegador local após correção.

## Perfil fiscal pessoal e templates (2026-11-02)

Cadastro autoritativo em `/perfil#fiscal`, com atalho na configuração Finance. Contrato separado do PUT de categorias/contas fixas; salvar um não regrava o outro. Dados pessoais escopados por `account_id` + `Principal.UserID` (inclusive admin), nunca por userId vindo do cliente. Acesso exige membership, módulo Finance e `finance.sheets.view`; autoatendimento altera apenas o próprio perfil/templates, não autoriza mutações em planilhas nem em perfis de terceiros.

- Migration0304: `finance.user_fiscal_profiles`, `finance.user_fiscal_credentials`, `finance.user_fiscal_templates`. FKs com cascade; um padrão por usuário/conta.
- GET/PUT `/v1/finance/fiscal/profile`; PUT/DELETE `/v1/finance/fiscal/credentials`; GET/POST `/v1/finance/fiscal/templates`; PUT/DELETE `/v1/finance/fiscal/templates/{id}`.
- Escritas/deleções existentes exigem `expectedUpdatedAt` (409 se desatualizado). Novo perfil/template usa versão vazia. Templates alheios retornam404. Payload estrito e limitado a32KiB.
- Credenciais de portal write-only, cifradas pelo `secretbox` compartilhado (`OMNI_SECRETS_KEY` existente); GET retorna apenas `credentialsSet` e timestamp. Não existe GET da senha. Salvar perfil preserva acesso; alterar URL invalida acesso anterior. Cofre indisponível →503 sem gravar plaintext. Nenhuma chamada de login ao portal.
- Formulários manuais com confirmação para descartar/remoção; troca de contexto invalida respostas antigas. Pinia `app/stores/fiscal-profile.ts` centraliza HTTP; `useFiscalProfile` mantém rascunhos. Nenhum segredo em localStorage ou conversa.
- Template tem descrição, códigos opcionais, observações, percentual e retenção opcionais/nulos. Nenhuma tributação é deduzida. Escolha de template mostra prévia com emitente do perfil; não gera NFS-e.
- Testes em `fiscal_*test.go` e `useFiscalProfile.test.ts`; roteiro em `docs/finance/PERFIL_FISCAL_E_TEMPLATES.md`. A etapa inicial não incluía chat; a integração textual está descrita abaixo. Extrato por imagem e emissão continuam pendentes.

## Chat Finance e acesso gov.br (2026-11-02)

- Surface `finance` usa o chat compartilhado `/v1/assistant/chat/*`, configuração própria em `finance.assistant_configs` (migration0305), GET/PUT `/v1/finance/assistant/config`. Leitura exige `finance.config.view`, escrita `finance.config.manage` e versão `expectedUpdatedAt` (409 em conflito).
- Modelo/chave resolvidos pelo cofre canônico sem herdar persona/modelo/configuração do Calendar. Configuração inicial desativada; chaves nunca entram no retorno público. O painel oferece edição do cofre somente a administradores transversais, mantendo a política existente.
- Contexto somente leitura: planilha selecionada da conta, perfil e templates do próprio usuário. Até50 linhas por seção/10 ajustes por linha e10 templates; totais originais preservados, truncamento sinalizado. Sem credenciais de portal no contexto.
- Conversas Finance pessoais inclusive para agência/admin; sem escopo amplo de clientes, sem acesso implícito ao Calendar, sem propostas de escrita/emissão.
- Perfil admite `portalLoginMethod` portal/govbr. Alterar método ou URL invalida credenciais antigas; govbr rejeita armazenamento de senha. Botão Emissor Nacional preenche o URL oficial e método govbr; link de entrada direciona ao portal, não realiza login automático.
- Status e roteiro: `docs/finance/CHAT_E_AUTOMACAO.md`. Chat textual não implementa leitura de imagem nem automação fiscal. Login gov.br/MFA é feito pelo usuário no portal. Nenhuma nota foi emitida em QA.

## Comprovante → rascunho (2026-11-02)

Migration0306 `finance.document_drafts`, conta+usuário da sessão. GET `/v1/finance/documents` (10recentes), POST `/extract` multipart imagem única, PUT `/{id}` com versão. Upload exige `finance.sheets.view`, autoatendimento pessoal; fora do escopo404 e conflito409. Hash deduplica sem sobrescrever revisões. OCR Tesseract local em pipe, JPEG/PNG8MB/24MP, timeout35s/2slots, não guarda imagem nem chama provedor. Valor/data/pagador/recebedor preenchem o rascunho; tomador e competência não são inferidos do pagamento. Templates pessoais podem ser aplicados e campos editados. Dados lidos (não credenciais) entram no contexto pessoal da IA. Não há emissão fiscal nesta etapa. Documentação e testes: `docs/finance/COMPROVANTES_E_RASCUNHOS.md`.

### PDF e extratos comuns

Upload aceita PDF/JPEG/PNG/TXT/CSV/OFX8MB, multipart `file` (legado `image` aceito). Formato validado pelo conteúdo; extensão só para textos. PDF: Poppler por stdin, até10páginas/90s/32ktexto; extração nativa com normalização geométrica de centavos elevados, fallback OCR por página digitalizada, sem arquivo temporário. PDF com senha de abertura ou cópia proibida rejeitado; leitura permitida de PDF criptografado sem senha não é bloqueada. UTF-8/Windows-1252 para texto; OFX exige formato. `document_pdf_test.go` cobre arquivos reais por env opt-in e casos sintéticos. Dockerfiles principal/Mac incluem Poppler. Não existe emissão automática ou importação de transações múltiplas nesta etapa.

### Revisão dos dados preparados

Migration0307 `finance.document_reviews` mantém confirmações/revogações com snapshot sem segredos, hash e versões do perfil/rascunho. GET `documents/{id}/review`, POST/DELETE `documents/{id}/review/approval`, sob membership/Finance e autoatendimento `finance.sheets.view`; conta/usuário da sessão, recurso alheio404 e versão velha409. Aprovação transacional trava o perfil pelo advisory lock compartilhado e o rascunho pela linha; hash fornecido só é comparado ao resumo do servidor. Mudanças invalidam o registro efetivo; histórico preservado, confirmação idempotente. UI lazy-load mostra pendências, bloqueia dados incompletos e confirma pelo helper canônico. `emissionAvailable:false`; confirmar revisão não autoriza emissão. Executor/portal autenticado continuam pendentes. Roteiro: `docs/finance/REVISAO_DO_RASCUNHO.md`.

### Preparação sem portal

`FinanceServiceFields` edita os mesmos campos de serviço persistidos do rascunho (códigos/ISS/retenção), mantendo null/zero/false distintos. `downloadPreparation` no composable de revisão refaz GET autoritativo e usa projeção explícita `document-preparation.ts` antes de baixar JSON local. Hash/versões/pendências acompanham o arquivo; nunca incluir credenciais, texto OCR bruto ou autorização de emissão. Bloquear download com dirty; invalidar resposta em troca de contexto. Roteiro/testes em `docs/finance/REVISAO_DO_RASCUNHO.md`. Usuário escolheu autenticar no portal depois; não pressupor sessão ativa.

Comprovantes: botão Reidentificar pagador e recebedor usa o texto OCR persistido e preenche só nomes vazios; bloqueado com dirty/busy, resposta antiga descartada por generation. Rascunho reidrata do retorno canônico. Não depende da disponibilidade do chat IA.

Seletor de comprovantes limpa também o input nativo quando File vira null, para permitir escolher o mesmo arquivo novamente e disparar `change`. Não limpar em falha enquanto File permanece disponível para retry.

### Intake compacto e seleção do tomador

O formulário agrupa upload/ações e identidade das partes, recolhe tributação/observações/orientações. Cada parte mostra nome e CPF/CNPJ; `usePartyAsCustomer` copia somente a parte escolhida, limpa documento anterior quando a nova parte não possui documento e exige salvar o rascunho. Nunca atribui competência ou emitente automaticamente.

Preferência do usuário (2026-11-02): priorizar testes locais Go/Vitest; não usar Work/navegador para testes sem solicitação explícita, para reduzir créditos. Registrar honestamente os limites da validação visual.

Seletor de arquivo (2026-11-03): removido o filtro nativo `accept` para evitar bloquear a seleção por associação de formato do navegador/SO. O composable continua validando formato/extensão e 8 MB antes de enviar, e o backend valida o conteúdo real. Validação do seletor no navegador pendente, conforme preferência de não usar Work.

## Conversa Finance sem IA e acesso inline

FinanceConversation é a entrada padrão do chat Finance; usa GET/POST /v1/finance/conversation sem consultar provedor. Mensagens persistidas via Pinia, retry com mesmo requestId, respostas antigas descartadas após troca de conta/usuário. CPF/CNPJ e senha aparecem no FiscalAccessPanel dentro da conversa antes do teste. O perfil usa o MESMO FiscalAccessPanel/useFiscalProfile/store/API/cadastro/cofre. Campo senha não participa das mensagens; a UI também intercepta senha explícita colada em mensagem. Salvar acesso não testa login; Testar acesso é separado e bloqueado durante edição de credenciais.

Anexo/perfil só aparecem por solicitação, não como formulário permanente no topo do chat. Chat de IA anterior preservado como modo opcional; modo sem IA possui vocabulário delimitado e não promete diálogo financeiro irrestrito. Acesso real não validado sem credenciais. Não usar Work/navegador para QA sem solicitação explícita.

## Sessão e preparação no chat

FiscalAccessPanel oferece Manter sessão, pelo mesmo cadastro/cofre do perfil. useFiscalProfile armazena apenas estado e validade públicos; cookies nunca são enviados ao cliente. FinanceDocumentIntake vincula o comprovante selecionado à conversa via draftId, avisa edição pendente e permite Preparar revisão. Mensagens que alteram campos rematerializam o editor a partir do GET autoritativo; editores ficam bloqueados durante o envio para não perder alterações. A conversa aceita campos explícitos sem IA e mostra revisão aberta para confirmação vinculada às versões. Sem emissão nesta etapa. Testes locais incluindo compilação de todos os templates fiscais; sem Work.

## Interface empresarial Accounting — CT-04

`components/accounting/FinanceAssistantSurface.vue` reutiliza o shell do
CalendarChatPanel e mantém o fluxo pessoal FinanceConversation separado. Aba
empresarial só aparece com Accounting habilitado e accounting.issuers.view efetiva;
não há bypass frontend por papel. Histórico pessoal e dados empresariais não são
concatenados. Voz, anexos pessoais, minimizar, resize e fechar continuam no shell.

AccountingWorkspace orquestra cadastro declarado, retomada de diagnóstico,
perguntas progressivas, rascunho explicitamente NFS-e/NF-e, validação e confirmação
local de revisão. Fonte autoritativa são as rotas CT-03; sem mock ou persistência
em localStorage. Tipos em types/accounting.ts; HTTP/composição em
composables/accounting/useAccountingWorkspace.ts, estado em accounting-controller.ts.

Cada trabalho captura conta/ator e AbortSignal; troca de contexto aborta/invalida
respostas e impede requests subsequentes na conta anterior. Versões/idempotência
vão na gravação; resposta do servidor reidrata. Erro409 preserva edições. Trocas de
emitente/diagnóstico/documento com dirty exigem descarte explícito. Selecionar um
documento retoma seu diagnóstico persistido, sem inferir empresa por nome.
Listas usam paginação20 e carregamento incremental. Componente empresarial oculto
não faz novos fetches ao trocar de conta; KeepAlive é limitado a2 instâncias.

Revisão mostra o snapshot e pendências do servidor, exige permissão específica e
confirmação. Não há botão ativo de envio nem título Plus presumido: config/roteador
Plus autoritativos são CT-05; evidências completas CT-06/07; conectores CT-08/09.
Emissão permanece indisponível. Dados desconhecidos permanecem null e declarados
não são promovidos a evidência confirmada. Credenciais seguem no fluxo protegido
pessoal; nenhum campo de segredo é enviado por esse workspace.

Testes: accounting-controller.test.ts e accounting-components.test.ts; regressões
fiscais/assistant-routes/access. Roteiro visual e estado de aceite em
`docs/accounting/handoffs/CT-04.md`. Não declarar teste visual sem autorização.

## Integração Plus — CT-05

FinanceAssistantSurface recebe status público autoritativo e oferece aba Plus no
shell existente. crow-plus-controller controla contexto, abort/geração e chave
estável de retry. Estado/histórico separado por conta/ator/emitente. Comprovante
pessoal exige seleção explícita; formulários protegidos existentes são reutilizados.
Config Plus independente da IA, desligada por padrão. Histórico recente limitado
a20mensagens; API suporta paginação. UI visual ainda pendente de autorização.
Testes: crow-plus-controller.test.ts, accounting-components.test.ts e fiscais.

## Gestão de fontes — CT-06

AccountingKnowledge dentro do emitente: blocos fechados para bases/importação/
revisão-publicação/consulta. knowledge-controller e composable cuidam API/CAS,
permissões e descarte de respostas atrasadas por conta/ator/emitente. Upload local
selecionado é enviado base64 com original intacto; URL serve só de origem, sem fetch.
Resultado FTS mostra referências/offsets e limite de aplicabilidade, sem v-html.
CrowPlusConversation exibe evidências retornadas, não executa conteúdo recuperado.
Testes de controller e compilação Vue; visual pendente de autorização.

## Evidências e configuração IA — CT-07

PlusExplanationConfig é bloco fechado dentro da configuração Finance existente;
seleção própria de credencial/modelo do cofre e limites por conta, desligado por
padrão. Não substitui modelo/config do chat antigo. Tabela de preços é declarada
e versionada explicitamente; nenhum preço/modelo comercial cravado no código.
AccountingEvidence renderiza somente referências autorizadas do servidor, texto
escapado, hipóteses/pendências e metadados de uso. URLs do texto LLM não geram links;
registry de fontes permite linkhttps sem credencial. Ausência de custo/tokens não
mostra zero. Testes em utils/plus-evidence.test.ts e compilação de componentes;
inspeção visual permanece pendente de autorização.

## Obrigações CT-11 — 2026-11-05

AccountingObligations no emitente, bloco fechado e gated por obligations.view/manage.
Controller/composable separados, respostas antigas invalidadas por conta/ator/emitente,
paginação20 e erros preservando seleção. Frente/período explícitos não confirmam
regime; criar apenas pendência. Valor/data ausentes não viram0, sem pagamento/guia.
Regra real ainda não qualificada. Histórico/memória API não comprovam transmissão.
Testes controller e compilação de componentes; visual não executado.

## Atualização de fontes CT-12

KnowledgeIngestion dentro da base selecionada: catálogo permitido, subscrições e
runs por conta/ator/emitente/base. Controller/composable separados, geração/abort,
sem fetch oculto ou URL livre. Configuração exige seleção, confirmação e CAS;
retry de write incerto preserva key, contexto alterado impede follow-up. Habilitar
coleta não publica documento. Sem vetor/embedding/provedor novo. Testes controller

- compilação Vue; layout/browser e pipeline vivo da conta continuam pendentes.

## Chat progressivo — 2026-11-06

FinanceConversation inicia somente com mensagens, sem reabrir formulário antigo
a partir do histórico. Formulários de perfil, acesso e documento continuam disponíveis
por pedido explícito; “criar nota” conduz perguntas sequenciais no chat. Respostas
comuns após uma conversa de acesso não são ocultadas como credenciais. Senhas
explícitas continuam bloqueadas/omitidas e devem usar o formulário protegido.

Estado da pergunta é a action da mensagem assistente no histórico canônico
(ask:<campo>/issuer:<campo>), vinculado ao draftId, conta e usuário. Dados são
gravados nas APIs/services fiscais/documentais existentes, sem tabela ou migração
nova. Pode iniciar rascunho sem comprovante; chave de criação derivada do request
evita duplicar no retry. Pergunta um campo faltante por vez, reaproveita dados
do rascunho e template padrão. Não consulta cadastro recorrente empresarial nem
conclui a emissão real. Preparação conversacional não tenta login no portal.

“Cancelar” interrompe perguntas e preserva dados salvos. “Abrir formulário da nota”
continua oferecendo o editor/revisão completo sob solicitação. Histórico antigo
que já foi salvo com dados omitidos não pode ser reconstruído por esta correção.

## Crow Assistant Plus — entrada unificada Finance (2026-11-06)

A conversa principal usa finance.access_messages para respostas locais e IA, com
requestId idempotente e dados pessoais por conta/usuário. Não há seletor com/sem IA.
ConversationOrchestrator escolhe preparação/perfil/acesso locais, leitura Finance
canônica ou resposta opcional via cliente LLM compartilhado. Sem IA/configuração
válida, limite ou falha de provedor, continua com resposta local; não simula uma
resposta do modelo. Consulta financeira não depende da flag de ferramentas avançadas.

POST /v1/finance/conversation aceita financeSheetId/month opcionais; contexto vem
da planilha selecionada da mesma conta e recurso é validado no serviço Finance.
Novas respostas IA usam somente histórico pessoal limitado/mascarado, configuração
Finance e cofre existentes; não herdam chave global, não executam ferramentas nem
confirmam emissão. Revalida configuração/permissão após chamada. Sem retry interno.
Limites de contexto/tokens/tempo/diário e teto monetário são os mesmos configurados
no bloco de limites Plus. Pricing só é reutilizado para a mesma credencial/modelo.

Migration0316 permite issuer_id=null no ledger finance.plus_llm_runs para classe
conversation; nenhuma tabela de histórico nova. Reserva é durável antes da rede;
retentativa com reserva anterior incerta não faz nova chamada. Histórico bem-sucedido
replaya pela mensagem existente. Retenção do ledger continua30dias; não promete
recuperar resposta não gravada após crash nem deduplicação eterna de reserva expirada.

UI Finance passa a Crow Assistant Plus; conversa principal sem ficha inicial.
Empresarial mantém contexto próprio com emitente explícito, ferramentas/RAG existentes
e cadastros/revisão abertos por solicitação. Não unificar contas/emitentes por página.
Históricos anteriores IA/Plus permanecem consultáveis em leitura; sem copiar mensagens
entre tabelas. Unificação integral de históricos antigos/empresarial e expansão de
Calendar não estão concluídas. Áudio e ditado reutilizam composables existentes;
transcrição não envia a mensagem automaticamente. Teste visual/voz real pendente.

Detalhes, evidência e roteiro em docs/finance/PLUS_UNIFICADO.md. Aceite do usuário
continua pendente no CT-15; nenhuma conta/configuração/chave foi ativada por esta tarefa.

## Paridade visual e Whisper local — 2026-11-06

Referência aprovada pelo usuário: layout Crow original (imagem1). Não criar outro
composer/estilo para Plus. CalendarChatComposer extrai o DOM aprovado de áudio/input
e é usado tanto pelo CalendarChatPanel quanto pelo adapter FinanceConversationVoice.
Reutiliza calendar/chat.css, modos Whisper/Ao vivo, waveform, timer, pausa, descarte,
confirmação e botão enviar com ícone. FinanceConversation usa CalendarChatMessage
e stream/empty state canônicos. Formulários só sob solicitação, sem ficha inicial.

A transcrição Finance canônica resolve provider=local, sem consultar ativação/modelo
de conversa ou chave paga. Usa o webhook existente para o Whisper compartilhado.
WithFinanceTranscriptionAuthorization injeta a checagem viva de membership, módulo
Finance e finance.sheets.view via composição; revalidada após transcrever. Nenhuma
permissão concedida pela UI ou mudança em Calendar/Meta/global para transcrição paga.
Serviço local Whisper ativo/healthy e rota de transcrição configurada foram conferidos,
sem expor URL/env/credencial. Teste de áudio sintético cobre providerlocal/chavevazia
e revogação antes/depois; microfone do usuário e comparação visual não executados.
Não trocar modelo/baixar pesos ou ativar configuração de conta para corrigir áudio.

## Seletores independentes por conta — 2026-11-06

O seletor da página Finance troca a conta da página. O seletor do chat troca
somente o contexto do chat: página em A e chat em B é um cenário suportado.
Ambos usam contas autorizadas de `/v2/me/accounts`, sem agregar todas as contas
nem usar `coreTenantId` como fronteira de segurança. Só aparece seletor para
quem tem mais de uma conta; Finance não contratado permanece indisponível.

`FinanceAssistantHost` conserva uma seleção própria por usuário na sessão,
carrega `/v2/me/context` para a conta escolhida e injeta o contexto apenas na
árvore do chat. Conversa, perfil, documentos, ferramentas empresariais e áudio
enviam explicitamente o `X-Account-Id` capturado. Não chama `switchAccount`, não
altera cookie/conta global nem a planilha da página. Ao trocar, a árvore antiga é
desmontada; respostas tardias são descartadas. A API continua validando conta,
usuário, módulo e permissões; selecionar na UI não concede acesso.

Uma planilha da página só entra no pedido se pertencer à conta do chat. Quando
página e chat estão em contas diferentes, não herdar planilha/valores da página;
a escolha independente de planilha/período pelo chat ainda precisa ser ampliada
(P15-26). Perfis/conversas pessoais continuam por conta + usuário; cadastro de
emitentes não substitui a fronteira de conta e seus grants explícitos.

A janela do Plus e a original usam a mesma área-base e medidas compartilhadas
na sessão por conta. Apenas Calendar persiste seu próprio layout após hidratar;
Finance não faz PUT da configuração Calendar.

QA automatizado cobre contexto privado B sem mutar página A, áudio B com página
A/C e descarte após troca do chat, respostas de contexto fora de ordem, geometria
compartilhada e autorização viva de admin/owner/custom/revogação. Layout no
navegador, microfone real e aceite do usuário continuam pendentes.

## Busca pública no chat — pacotes 1/2, 2026-11-06

0317 cria `knowledge.public_sources`: acervo global exclusivamente público,
curado e informativo; não possui tenant porque não recebe uploads ou dados privados.
Não copia nem abre `messaging.knowledge_*`/bindings Accounting. Sem endpoint de
publicação pública, sem scraping no chat. Versões publicadas imutáveis (trigger),
retirada por status; nova busca seleciona versão publicada mais recente. Referências
históricas publicadas permanecem válidas; retiradas são ocultadas em lote no GET.
0318 adiciona cinco sínteses curtas com origem/revisão/limites em manifesto, sem
alterar configurações, contas, credenciais ou seeds anteriores. Vigência fiscal
não confirmada; são evidências informativas, nunca regra de cálculo do emitente.

SearchPublic → Store, FTS simple com normalização de acentos, até12termos e3trechos.
Aliases de vocabulário no roteador não contêm valores fiscais. Finance usa portas
PublicKnowledge/ValidatePublicEvidence, autorizadas pelo Finance canônico antes/
depois. Não exige Accounting contratado, emitente, modelo, cofre, Jeff ou API paga
para ler material público. Corpus privado empresarial conserva todos os gates.

finance.access_messages.public_evidence persiste snapshot estruturado separado do
texto; List revalida até180IDs e oculta fontes retiradas/indisponíveis. Conta/usuário
continuam filtrados no SQL. Perguntas fiscais explícitas no chat chamam busca antes
de qualquer modelo. Falta de fonte difere de falta de serviço. Nenhuma resposta é
marcada como confirmação jurídica ou execução de nota/declaração.

Frontend reutiliza AccountingEvidence e cartões/mensagens do Crow; mostra título,
trecho, fonte HTTPS e versão. Roteiro manual em docs/finance/ROTEIRO_TESTES.md.
Testes PostgreSQL sintéticos, corpus público inicial e regressões de conversa
passaram. Comparação visual, microfone real e aceite do usuário não executados.

CT-15 pacote 8 (2026-11-06): `useFinanceConversation` libera `selectedDraftId` quando a
resposta do servidor move a conversa para outro rascunho (ex.: nota nova após comprovante
lançado como despesa); o servidor é a fonte do rascunho ativo. Ações novas da conversa:
`pick:*`, `sheet:*`, `sheetpick:*`, `receipt:*` são estado do servidor, sem editor.

## Comprovantes, retenção e clientes — CT-15 fase B, 2026-11-09

- `components/finance/FinanceReceiptsConfig.vue`: no painel de configuração, lista os
  documentos enviados com vínculo à planilha (linha apagada/valor alterado em aviso),
  situação do original, baixar, desfazer vínculo e apagar original (confirmação).
- `components/finance/FinanceRetentionConfig.vue`: prazo dos originais (padrão 3 meses),
  contadores, próximo vencimento, histórico. Aviso quando o storage está desligado.
- `components/fiscal/FiscalCustomersPanel.vue` (perfil → dados fiscais): clientes salvos,
  edição que cria nova versão, histórico. CPF/CNPJ mascarado na lista.
- Dados via `app/stores/finance-records.ts` + `composables/useFinanceRecords.ts`
  (ticket por conta, abort ao trocar/desmontar, relê do back após cada ação).
- Fontes públicas citadas no chat mostram a revisão (dono/IA/contador) via
  `evidenceReviewLabel` em `utils/plus-evidence.ts`.
- Curadoria da base pública (platform_admin): `/manage/base-publica`
  (`app/components/admin/knowledge/*`), workspace `knowledge_admin` registrado nos mesmos
  pontos de `storage_admin` (workspaces, permissions, AGENCY_ONLY_PATHS, nav).

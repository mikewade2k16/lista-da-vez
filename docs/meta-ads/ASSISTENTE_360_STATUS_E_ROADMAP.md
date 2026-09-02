# Assistente 360 + Meta Ads — estado real e roadmap

> **Documento canônico vigente**, atualizado em 2026-08-27. O
> [plano anterior](./PLANO_INTEGRACAO_META_ADS.md) é um arquivo histórico e não
> deve orientar implementação ou deploy. Nenhum item é considerado concluído
> apenas porque existe em prompt, mock, arquivo local ou roadmap: é preciso haver
> enforcement no backend e validação proporcional ao risco.

## Checkpoint operacional — 2026-08-21

O escopo funcional desta rodada está congelado neste checkpoint. Não devem entrar
novas features antes de homologar o que já foi integrado e fechar os bloqueios
listados abaixo.

### Estado do Crow Assistant local e do Calendário

- A API, o painel e o PostgreSQL estavam saudáveis, mas `redis` e `n8n` do profile
  `automation` estavam parados. Isso fazia
  `GET /v1/assistant/chat/status` terminar repetidamente em `504` antes de testar
  qualquer chave de IA. Em 2026-08-19 foram iniciados somente `redis+n8n`; ambos
  ficaram `healthy` e `http://127.0.0.1:5680/healthz` respondeu `200`.
- A chave alterada pela tela antiga do Calendário foi gravada em
  `/v1/calendar/ai-keys/global`. Esse cofre continua atendendo plano mensal e
  compatibilidade legada; ele **não é a fonte primária do Assistente 360**.
- O workflow `Calendar Chat` foi copiado para backup, importado somente pelo owner
  `calendar`, reativado e sincronizado com
  `automation/export/workflow-calendar-chat.json`. O export pós-import foi
  normalizado e comparado com o arquivo versionado (`STATUS:unchanged`). Nenhum
  workflow de Omnichannel/Automation foi importado.
- O chat compartilhado resolve provider/modelo/chave por
  `automation.omni_chat_configs` + `messaging.ai_credentials`. No banco local há
  credenciais cifradas e mascaradas. O drawer aceita quantidade livre de chaves
  nomeadas OpenAI, Anthropic, Gemini e GLM; cada chave recebe apelido e a ativa é
  trocada no seletor sem editar env ou n8n. Anthropic usa a API Messages nativa;
  o cofre legado dos agentes Omnichannel continua com o contrato anterior.
- Por decisão do responsável, a chave OpenAI atual não será rotacionada nesta
  rodada e essa rotação não bloqueia o Meta Ads. O valor bruto continua proibido
  em documentação, Git, logs ou env versionado.

### Preservação do Calendário e dos demais módulos

- O Calendário continua sendo o motor de UX/conversa do chat, mas a configuração
  de IA do Assistente 360 pertence ao runtime compartilhado. Os dois cofres ainda
  coexistem por compatibilidade. A aba IA do Calendário agora identifica suas
  chaves como plano/compatibilidade e direciona a rotação do Crow Assistant para
  a engrenagem do chat, evitando salvar a credencial no cofre errado.
- Confirmações locais deixaram de executar mutações no navegador. `event`, `note`,
  `clientProfile`, `task` e `taskItem` usam recibo idempotente e transação
  PostgreSQL. Eventos vinculados a Tasks voltaram a preservar o comportamento de
  produção: a mutação Calendar e o recibo são atômicos; o espelhamento na Task é
  pós-commit e best-effort, como no endpoint normal do Calendário.
- `go test ./...` passou. Em PostgreSQL 16 limpo, as migrations 0001–0294 foram
  aplicadas; a integração da 0287 provou replay sem efeito duplicado, conflito de
  hash e revogação de capability no Calendar. No front, 28 testes focados de
  chat/API/store Meta passaram; ESLint teve zero erros nos arquivos alterados. O
  build Nuxt dentro do container dev terminou em `137` pelo limite rígido de 4
  GiB. O build oficial revelou e corrigiu primeiro um lockfile incompatível com
  `npm ci`; depois compilou 3.131 módulos e concluiu client/server, mas o Docker
  Desktop local ficou preso ao iniciar o prerender e passou a responder `500`.
  O build temporário foi cancelado para preservar a stack. O artefato final ainda
  deve ser gerado em runner com RAM/daemon estáveis.
- Não houve commit, push, deploy, OAuth Meta real ou chamada Graph de escrita
  nesta rodada. `META_ADS_WRITES_ENABLED` permanece `false`.
- Em 2026-08-24, uma integração PostgreSQL adicional criou uma agência, dois
  clientes da mesma organização e um cliente externo. Ela validou vínculo de ad
  accounts, vínculo Page/Instagram, visão agência `all`, agência filtrada por
  cliente, leitura restrita dos dois clientes e rejeição cross-org. O teste
  encontrou e corrigiu a comparação `uuid = text` no SQL bounded do contexto Meta.
- No mesmo dia, staging recebeu a sequência pendente oficial `0159–0294` —
  necessária porque o banco estava em `0158` — e terminou com 249 migrations,
  última `0294`. As 13 migrations `0282–0294` e seus objetos principais foram
  conferidos. Somente o PostgreSQL foi ligado durante a operação e voltou a ficar
  desligado; API/web não foram implantadas. Dois backups pré-migration ficaram em
  `/home/deploy/lista-atendimento-staging/backups/`.
- Em 2026-08-27, o gate P0 local foi endurecido. O criativo de post passou a usar
  `object_id` para a Page, junto de `instagram_user_id` e
  `source_instagram_media_id`, conforme o codegen atual do Business SDK oficial;
  `page_id` top-level foi removido. Testes HTTP agora comprovam targeting somente
  Instagram, árvore pausada, timeout pós-request como `unknown`, `429` sem repetir
  campaign/ad set, replay terminal sem segunda execução e reconciliação pelos
  recibos parciais. A suíte completa do pacote Meta Ads passou.

### Decisão de prontidão

- **Calendário/runtime:** código, migrations, workflow, API local e regressões
  transacionais estão validados. Cofre multi-chave e troca de credencial ativa são
  parte do único modal do Assistente. A rotação da chave atual foi retirada do
  escopo por decisão do responsável; smoke externo de texto/voz continua uma
  verificação de ambiente, não uma lacuna de implementação. O build final deve
  rodar no CI/runner.
- **Meta Ads leitura/configuração:** implementação local avançada e coberta por
  testes; as migrations chegaram ao staging, mas Meta App first-party, OAuth/Graph
  real e o E2E autenticado ainda não foram homologados.
- **Meta Ads escrita:** o código local agora possui executor first-party para
  criar campanha pausada, duplicar profundamente como pausada, editar
  nome/orçamento BRL, pausar, retomar CBO dentro do teto comprovado e promover um
  post real do Instagram criando campanha → conjunto → criativo → anúncio. Todas
  as entidades veiculáveis nascem `PAUSED`; cada etapa da árvore possui recibo
  at-most-once. **Ainda não está 100% homologado** porque o kill switch continua
  desligado e nenhuma escrita real foi executada contra uma conta Meta de staging.
  O gate P0 local de falhas e payload foi fechado em 2026-08-27; o gate externo
  continua separado e não foi reclassificado como concluído.
- **Prontidão para produção do Calendário:** aprovada no código e no banco, sujeita
  ao smoke autenticado com chave rotacionada e ao build de imagem no pipeline com
  memória suficiente. Isso não aprova Meta Ads para produção nem liga writes Meta.

### Onde acompanhar sem depender desta conversa

- A página Roadmap do painel possui, no início da fase **META**, checkpoints
  separados para runtime, cofre, smoke, regressão Calendar, migrations de staging,
  E2E Meta e executor local da árvore de anúncio.
- Este documento detalha arquitetura, lacunas e critério de aceite. A página é o
  resumo operacional; este arquivo é a fonte canônica para investigação e deploy.
- Item `done: true` significa validado no nível descrito na nota. Código local,
  mock ou teste isolado não transformam um gate externo em concluído.

## Resultado da auditoria

O módulo já possui uma base útil de leitura: conexão por token cifrado, descoberta
de contas de anúncio, sincronização de campanhas e insights, dashboard e acesso ao
feed do Instagram Business. O antigo runner MCP foi preservado somente como
compatibilidade interna read-only; ele não integra o produto nem executa writes.
Antes desta revisão, o caminho de escrita não atendia ao contrato de segurança
anunciado nos documentos.

Antes desta revisão existiam três chats paralelos:

1. o chat do Calendário, com a melhor UX, voz, conversas, memória e cards;
2. o chat legado do Meta, que chamava o runner/MCP diretamente;
3. o Omni Chat de Automação, com a configuração e o cofre de credenciais mais
   atuais, mas sem o histórico e os cards do Calendário.

Além da duplicação, o runner mantinha token OAuth, login pendente e sessão MCP em
estado global. Em um serviço com várias contas isso criava risco de uma conta
herdar o contexto de outra. A “confirmação” de escrita documentada era somente uma
frase no system prompt: as tools eram pré-aprovadas e podiam executar no mesmo
turno do pedido. Portanto, MA3/MA4 nunca estiveram tecnicamente concluídas.

## Arquitetura decidida

O produto passa a ter um único Assistente Omni no shell autenticado:

```text
Calendar / Meta Ads / demais páginas
                 │ surface + account + usuário autenticado
                 ▼
        /v1/assistant/chat/*
                 │
       conversas, voz e cards do Calendar
                 │
        capabilities efetivas no Go
      ┌──────────┼───────────┬──────────┐
   calendar    tasks       meta_ads    users
      │          │             │          │
  fontes Go/PostgreSQL autoritativas e escopadas por conta/cliente
                 │
      configuração/provider/modelo/chave
       automation.omni_chat_configs + cofre messaging
```

- A rota em que a conversa nasceu define apenas os defaults da `surface`; não
  concede permissão.
- O acesso efetivo é sempre a interseção entre configuração da surface, módulo
  contratado/habilitado e RBAC do usuário.
- A surface de uma conversa persistida é imutável. Navegar para outra página não
  troca silenciosamente o contexto da conversa aberta.
- Agência pode selecionar todos os clientes visíveis. Usuário de cliente continua
  preso ao próprio escopo resolvido no servidor.
- Meta Ads é capability/adaptador do chat, e não uma quarta implementação de IA.

## Entregue localmente neste ciclo

- Host único do chat no layout autenticado, reutilizando o painel completo do
  Calendário em vez de montar outro chat na página Meta.
- Rotas neutras `/v1/assistant/chat/*`, mantendo aliases legados do Calendário
  para compatibilidade.
- `entry_surface` persistida em cada conversa e `surface_modules` por conta, com
  defaults separados para Calendário, Meta Ads e visão global.
- Modal compartilhado para ligar/desligar acesso de leitura/escrita a Calendário,
  Tasks, Meta Ads e Usuários por surface, além de provider, modelo, prompt,
  temperatura, memória e credencial do cofre.
- Reset fail-closed do estado do chat ao trocar conta/usuário, cancelando requests
  antigas para que uma resposta da conta anterior não apareça depois da troca.
- A migration 0287 adiciona recibos idempotentes no backend para `/ask`: cada
  efeito exige `Idempotency-Key` estável, é claimado antes de side effects e um
  retry de request já concluído reproduz exatamente o snapshot `succeeded`.
- A mesma migration persiste a execução dos cards locais. `event`, `note`,
  `clientProfile`, `task` e `taskItem` gravam efeito e recibo na mesma transação.
  Tasks usa uma porta `pgx.Tx` própria e dispara auditoria/realtime/sync somente
  após o commit. Eventos vinculados mantêm o sync pós-commit já existente. O teste
  PostgreSQL real cobre replay, ausência de duplicidade e capability revogada.
- A migration 0292 amplia somente o cofre compartilhado e a configuração do
  Assistente 360 para `anthropic`; os keyrings/agentes nativos do Omnichannel não
  foram ampliados. O workflow usa `POST /v1/messages` com `x-api-key` e
  `anthropic-version`, normalizando a resposta para o parser comum.
- Endpoints Meta passam a usar a account validada por `RequireAuthWithAccount` e
  permissões `meta_ads.view`, `meta_ads.manage` e `meta_ads.connect`; não confiam no
  `X-Account-Id` cru dentro do handler.
- O painel read-only (`overview`, seletor de ad accounts, campanhas, insights e
  `sync`) já resolve a conexão central da conta-agência para um cliente da mesma
  organização e só aceita contas de anúncio vinculadas ao seu `client_account_id`.
- A API já expõe `PATCH /v1/meta-ads/ad-accounts/{id}/client` (`meta_ads.manage`)
  para a conta-agência vincular ou desvincular uma ad account. O backend valida
  que o cliente está ativo, não é agência e pertence à mesma organização.
- A aba Conexões já administra ad accounts e identidades Page/Instagram por cliente.
  `GET/PATCH /v1/meta-ads/instagram-identities*` revalida a identidade na Graph e
  persiste somente o vínculo tenant-scoped da migration 0288; cliente não remapeia.
- Contexto Meta read-only injetado no chat com conexão, contas de anúncio,
  campanhas cacheadas e posts reais do Instagram, sempre filtrado novamente pelo
  escopo autorizado. Em conexão central, o client scope recebe apenas a interseção
  exata `Graph atual × Page/IG mapeada`; em `all`, posts de múltiplas identidades
  mantêm username/Page/cliente próprios e respeitam tetos globais.
- Esse contexto limita no PostgreSQL a leitura a 12 ad accounts autorizadas e
  100 campanhas globais, interrompe consultas de campanha quando o saldo acaba e
  consulta performance de no máximo 12 contas/90 dias/360 pontos. Flags explícitas
  de truncamento para contas, campanhas, performance e série diária avisam a IA
  quando o recorte não representa a lista completa.
- O chat/runner legado foi desmontado da superfície pública. Seu `/run` permanece
  autenticado, isolado por account e tecnicamente read-only apenas para
  compatibilidade; não é o braço de ação do produto.
- OAuth, login pendente, sessão MCP e bridge do runner isolados por `accountId`
  validado, sem path derivado de entrada livre.
- Workflow do Assistente 360 com schema fechado `kind=metaAction` para criar,
  duplicar, editar, pausar e retomar campanhas ou promover um post real do
  Instagram. O workflow só emite a intenção quando a capability efetiva é
  `meta_ads=write`; o Go descarta IDs fora do `context.metaAds` autorizado, deriva
  novamente a ad account e nunca aceita Page/IG enviada livremente pelo modelo.
- Propostas Meta persistidas em `meta_ads.action_proposals`, com chave
  idempotente, auditoria append-only, expiração em 30 minutos e binding ao card
  somente depois de a mensagem do chat existir. Proposta órfã ou não vinculada
  não pode executar.
- Card compartilhado mostra ação, conta, campanha, mudança/orçamento, status,
  indisponibilidade e erro acionável. Confirmação visual chama primeiro o
  executor durável e só marca o card aceito depois de `succeeded`.
- Confirmação textual determinística pelo comando integral exibido no card:
  `CONFIRMAR META <prefixo>` ou `CONFIRMAR GASTO META <prefixo>`. “Sim” e
  “confirmar” genéricos nunca executam; prefixo ausente ou ambíguo não chega ao
  modelo nem à Graph.
- Recusa cancela a proposta durável antes de marcar o card. Excluir uma conversa
  cancela suas propostas pendentes; ação já claimada bloqueia a exclusão. Reload
  hidrata o card a partir do estado autoritativo da proposta, inclusive
  `cancelled` e `expired`.
- Escrita não depende do runner MCP. O executor first-party implementa
  `create_campaign`, `duplicate_campaign`, `update_campaign`, `pause_campaign`,
  `resume_campaign` e `promote_instagram_post`. Criação/duplicação nasce pausada;
  retomada exige budget CBO vivo e cap; qualquer gasto exige confirmação reforçada.
- As migrations 0293/0294 ampliam o guard BRL de criação e registram recibos
  separados para campaign/ad set/creative/ad. Antes da confirmação e novamente
  sob o lease da conexão, o backend revalida ad account, vínculo cliente,
  Page/Instagram e o post na Graph atual. Uma etapa externa incerta nunca é
  repetida automaticamente; a trilha exibe todos os IDs confirmados.
- O guard 0290 captura e revalida connection revision, mapping, policy e campanha
  dentro do claim; o token da revision é lido sob advisory lock mantido durante o
  preflight/POST. Rotação, exclusão, expiry ou drift falham antes de nova mutação.
  Budget executável está restrito inicialmente a BRL.

Tudo acima é alteração local. As migrations 0293/0294 e o rebuild da API foram
aplicados no localhost, e o workflow Calendar Chat foi reimportado owner-scoped.
Deploy/staging, login OAuth e chamadas reais à Meta não são presumidos como feitos.

O kill switch de escrita permanece desligado por padrão. Portanto, esta entrega
fecha o contrato local de proposta, confirmação, cancelamento e auditoria, mas não
declara escrita real homologada na Meta.

## Validação local executada em 2026-08-21

- `go test ./...` passou em todo o backend após a inclusão da árvore de anúncio,
  sem regressão nos pacotes Calendar, Tasks, Automation ou Omnichannel.
- O lint focado de Calendar, Meta Ads, composition root e migrations passou com
  `0 issues`; o helper morto `decodeMedia` do Calendar foi removido sem alterar o
  contrato de leitura ou escrita de mídia.
- As migrations 0001–0294 foram aplicadas com sucesso em PostgreSQL 16 efêmero. O
  banco temporário foi removido depois do teste.
- A integração PostgreSQL da 0287 foi executada no mesmo schema atualizado e
  passou para efeito único, replay, conflito de hash e revogação de surface.
- Testes do executor comprovam árvore totalmente pausada, quatro recibos,
  validação do post/vínculo vivo e zero POST quando a fonte mudou. Testes do
  `MetaClient` comprovam Bearer e ausência de token na URL para campaigns,
  ad sets, creatives e ads.
- 28 testes focados do frontend passaram para chat, normalização fechada,
  confirmação e store Meta. O workflow `calendarchat0001` foi importado apenas
  pelo owner `calendar`, reativado e comparado com o export canônico: alinhado.
- API local reconstruída e saudável; migrations 0293/0294 aplicadas. Web, API e
  n8n responderam saudáveis. Em 2026-08-24, `package-lock.json` foi sincronizado
  para `npm ci`, produção passou a desabilitar sourcemaps e o Dockerfile passou a
  fixar `NODE_ENV=production` antes do config Nuxt. O target final compilou os
  bundles client/server, mas o Docker Desktop local travou no prerender; o build
  de imagem continua como gate em runner estável. Nenhuma escrita Graph real foi
  feita.

Complemento executado em 2026-08-24:

- integração PostgreSQL `TestAgencyTwoClientScopePostgresIntegration` passou com
  agência, clientes A/B e organização externa; cobre mapping de ad account e
  Page/Instagram, `all`/cliente, posts do contexto 360 e bloqueio de leitura ou
  remapeamento fora do tenant;
- a integração revelou e corrigiu um cast ausente em
  `ListAssistantAdAccounts`: a query comparava o UUID do owner com parâmetros
  textuais e falhava assim que havia conexão Meta real;
- 32 testes focados do frontend passaram para store Meta, vínculo de conexão,
  actions/policies e chat compartilhado; ESLint ficou com zero erros nos arquivos
  Meta (somente warnings de tamanho já existentes);
- staging recebeu as 134 migrations pendentes `0159–0294` de forma transacional,
  com backup anterior e validação dos 13 registros `0282–0294`. Isso foi somente
  migração de banco, sem deploy da API/web e sem habilitar writes.

Esses resultados validam o contrato local, não substituem migration/deploy em
staging nem OAuth/E2E contra uma conta real da Meta.

## Contrato de escrita segura

Não basta mostrar um botão “Confirmar”. A confirmação de Meta precisa ser uma
operação única no backend:

1. a IA devolve uma proposta fechada, sem executar tool;
2. o backend valida novamente conta, cliente, ad account, limites, permissão e
   snapshot do alvo;
3. o usuário aprova o card;
4. o backend grava uma execução com chave idempotente **antes** da chamada externa;
5. somente o executor first-party autorizado chama a Graph;
6. resultado, IDs Meta, erro e autor ficam auditados;
7. retry devolve a execução existente e nunca repete a criação/gasto.

Se houver timeout depois de enviar à Meta, o estado deve virar `unknown` e exigir
reconciliação; repetir cegamente seria capaz de duplicar campanha ou orçamento.
O contrato local já cobre o lifecycle e a idempotência acima. A disponibilidade
por ação é deliberadamente menor que o schema de intenção:

- `pause_campaign`: executor disponível quando o kill switch e o contexto real
  permitirem;
- `update_campaign`: somente nome/orçamento; orçamento usa cap e aceite reforçado;
- `create_campaign`: cria campanha `PAUSED`; resposta incerta não permite retry;
- `duplicate_campaign`: deep copy `PAUSED`; resposta incerta não permite retry;
- `resume_campaign`: somente CBO com orçamento atual vivo em BRL dentro do cap;
- `promote_instagram_post`: revalida post/identidade/mapping e cria campaign,
  ad set, creative e ad com recibo individual; campaign/ad set/ad nascem `PAUSED`;
- `unknown`/`executing`: não repetem a mutação e oferecem reconciliação visual;
  ainda não existe comando textual de reconciliação.

Até migrations, preflight final e E2E Graph passarem em staging, o kill switch deve
continuar desligado mesmo que a surface esteja configurada com `meta_ads=write`.

## Gaps funcionais restantes

### P0 — bloqueiam produção com escrita

- **Migration concluída em staging em 2026-08-24.** Ainda falta homologar
  concorrência, timeout, reconciliação e retry contra uma conta Meta real de
  staging; nenhum teste local substitui esse E2E.
- **Gate de implementação local concluído em 2026-08-27.** Timeout real após o
  envio vira `unknown`; confirmação repetida não chama o executor novamente; `429`
  no ad set preserva o recibo da campaign e não repete nenhuma das duas etapas; a
  reconciliação devolve os IDs parciais. O formulário do criativo usa
  `object_id` + `instagram_user_id` + `source_instagram_media_id`, e o ad set
  restringe `publisher_platforms` a `instagram`.
- Manter o kill switch desligado até o fluxo acima e os caps de budget serem
  validados em moeda real.
- Homologar em uma conta de teste as seis ações first-party, inclusive retorno
  `unknown`, expiração, rotação de token e rejeição `4xx/429`; só depois alterar
  `META_ADS_WRITES_ENABLED` por ambiente.
- Validar no staging o formato aceito pela versão Graph configurada para
  `source_instagram_media_id`, targeting Instagram-only e POST engagement. Se a
  conta exigir campos adicionais, ajustar o contrato fechado antes de liberar.

### P1 — necessários para a experiência agência/cliente completa

- A integração PostgreSQL de agência + dois clientes passou localmente para ad
  account, Page/Instagram, `all`/cliente e bloqueio cross-org. Ainda falta somente
  repetir o mesmo cenário com assets reais da Graph e usuários reais em staging.
- Sync em worker agendado com rate budget/backoff e estado de freshness. A
  paginação por cursor e o snapshot transacional já estão implementados.
- Refresh/reconexão/revogação do token first-party, alerta de expiração e runbook
  de recuperação sem intervenção no banco.
- Agregações corretas de conversão/ROAS/CPA por objetivo e fonte de receita.
- Tornar insights por campanha/ad set/ad/creative realmente consumíveis, com
  attribution window, breakdowns, frequência, vídeo e exportação.
- Inventário de Business portfolios e assets (Pages sem IG, pixels/datasets,
  catálogos e permissões), além de múltiplas conexões quando a agência precisar.
- Targeting/audiences, geo/idade/interesses/exclusões, placements, optimization
  goal, billing event e bid strategy sob policies server-side.
- Sweeper de `unknown/executing` e alertas de quota, expiry, custo e falha Graph.
  Timeline/API e card de histórico já existem.
- E2E: listar posts → escolher post → revisar campanha/ad set/ad → aprovar → criar
  pausada → sincronizar → exibir IDs/resultados no mesmo card.

### P2 — produto completo

- Tornar a leitura de múltiplos feeds do Instagram concorrente com limite baixo e
  degradação parcial: hoje até oito identidades são consultadas em sequência e uma
  falha Graph encerra o bloco de posts como `unavailable`, sem misturar resultado
  incompleto entre clientes.
- Editor visual manual espelhado às mesmas propostas/validações do chat.
- Dashboards ricos, comparação de períodos, exportação e visão por cliente.
- Observabilidade de custo, latência, rate limit e falhas Graph/MCP por conta.
- Retenção/purge das conversas e trilhas de auditoria segundo a política LGPD.

## Tokens e ações inevitáveis do usuário

O sistema deve gerar URLs, persistir tokens, fazer refresh e esconder detalhes
técnicos. Ainda assim, OAuth não pode ser totalmente silencioso: Meta/Facebook
exige que uma pessoa autenticada conclua login, 2FA e consentimento. Criar/configurar
o Meta App, aprovar permissões em produção e qualquer ação que possa gerar gasto
também exigem autorização humana.

O caminho de produto é somente o Facebook Login first-party usado pelo Go para
relatórios, feed e executor confirmado. O OAuth do runner MCP é legado interno e
não deve ser apresentado ao usuário nem exigido em produção.

O Facebook Login está implementado localmente: start autenticado por account,
state SHA-256/TTL/single-use no PostgreSQL, callback publico, troca server-side por
token de longa duracao, verificacao server-side de todos os scopes solicitados (incluindo
`pages_read_engagement` para discovery Page/Instagram) em
`/me/permissions` e persistencia cifrada somente quando todos estiverem `granted`. O
System User token ficou como fallback manual avancado e agora passa pela mesma allowlist
de seis grants antes de descobrir ad accounts ou persistir qualquer snapshot; grant
ausente/negado falha com 422 sem refletir token nem corpo livre da Meta. O cadastro
existente em `meta-ads-assistant/.auth/client.json` é um cliente dinâmico do MCP,
não fornece `META_ADS_APP_ID`/`META_ADS_APP_SECRET` para a Marketing API first-party
e não prova os grants `pages_read_engagement` e `instagram_basic`. Ele permanece
compatibilidade read-only, mas não pode ser tratado como aprovação do app de
produto. Ainda falta localizar/configurar as credenciais do Meta App first-party,
cadastrar a redirect exata e concluir um OAuth real em staging; 2FA e consentimento
ficam para essa última homologação humana e não bloqueiam o restante do trabalho local.

## Critério objetivo de 100% funcional

O módulo só deve ser chamado de 100% quando os cenários abaixo passarem em staging
e produção controlada:

- duas contas usam o chat simultaneamente sem compartilhar conversa, OAuth, tool
  context, cache ou resultado;
- agência alterna entre “todos” e um cliente; cliente só enxerga seus recursos;
- voz e texto geram o mesmo tipo de proposta;
- posts e campanhas exibidos em cards correspondem a IDs retornados pelas fontes
  autoritativas, nunca a URLs/IDs inventados pelo modelo;
- confirmar duas vezes ou repetir após timeout não duplica nenhuma ação;
- toda criação nasce pausada e respeita o budget cap;
- conexão, refresh OAuth, sync, criação, duplicação, edição, pausa e retomada têm
  teste E2E e trilha de auditoria;
- API, migrations, secrets e volumes persistentes estão no deploy oficial;
- o runbook permite recuperar token expirado e execução `unknown` sem entrar em
  arquivos ou banco manualmente.

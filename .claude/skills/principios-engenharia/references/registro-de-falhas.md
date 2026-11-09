# Registro de falhas e aprendizados

Leia ao depurar um bug que parece familiar. Cada entrada é um erro real já resolvido + a regra que evita repeti-lo. Consultar isto antes de gastar tempo re-descobrindo uma causa-raiz conhecida. Os exemplos vêm de um projeto Go + Vue/Nuxt multi-tenant, mas as causas-raiz são gerais.

## Índice
1. Migrate no banco errado (duas instâncias, portas diferentes)
2. Panic no boot por conflito de rota (módulo novo × legado)
3. Porta do serviço perdida ao recriar container (config só em shell)
4. WorkspaceId novo não aparece no menu (falta wire em 3 arquivos)
5. Componente some para um papel (fonte de dados não chega àquele papel)
6. Select de UI explode com item de value vazio
7. Sync de API externa bloqueado por WAF (User-Agent default)
8. Visibilidade de account em dois caminhos que divergem (leitura × enforcement)
9. Campo "reverte" no reload (front não re-hidrata do banco)
10. Dep marcada como `external` no build quebra o bundle do browser (componente lazy em branco só em prod)
11. CSS `scoped`/`:deep` não alcança conteúdo teleportado (modal não troca de modo, resize não redimensiona)
12. Dev "lento normal do framework" era bind mount Windows→container (misdiagnóstico + paliativos que pioraram)
13. Deploy que embarca o working tree pulou passos manuais do runbook (produção caiu com 502)
14. Refresh de contexto zera o board da operação (normalizador sobre estado existente)
15. Metadados AppleDouble do macOS derrubam o painel inteiro

---

## 1. Migrate no banco errado
**Sintoma:** migrations aplicadas no banco errado. **Causa:** duas instâncias de Postgres na mesma máquina em portas diferentes; o `.env` apontava para a porta errada.
**Regra:** antes de qualquer `migrate up` local, verificar `echo $DATABASE_URL` — confirmar host/porta. Documentar a porta certa no doc do módulo de database.

## 2. Panic no boot por conflito de rota
**Sintoma:** app crasha no boot com "pattern ... conflicts with pattern ...". **Causa:** duas implementações concorrentes da mesma rota (módulo novo + legado) com shapes incompatíveis; o conflito só apareceu quando o módulo novo foi compilado junto.
**Regra:** antes de adicionar alias de uma rota nova, fazer grep do path no projeto inteiro — conflito de rota é detectado só no boot, não no build. Documentar o shape que o frontend espera antes de "modernizar" uma rota legada.

## 3. Porta do serviço perdida ao recriar container
**Sintoma:** "Failed to fetch" após rebuild; o serviço subiu numa porta diferente da esperada. **Causa:** config implícita via variável exportada em shell, sem persistência em arquivo — o rebuild criou um shell novo e perdeu a config, caindo no default.
**Regra:** toda variável que afeta network (`*_PORT`, `*_URL`, `*_BASE`) vive em arquivo `.env` versionado por `.env.example`, nunca em shell export. Marcar em "Notas de Deploy" quando uma variável nova precisa ser adicionada no servidor.

## 4. WorkspaceId novo não aparece no menu (falha silenciosa em 3 lugares)
**Sintoma:** página existe, build OK, mas o item não aparece no menu lateral — nenhum erro, nenhum log. **Causa:** o menu filtra cada item por um set de workspaces permitidos do papel; sem o id na lista de workspaces do papel, o item é silenciosamente filtrado. O type-checker não pega porque os ids são string.
**Regra:** ao adicionar um `workspaceId` novo, atualizar **3 lugares** juntos: (1) o array de definição de workspaces (id, label, ícone, path); (2) o mapa de acesso por papel (adicionar o id em cada papel que deve ter acesso, com as permissões de view/edit); (3) a config de navegação/menu com o mesmo `workspaceId`. Verificável: `grep` o id nos 3. Em review, confirmar o item visível no menu pelo papel-alvo antes de aprovar.

## 5. Componente some para um papel específico
**Sintoma:** um componente (ex.: faixa de ações) vem vazio para um papel operador, mas aparece normal para admin — parece bug de permissão de mutação. **Causa:** a fonte de dados do componente vinha só de um endpoint de GESTÃO, gateado por uma permissão que o papel operador não tem. Outra parte da tela funcionava porque lia de um endpoint que o papel PODE ler (um snapshot), mas esse snapshot não trazia os dados que o componente sumido precisava.
**Regra:** ao diagnosticar "componente sumiu", checar PRIMEIRO se a FONTE DE DADOS dele chega àquele papel (o gate de *fetch*), não só o `v-if` de render. Todo papel que pode OPERAR precisa dos dados pela via que ele já pode ler. E nunca colapsar `view == edit` para resolver UI — view não muta. Cuidado para não introduzir regressão de segurança liberando mutação com permissão só de leitura ao "consertar" a UI.

## 6. Select de UI explode com item de value vazio
**Sintoma:** tela dá erro 500 ("SelectItem must have a value prop that is not an empty string"). **Causa:** alguns componentes de select reservam o value vazio (`''`) para o estado "sem seleção" (placeholder) e proíbem um item com `value=""`. Um `<select>` HTML nativo aceita `value=""` numa boa, então componentes que usam o nativo não quebram — só os que usam o componente de UI.
**Regra:** em componentes de select da biblioteca de UI, nunca usar `value: ''` num item (placeholder "Todos"/"Selecione"). Usar um sentinela não-vazio (`'all'`, `'none'`) e converter para `''` na borda da query; ou usar a prop `placeholder` com `model-value` vazio e sem item-placeholder.

## 7. Sync de API externa bloqueado por WAF
**Sintoma:** chamada a uma API de terceiro retorna 406/403, mas a mesma URL responde 200 no `curl`. **Causa:** o cliente HTTP não setava User-Agent customizado; o default (ex.: `Go-http-client/1.1`) cai numa regra de WAF do servidor do terceiro. Até `Mozilla/5.0` genérico era bloqueado.
**Regra:** todo cliente HTTP que consome API de terceiro deve setar um `User-Agent` descritivo próprio (`MeuApp/1.0 (+url)`) + `Accept`. Não confiar no UA default. Ao integrar um cliente novo, testar o endpoint com `curl -A` antes — UA é a causa nº 1 de 403/406 em sync externo.

## 8. Visibilidade de account em dois caminhos que divergem
**Sintoma:** o switcher LISTA uma conta, mas ao usá-la a rota dá `403 not_member` — a tela "some" ao trocar de conta. **Causa:** duas fontes de verdade para "quais accounts o user acessa" — uma de LEITURA (alimenta a listagem) e uma de ENFORCEMENT (o middleware que valida o header em toda rota) — em pacotes diferentes. Mudar uma sem a outra cria drift silencioso (compila, passa nos testes do pacote).
**Regra:** ao mudar a regra de visibilidade/escopo de account, alterar OS DOIS caminhos juntos (leitura E enforcement do middleware). Um teste de contrato em cada lado documenta a regra. Antes de mover dado de tenant entre accounts, validar que o portão de membership aceita o novo dono.

## 9. Campo "reverte" no reload (front não re-hidrata do banco)
**Sintoma:** trocar um valor e recarregar a página parece não salvar — o campo volta ao valor antigo. Suspeita inicial recai no banco/back. **Causa:** o dado JÁ persistia certo; o bug era 100% de leitura no front. O draft de cada linha era semeado com `drafts[id] ?? createDraftFromStore(store)`. No reload, uma fonte parcial (contexto/cache, SEM o campo) chegava PRIMEIRO e semeava o draft com o default; quando o endpoint autoritativo chegava depois com o valor real, o `??` preservava o draft velho e ignorava o banco. Fonte parcial vencia a autoritativa.
**Regra:** ver o princípio inegociável "Fonte única de verdade no banco". O front nunca renderiza dado real a partir de fonte parcial/fallback; o draft re-hidrata do back assim que ele responde, e só se preserva enquanto há edição pendente (`touched`/`rowBusy`). Ao depurar "não salvou/reverteu", checar PRIMEIRO se o dado está no banco e se a API o devolve, antes de mexer no back.

## 10. Dep marcada como `external` quebra o bundle do browser (lazy component em branco só em prod)
**Sintoma:** um componente carregado via `defineAsyncComponent(() => import(...))` (ex.: modal de detalhe da task) abre EM BRANCO só em produção; em dev abre normal. Sem erro no servidor (rota SPA `ssr:false`), o chunk existe e é servido 200, todas as dependências existem. **Causa:** o build tinha `vite.build.rollupOptions.external: ['@tiptap/y-tiptap']` (uma sub-dep do editor, marcada external "pra não bundlar a colab Yjs"). `external` num build de **browser** deixa um bare specifier (`import"@tiptap/y-tiptap"`) no output; o navegador não resolve specifier nu sem import map → `TypeError: Failed to resolve module specifier` → o `import()` do chunk rejeita → o `defineAsyncComponent` (sem `errorComponent`) renderiza NADA. O board sobrevive porque é outra subárvore e não importa o editor. Em dev funciona porque `build.rollupOptions.external` NÃO se aplica ao dev server do Vite.
**Regra:** nunca usar `build.rollupOptions.external` para uma dep que vai rodar no browser — `external` é para builds Node/SSR, não para o bundle do client (não há resolução de bare specifier no navegador). Se a intenção é reduzir bundle, ou se deixa o Rollup bundlar (tree-shaking cuida do não-usado), ou se faz alias para um stub vazio — nunca external. Diagnóstico decisivo sem precisar de login: subir o preview do build de prod e, num browser headless, `await import('/_nuxt/<chunk>.js')` do chunk lazy — o erro de avaliação aparece na hora. E todo `defineAsyncComponent` importante deveria ter `errorComponent` para o erro não virar tela branca silenciosa.

## 11. CSS `scoped`/`:deep` não alcança conteúdo teleportado
**Sintoma:** um modal (`OmniEntityDrawer` sobre `USlideover`) parou de trocar de modo (lado/centro/tela-cheia) e o resize "só movia o fundo, não o painel" — os botões emitiam o evento e a classe (`--center`/`--side`) trocava no elemento certo, mas nada mudava visualmente. **Causa:** as regras de posição/tamanho do painel foram escritas em `<style scoped>` com `:deep(.painel--modo)`. O `USlideover` (Reka Dialog) **teleporta** o painel de conteúdo para o `body` via Portal — fora da subárvore do componente. Um seletor scoped compila para `[data-v-hash] .painel--modo`, que exige um ancestral com aquele `data-v` no destino do teleport; como o painel foi movido para o `body`, esse ancestral não existe e a regra **nunca casa**. A var de largura era setada no `:root` (e até herdada pelo painel), mas a regra que a **consumia** não aplicava. Funcionava antes porque essas regras viviam num stylesheet **global** (carregado no `nuxt.config` css array), e global casa com o teleportado. O markup do próprio template (header, menu, corpo) continuava estilizado porque cada elemento carrega o `data-v` nele mesmo — só o elemento criado pela lib (o painel) é que escapa.
**Regra:** CSS que mira um elemento **teleportado/portaled** por uma lib (painel de slideover/dialog, conteúdo de popover/tooltip) tem que ser **global** — `<style>` sem `scoped`, ou stylesheet global — nunca `:deep` de dentro de `<style scoped>`. Manter o escopo na prática prefixando com a classe-raiz do componente (ex.: `.omni-entity-drawer--center`). Estilo de elemento que o próprio template renderiza (mesmo que acabe dentro do painel teleportado) pode seguir `scoped`, pois o `data-v` está no elemento. Ao diagnosticar "a classe troca mas o CSS não aplica", checar PRIMEIRO se o alvo está sob um Teleport/Portal e se a regra é scoped.

## 12. Dev "lento normal do framework" era bind mount Windows→container
**Sintoma:** dev server (Nuxt/Vite) cada vez mais lento — troca de página em minutos, até o boot parar de terminar (porta recusava conexão). Diagnóstico repetido durante semanas: "compile on-demand do Vite, normal em dev". Paliativos (watcher em polling, warmup de todas as páginas no boot) **pioraram**. **Causa:** o fonte era bind mount de pasta do WINDOWS num container Linux (ponte 9P/gRPC-FUSE do Docker Desktop/WSL2) — cada stat/read ~100x mais lento, e um dev server toca milhares de arquivos. Os paliativos multiplicaram o I/O na ponte lenta (polling varrendo a árvore toda + warmup do grafo inteiro). O framework era inocente: em FS nativo os mesmos comandos sempre foram rápidos.
**Regra:** lentidão PATOLÓGICA em dev (minutos, não segundos) é infraestrutura até prova em contrário — medir ONDE o tempo é gasto (FS? CPU? rede?) antes de aceitar "é normal do framework". Nunca montar caminho Windows como bind mount de fonte de dev server; usar `docker compose watch` (código copiado no build + sync host→container, inotify real, sem polling). Nunca empilhar paliativos sem confirmar a causa raiz: paliativo sobre diagnóstico errado multiplica o problema. Armadilhas do compose watch: NÃO faz sync inicial (edição com watch desligado exige rebuild — por isso reconciliar com `up -d --build` antes de ligar o watch); `up -d` sozinho deixa o código congelado no último build; só 1 watch por projeto (lock exclusivo).

## 13. Deploy que embarca o working tree pulou passos manuais do runbook (produção caiu)
**Sintoma:** logo após um deploy "rápido" (build local do working tree, sem git/CI), o site inteiro devolve 502; na infra, a API está em crash-loop com erro de autenticação de banco (`28P01: password authentication failed`) e o frontend nem inicia (depends_on api healthy). **Causa:** o deploy embarcou uma mudança de infra pendente no working tree (role de banco least-privilege) cujo runbook exigia passos manuais no servidor ANTES da imagem nova (criar a role no Postgres + novas env vars no `.env` do servidor — env var nova nunca viaja na imagem). O compose interpolou senha vazia para uma role inexistente; o mock SCRAM do Postgres devolve "password authentication failed" mesmo quando a role NÃO EXISTE — o erro engana, parecendo senha errada.
**Regra:** antes de qualquer deploy que builda do working tree, conferir as "Notas de Deploy" do doc canônico por passos manuais pendentes (env var nova, role/grant de banco, script a copiar) — esse tipo de deploy embarca TUDO, inclusive o que não está pronto para o servidor. Preferir auto-provisão idempotente no migrate/startup a passo manual de banco. Diagnóstico: `28P01` logo após deploy = checar primeiro se a role e as envs existem no ambiente (o "password failed" pode ser role ausente), antes de suspeitar do código.

## 14. Refresh de contexto ZERA o board da operação (spread de normalizador sobre estado existente)
**Sintoma:** na visão de loja operável, o board inteiro (fila, atendimentos, finalizados, pendências) fica vazio durante a janela do countdown de auto-encerramento e "volta ao normal" sozinho no fechamento; chips do nav zerados; NENHUM erro no console e NENHUMA request no instante do zeramento. Em outros momentos, só um flash ("esvazia e volta rapidinho"). **Causa:** `context.updated` (ex.: alerta materializado no mesmo tick do sweep) → `refreshContextState` → `auth.fetchContext` → `syncRuntimeAccess` → `hydrateRuntimeStoreContext` com `canFetchQueueSettings()=false` (conta ATIVA = agência; também conta sem o módulo queue) → `fetchRemoteStoreData` PULA settings+consultants+snapshot → `operationsSnapshot=null` → `applyRemoteStoreData` espalhava `normalizeOperationSnapshot(null)` (todas as listas VAZIAS) POR CIMA do snapshot existente e ainda carimbava `_operationSnapshotFetchedAt` novo. "Não busquei" virou "apaguei", sem request e sem log. A corrida com o refetch legítimo do WS decide o sintoma: refetch depois = flash; refetch antes = vazio persistente até o próximo evento.
**Regra:** normalizador de payload NUNCA entra em spread quando o payload é null/ausente — "não buscado" PRESERVA o estado existente, não sobrescreve com default vazio (é o princípio da fonte única: default não pode vencer dado real). Carimbo de frescor (`fetchedAt`) só avança quando houve fetch de verdade. Diagnóstico decisivo para "estado zera sem motivo": Playwright headless (venv do qa-bot) logado + probe do Pinia via `page.evaluate` + captura de `page.on('response')` — cruzar "o que a API devolveu" × "o que o estado virou" no MESMO instante; rede vazia no instante da escrita = escrita local (procurar quem aplica default), payload cheio + estado vazio = corrida de escritas.


## 15. Metadados AppleDouble do macOS derrubam o painel inteiro

**Incidente local em 2026-11-03.** Alterações em componentes do Cardápio coincidiram com tela branca em todo o painel. O console mostrou 502 para CSS globais (`omni-design-system.css`, `components.css`, `calendar.css`, `presentation.css`), entrada JavaScript do Nuxt, cliente Vite e favicon. Os 502 eram consequência da queda do servidor web, não prova de regressão em cada um desses arquivos.

**Causa comprovada:** dez arquivos binários AppleDouble entraram no container junto dos fontes, incluindo `._CardapioProductModal.vue`, `._CardapioSectionProdutos.vue`, `._OmniTagsInput.vue` e `._OmniCollectionFilters.vue`, além de seis `._*.ts`. O prefixo `._` identifica metadados do macOS; a extensão `.vue` fez o scanner tentar tratá-los como código. O Tailwind/Oxide 4.3.0 abortou em `crates/oxide/src/extractor/pre_processors/vue.rs:17:59` com `Utf8Error { valid_up_to: 45, error_len: Some(1) }`, seguido de `fatal runtime error`/`Aborted`. O proxy ficou sem upstream para atender os assets. O compilador de estilos é compartilhado pelo painel: um arquivo contaminante em um módulo pode derrubar todos.

**Prevenção obrigatória ao copiar, sincronizar ou empacotar fontes no Mac:**

- Preservar `._*` no `.gitignore`, `**/._*` e `**/.DS_Store` em `web/.dockerignore` e nas exclusões do Compose watch. Git, build e sincronização são caminhos distintos; proteger um deles não protege os demais.
- Preservar `@source not '../../../**/._*';` em `web/app/assets/styles/omni-design-system.css`, relativo ao stylesheet, para excluir AppleDouble do scanner Tailwind mesmo se chegarem ao runtime.
- Ao criar tar no macOS, definir `COPYFILE_DISABLE=1` no comando de criação e conferir o inventário antes de extrair. Cópia manual também precisa excluir metadados; não presumir que Git ou Docker ignore filtram um tar ou um `docker cp` arbitrário.
- Exclusões novas não removem arquivos já presentes no container. Um Compose watch já aberto precisa ser reaberto para carregar mudanças de configuração. Não fazer rebuild completo apenas para validar uma limpeza de metadados.

**Diagnóstico e recuperação:** consultar logs do web antes de atribuir o problema ao módulo editado. Procurar `Utf8Error`, `vue.rs`, `panicked` e `Aborted`; identificar `._*` no container, não só no host. Neste incidente, os fontes reais do host eram UTF-8 válidos, mas havia metadados binários no runtime. Confirmar AppleDouble pelo cabeçalho hexadecimal `00 05 16 07` e remover somente esses metadados. Preservar fontes reais, volumes, senhas e sessões; não apagar caches ou trocar dependências como tentativa sem evidência. Se o processo não se recuperar após a limpeza, reiniciar somente o serviço web local, respeitando a orientação do usuário. Nunca reiniciar o Docker Desktop ou encerrar seus processos internos por iniciativa própria.

**Validação:** a exclusão foi reconhecida pelo compilador instalado, e um teste isolado do scanner leu um Vue válido e ignorou um AppleDouble binário sem panic. Compose e formatação passaram. Isso não comprova recuperação do painel: HTTP 200 apenas no HTML também não basta. Conferir os CSS e JavaScript da entrada pelo proxy e a ausência de novos crashes; realizar validação visual somente com autorização. Nesta sessão, a recuperação HTTP final ficou pendente e o usuário optou por reiniciar o Docker manualmente. Não registrar o painel como recuperado sem essa confirmação.

**Regra de aprendizado:** ao resolver uma falha real com risco de repetição, registrar sintoma, causa comprovada, prevenção e validação (incluindo pendências). Separar hipóteses de evidência; não transformar coincidência temporal em culpa de um módulo, nem prometer que uma regra documental elimina todo risco futuro. Runbook do ambiente: `docs/SETUP_MAC.md`.


## 16. Deploy trava em diretório novo ou transferência interrompida

**Evidência em 2026-11-04:** a validação local já recebeu correção para criar diretórios pais antes de `docker cp`; fontes novos podem ter pastas ausentes na imagem anterior. No terminal apresentado, o empacotamento e o snapshot remoto passaram, o diretório das imagens foi preparado e o SCP parou aos 34% com `Timeout, server not responding` e `lost connection`. Não há evidência suficiente para atribuir o timeout a rede, SSH ou carga da VPS especificamente.

**Correção:** preservar a preparação idempotente de diretórios; substituir o envio monolítico das imagens por SFTP com `reput`, parcial por tag/hash, até três tentativas e SHA256 obrigatório antes de promover/carregar. Arquivo local permanece disponível para reutilizar sem rebuild; o cliente SFTP é verificado antes do build no fluxo rápido. Não alterar timeout ou reiniciar serviços sem diagnóstico.

**Validação:** teste isolado de controle de fluxo com transporte simulado cobre sucesso, falha seguida de retomada, três falhas consecutivas e propagação de erro na etapa de integridade. Sintaxe e launcher são validados localmente. Rede real, SFTP remoto e rollout não são comprovados por esses testes. Procedimento e limites em [deploy.md](deploy.md#preparação-e-transporte-do-deploy-incidentes-de-2026-11-04).

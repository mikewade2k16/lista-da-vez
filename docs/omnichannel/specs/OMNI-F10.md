# OMNI-F10 — Telas de config

**Prioridade:** P0 · **Plano canônico:** `docs/omnichannel/PLANO_ATENDIMENTO.md` (§9.2 F10, §5.3, §5.4, §12 risco 2)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**, canônico §2). O aviso de congelamento que constava aqui não vale mais.

Ler `principios-engenharia` (+ `references/frontend.md`) antes de qualquer coisa.

---

## Objetivo

O operador configura o módulo **inteiro pelo painel, sem tocar no banco**: cadastra um número
escolhendo o provider e gravando a credencial cifrada, monta setores/filas/regras de roteamento, e
edita/publica/reverte a versão do agente de IA — testando o agente num simulador antes de publicar.
A UI **degrada por número** conforme `Capabilities()`: nunca oferece o que aquele provider não faz.

## Depende de / Bloqueia

| | Fases |
|---|---|
| **Depende de** | **F4** (`ChannelProvider`, `Capabilities()`, adapters, "um número = um cérebro") · **F8** (`departments`/`queues`/`queue_members`/`routing_rules`) · **F9** (`ai_agents`/`ai_agent_versions`/`ai_runs`) · **F3** (`platform/secretbox`) · **F1** (rota `/omnichannel` registrada e demo removido) |
| **Bloqueia** | Piloto P0 (`F0 → F10 + F13-mínimo`). Nada depende da F10. |

## Escopo — o que é da F10 e o que não é

| É da F10 | NÃO é da F10 (dono) |
|---|---|
| Superfície HTTP de config de **números/providers/credenciais** (nenhuma fase anterior a entrega) | Tabelas e migrations de tudo (F2/F4/F8/F9) |
| **Telas** de setores/filas/regras e do agente | As **rotas** dessas entidades: `/settings/*` é da **F8** (Contrato 6) · `/agents/*` é da **F9** (C9.5) — a F10 **consome** |
| **Tela** do simulador (mostrar o traço) | Rota `POST /agents/{id}/simulate` e seu contrato (**F9**, C9.7) · motor de roteamento e triagem em runtime (F8/F9) |
| Telas novas no **design system da casa** | Inbox verbatim (F1) — não se toca |
| Consumo de `Capabilities()` | **Definição** da interface `ChannelProvider` (F4, canônico §5.4) |
| Uso do `platform/secretbox` | Implementação do secretbox (F3) |

> **Regra:** se F4/F8/F9 já entregaram uma rota de escrita, a F10 **consome** — não recria.
> Duplicar contrato é criar duas verdades (princípio 1). Confirmar o que já existe antes de escrever.
>
> **Aplicação da regra (reconciliado 2026-07-16):** uma versão anterior desta spec violava a própria
> regra — re-tabelava `/departments`, `/queues`, `/routing-rules` e `/ai-agents` com paths, verbos e
> bodies **diferentes** dos que a F8 e a F9 já definiam. Os contratos válidos são os **da fase dona**;
> as divergências de substância foram resolvidas lá, não aqui: membros da fila = `POST`/`DELETE`
> incremental (F8, Contrato 6) e `simulate` = `{versionId?, messages[]}` (F9, C9.7).

---

## Entregas

### Back — `back/internal/modules/omnichannel/`

| # | Item | Arquivo alvo |
|---|---|---|
| 1 | Rotas de config de números/providers + credenciais | `http_config_instances.go` |
| 2 | Serviço: limite `max_whatsapp_numbers`, conflito "um número = um cérebro", cifragem via `secretbox` | `service_config_instances.go` |
| 3 | Repositório (filtra por `account_id` **também** na query) | `store_postgres_config_instances.go` |

**É só isso no back.** Setores/filas/membros/regras/ordem = **F8** (`http_domain.go`, Contrato 6);
agentes/versões/publish/rollback/collect-fields/simulate/runs = **F9** (`http_ai.go`, C9.5/C9.7).
Não existem `http_config_routing.go` nem `http_config_agents.go` — seriam a segunda verdade que a
regra do Escopo proíbe. Se ao chegar aqui faltar alguma rota, ela **nasce na fase dona**, não numa
cópia local.

Padrão da casa **confirmado no disco** (`calendar/http_secrets.go:21-33`): `RegisterXRoutes(mux, svc, middleware)`
chamado do `module.go`; `middleware.RequireAuthWithAccount(h)` para tudo que é account-scoped;
`accountScope(r)` extrai o id do Principal. Teto ~450 linhas/arquivo **vale** (código novo).

### Front — `web/app/components/omnichannel/config/` (código **da casa**, não verbatim)

| # | Componente | Papel |
|---|---|---|
| 1 | `OmnichannelConfigDrawer.vue` | Host com abas + deep-link `?config=<aba>` |
| 2 | `ConfigNumbers.vue` | Lista/CRUD de números, seleção de provider, badge de capabilities |
| 3 | `ConfigNumberCredentials.vue` | Credencial write-only `{set,last4}` |
| 4 | `ConfigDepartments.vue` · `ConfigQueues.vue` | Setores, filas e membros da fila |
| 5 | `ConfigRoutingRules.vue` | Regras + reordenação de prioridade |
| 6 | `ConfigAiAgent.vue` · `ConfigAiAgentVersions.vue` | Editor + publish/rollback |
| 7 | `ConfigAiAgentSimulator.vue` | Simulador mínimo |
| 8 | `web/app/domain/omnichannel/config-api.ts` | Client tipado (sem `any`) |

**Precedente a espelhar (verificado):** `web/app/components/calendar/config/CalendarConfigDrawer.vue` —
`OmniEntityDrawer` + abas + rascunho `draft`/`touched` que **re-hidrata da resposta do back**, footer
"Salvar" por aba. `ConfigNumberCredentials.vue` é o espelho de `ConfigAiKeys.vue` (mesma mecânica:
`type="password"`, PUT write-only, vazio = limpar, re-lê o status mascarado após gravar).
Componentes de UI existentes: `AppPanelButton`, `AppSelectField`, `AppToggleSwitch`, `AppSearchInput`,
`OmniEntityDrawer` (`web/app/components/ui/`). Toasts via `useUiStore` (`~/stores/ui`, **já existe**).

---

## Contratos

Prefixo `/v1/omnichannel` (dentro do gate de módulo — canônico §11). `account_id` **sempre** do
Principal. JSON camelCase.

### Números / providers — permissão `omnichannel.instances.manage`

| Método | Rota | Nota |
|---|---|---|
| `GET` | `/whatsapp/instances` | **Já é da F2** — consumir (`SPECS_PORT_OMNICHANNEL.md` F2.3) |
| `POST` | `/whatsapp/instances` | Cria; body traz `provider` + `providerConfig`. **409** se estourar `max_whatsapp_numbers` ou se o número já estiver cadastrado em outra instância da conta (validação **interna** — F4, C6) |
| `PATCH` | `/whatsapp/instances/{id}` | Renomear / `providerConfig` |
| `DELETE` | `/whatsapp/instances/{id}` | |
| `PUT` | `/whatsapp/instances/{id}/credentials` | **Write-only.** Cifra via `platform/secretbox` → `credentials_ciphertext`. Vazio = limpar. Responde **só** `{set,last4}` |
| `GET` | `/whatsapp/instances/{id}/capabilities` | Projeção de `Capabilities()` do adapter daquele número |

`provider` ∈ `meta_whatsapp_cloud | evolution | waha | mock` (CHECK nasce na F2 — canônico §7.2).
Provider fora do enum → **422** com erro acionável.

### Setores / filas / regras — **rotas da F8** (`OMNI-F8.md`, Contrato 6) · permissão `omnichannel.settings.manage`

A F10 **consome, não recria**. Paths definitivos, **com o prefixo `/settings`**:
`/settings/departments`, `/settings/queues`, `/settings/queues/{id}/members`,
`/settings/routing-rules` e `PUT /settings/routing-rules/order`. Verbos, bodies e os 404 de escopo
estão na F8 — **não repetidos aqui** (princípio 1).

O que é **da tela**:

- **Membros da fila = incremental** (`POST` / `DELETE .../members/{userId}`). A F8 decidiu contra o
  `PUT` de conjunto completo, que faz *lost update* silencioso entre dois supervisores. A tela guarda
  a seleção, faz o **diff** contra o que veio do back e dispara as chamadas; se uma falhar, mostra
  **quem entrou e quem falhou** — nunca um "salvou" mudo (armadilha 7).
- **Remover setor/fila = soft** (`is_active=false`, F8): a conversa que já está na fila **continua
  visível** (princípio 3). A tela diz "desativado" e não promete apagar — não existe o 409 de
  "conversa viva" que uma versão anterior desta spec inventava.
- **Reordenar regras** usa `PUT /settings/routing-rules/order` (uma transação, tudo ou nada) — nunca
  N `PATCH` de `priority`, que deixa a ordem inconsistente no meio do caminho.
- `queue_members` **é o gate de dado** (canônico §5.2): mexer nele muda o que o atendente enxerga —
  ao remover alguém, a tela avisa que ele **perde acesso às conversas daquela fila**.

### Agente de IA — **rotas da F9** (`OMNI-F9.md`, C9.5) · permissão `omnichannel.agents.manage`

A F10 **consome, não recria**. Paths definitivos: `/agents`, `/agents/{id}`, `/agents/{id}/versions`,
`POST /agents/{id}/versions/{v}/publish` (**versão no path**), `POST /agents/{id}/rollback`
(`{versionId}`), `/agents/{id}/collect-fields`, `POST /agents/{id}/simulate` e
`GET /agents/{id}/runs`. **Não existe `/ai-agents/*`** — o path casa com a permission key
`omnichannel.agents.manage` do canônico §5.2.

**Simulador — contrato em `OMNI-F9.md` C9.7** (body `{versionId?, messages[]}`, resposta com o
traço). Já decidido lá e a F10 **não re-decide**: a simulação **grava `ai_runs` e consome o limite
mensal** — chama o modelo de verdade, e o custo tem de aparecer na F13. Limite estourado → **409**
acionável.

O que é **da tela**:

- Manda `messages[]` (histórico), não uma mensagem só: é o que exercita a camada 7 do prompt.
- `versionId` ausente = versão publicada → a tela envia o `versionId` do **rascunho** para testar
  **antes de publicar**. É o motivo de o simulador existir.
- Mostra o traço: `valid` / `validationErrors`, `extractedFields`, `matchedRule` e `wouldRoute` — é
  isso que prova "**IA sugere, motor decide**" para quem está configurando.
- A simulação **não envia mensagem**: a tela nunca insinua que o cliente recebeu algo.

### Capabilities — a UI degrada por número (canônico §12, risco 2)

A F10 **consome** o que a F4 definir. Regra de tela, independente do shape final:

| Capability ausente no número | A UI faz |
|---|---|
| Templates / janela 24h (não-oficial) | Não mostra seletor de template |
| Janela 24h ativa e **expirada** (Cloud) | Campo de texto livre bloqueado + "fora da janela de 24h: use um template" |
| Reação / sticker | Esconde o controle — **não** oferece e falha depois |

**Nunca** presumir capability por provider cravado no front: ler do número. Capability desconhecida
= tratar como **ausente** (degradar), nunca como presente.

### Migrations

**A F10 não cria migration.** Todas as tabelas vêm de F2/F4/F8/F9. Se faltar coluna, ela pertence à
fase dona — **não** criar uma 0200 aqui para remendar. Caso, e só caso, uma seja mesmo inevitável:
a partir de **0200** (última no disco: `0199_calendar_drop_day_media.sql`; **há dois arquivos 0197** —
conferir o disco, a numeração não é validada por ninguém), SQL plano idempotente, schema-qualificado,
**sem `-- +goose Down`**, `account_id uuid NOT NULL REFERENCES core.accounts(id)`.

---

## Armadilhas / o que NÃO fazer

| # | Armadilha | Regra |
|---|---|---|
| 1 | **Hex hardcoded / token inexistente** | Só tokens de `web/app/assets/styles/omni-tokens.css` (`--surface`, `--border`, `--text`, `--muted`, `--primary`, `--danger`, `--radius-*`). Caso real: `var(--color-primary, #16a34a)` — nome inexistente cai no **fallback silencioso** e o componente ignora o dark mode. Conferir o nome no arquivo antes de usar |
| 2 | **`platform_admin` tem `has()` = false no front** | Todo gate de aba/seção precisa de `isPlatformAdmin \|\| has('omnichannel.<x>.manage')` — senão a config some justamente para quem administra |
| 3 | **Pilha vertical em aba de config** | Regra OBRIGATÓRIA de accordion: `<details class="settings-collapse">`, **todos fechados** (sem `open`), um por categoria, `__meta` com resumo (contagem/estado). Classes já existem em `web/app/assets/styles/components/settings.css` — **reutilizar, nunca editar** |
| 4 | **Rascunho que não re-hidrata** | `draft` re-hidrata da resposta do back assim que chega; só preserva sob `touched`. Nunca "achar que salvou" (princípio 1). Trocar de escopo/número **descarta** rascunho de credencial (precedente: `ConfigAiKeys.vue:84-90` — senão a chave da conta A vai parar na B) |
| 5 | **Chave crua no front** | Credencial **nunca** volta do back, **nunca** em log, **nunca** em `localStorage`. Só `{set,last4}` |
| 6 | **Dropdown à mão** | Fecha no clique-fora (`pointerdown` + `contains`) **e** no `Esc`, listeners removidos no unmount |
| 7 | **Botão morto** | Marcar obrigatório/opcional e **dizer o que falta** quando o submit trava. Erro diz o que fazer, não só o que quebrou. Limite estourado → 409 com texto acionável, nunca falha muda |
| 8 | **Misturar com o verbatim** | `components/omnichannel/config/**` é código **da casa**: ESLint `max-lines` vale aqui (nos arquivos verbatim do inbox, não — é violação consciente, alvo F14). Não reformatar nada do verbatim |
| 9 | **Rota-pai engole a filha** | Se optar por **página** em vez do drawer: `pages/omnichannel.vue` (demo) tem que ter sido removido na F1.5, senão `pages/omnichannel/config.vue` vira rota-filha e não renderiza. O **drawer evita isso** — e é o precedente da casa (o calendário trocou `/calendario/config` por drawer) |
| 10 | **SSR** | `web/nuxt.config.ts:58` tem `'/omnichannel': { ssr: false }`; a F1 deve ter somado `'/omnichannel/**'`. Conferir antes de culpar o componente |
| 11 | **Store de layer não é auto-importada** | Import explícito de store de `web/layers/*`; HTTP 200 sem token não prova render autenticado |
| 12 | **Bundle stale no dev** | "Mudei o CSS e não mudou" costuma ser bundle velho do container — conferir a versão **servida**, não só o disco |

---

## Segurança

| Item | Regra |
|---|---|
| **Escopo** | `account_id` **sempre** do Principal (`accountScope(r)`), **nunca** do body. Ignorar qualquer `accountId` que venha no payload |
| **Defesa em profundidade** | O repositório filtra por `account_id` **também**, mesmo o service já tendo validado |
| **Fora de escopo** | **404, NUNCA 403** (403 vs 404 vaza que o recurso existe — enumeration) |
| **Permissão ≠ escopo** | Falta de permissão → **403**; recurso de outra conta → **404**. Precedente: `SPECS_PORT_OMNICHANNEL.md` F5 ("VIEWER → 403 — é permissão, não escopo") |
| **Credenciais** | Rotas de credencial usam `middleware.RequireAuthWithAccount` — valida **membership** no `X-Account-Id` (`core.account_users`, com bypass de `platform_admin`). Sem isso, usuário da conta A manda `X-Account-Id: B` e lê/grava a credencial da conta B. É o motivo documentado em `calendar/http_secrets.go:23-27`, e aqui a superfície é a mesma |
| **Cifragem** | Credencial **só** via `platform/secretbox` (AES-256-GCM, prefixo `v1:`). **Não** copiar o `calendar/secrets.go` inteiro: ele acerta o contrato de saída `{set,last4}` mas **grava a chave crua** — é o gap que a F3 existe para fechar (canônico §14.5) |
| **Log** | Nunca logar credencial, payload bruto de provider, nem o corpo do simulador |

---

## Verificável (prova no browser/banco)

1. **Ponta a ponta sem SQL:** logado, abrir a config e cadastrar **um número + um setor + uma fila +
   uma regra + um agente** — tudo pelo painel, **sem um `INSERT` na mão**. Recarregar a página: tudo
   voltou do banco.
2. **Credencial cifrada:** gravar a credencial → a resposta traz só `{set,last4}`. No banco,
   `select credentials_ciphertext from messaging.whatsapp_instances` → começa com **`v1:`** e é
   ilegível. A chave crua **não** aparece em nenhum log.
3. **Degradação por número:** número `evolution` → a UI **não** oferece template. Número
   `meta_whatsapp_cloud` fora da janela de 24h → a UI **exige** template.
4. **Limite:** com `{"max_whatsapp_numbers": 1}` em `core.account_modules.config`, cadastrar o
   segundo número → **409** com mensagem que diz o que fazer (não erro genérico).
5. **Um número, uma instância:** cadastrar na config um número já usado por **outra instância da
   mesma conta** → **409** acionável, nomeando a instância que já o usa.
6. **Isolamento:** `X-Account-Id` de outra conta num `PATCH /settings/departments/{id}` alheio →
   **404** (não 403). Repetir com o id de uma fila (`PATCH /settings/queues/{id}`) e de um agente
   (`GET /agents/{id}`).
7. **Publish/rollback:** publicar a v2 → o comportamento em runtime muda; `rollback` para a v1 →
   volta ao anterior. A versão ativa aparece na tela.
8. **Simulador não envia:** contar `messaging.messages` e `messaging.outbox` antes e depois de rodar
   o simulador → **os dois contadores não mudam**. A tela mostra a regra que casou e a fila destino.
9. **Tema:** com o drawer aberto, alternar claro/escuro → nenhuma cor fixa; tudo acompanha.
10. **Formulário:** submeter com campo obrigatório vazio → a tela **diz qual campo falta**; o botão
    nunca fica morto e mudo.
11. **Admin não perde a tela:** logar como `platform_admin` → todas as abas de config visíveis.

---

## Notas de Deploy

| # | Item | Detalhe |
|---|---|---|
| 1 | **Rebuild da API** | A F10 mexe em `back/` (rotas novas) → `docker compose up -d --build api` |
| 2 | **Migration** | **Nenhuma prevista.** Se por algum motivo entrar uma, ela exige `docker compose build --no-cache api` — as migrations são `embed.FS` e o cache da camada `go build` pode **não re-embutir** o `.sql` novo (sintoma: `migrate status` para na anterior, sem erro) |
| 3 | **`OMNI_SECRETS_KEY`** | Já obrigatória desde a **F3** (sem ela o módulo não sobe). A tela de credencial não funciona sem ela — não é env nova desta fase |
| 4 | **Front** | Sem env nova. Dev = `npm run dev` (compose watch) |

**Ordem:** rebuild api → rebuild web. Nada de container novo, nada de Caddy nesta fase.

---

## Referência cruzada

- Canônico → [`../PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§5.3 limites · §5.4 `Capabilities` · §7.2 colunas · §9.2 F10 · §12 risco 2)
- Contratos do front do inbox (não duplicados aqui) → [`../SPECS_PORT_OMNICHANNEL.md`](../SPECS_PORT_OMNICHANNEL.md) · [`../PLANO_PORT_OMNICHANNEL.md`](../PLANO_PORT_OMNICHANNEL.md)
- Precedentes no disco → `web/app/components/calendar/config/` · `web/app/components/settings/AGENT.md` · `back/internal/modules/calendar/http_secrets.go`
- Princípios → skill `principios-engenharia` (+ `references/frontend.md`)

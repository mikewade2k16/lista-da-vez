# OMNI-F0 — Decisões + fundação

**Prioridade:** P0 · **Plano canônico:** [`docs/omnichannel/PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§2, §4, §9.2, §14)

> ## LIBERADO PARA IMPLEMENTAÇÃO (2026-07-17, decisão do dono)
>
> A branch `refactor/multi-tenant-complete` fechou e o dono **liberou a implementação em
> 2026-07-17** (decisão **D-D**). O aviso de congelamento que constava aqui **não vale mais**.

Ler a skill `principios-engenharia` antes de executar. Não rodar git. Não commitar.
Devolver os comandos ao usuário.

---

## Objetivo

Tornar as **7 decisões do dono (D-A a D-G)** **citáveis e imutáveis**, publicar o plano canônico
e rebaixar o port a anexo técnico, marcar o legado do módulo e espelhar F0–F14 no roadmap. Quando
F0 fecha, ninguém precisa perguntar "qual provider?", "o front vem verbatim?", "a IA fica no
n8n?", "quem produz `PENDING`?" ou "o `idempotency_key` é global?" — está escrito, com data e
racional. **Zero linha de código de produto.**

## Depende de / Bloqueia

| | |
|---|---|
| **Depende de** | **Nada.** A branch `refactor/multi-tenant-complete` fechou e o dono liberou a implementação em **2026-07-17** (D-D): o blocker que o canônico §9.2 declarava **caiu** |
| **Bloqueia** | Formalmente **nenhuma**: F1/F2/F3 declaram `Blockers: nenhum` (§9.2) e podem correr em paralelo. Mas F0 é o que torna D-A a D-G citáveis — toda fase seguinte deriva delas |
| **Único artefato `.ts`** | Dados do roadmap (`phases-part7.ts`). É **dado**, não código de produto |

---

## Entregas

| # | Entrega | Alvo no disco |
|---|---|---|
| **E1** | **7 decisões** registradas com data: D-A multi-provider · D-B port=front/spec=backend · D-C LLM no Go (**2026-07-16**) · D-D liberação · D-E `PENDING` = 7º `state` · D-F D4 fora · D-G `idempotency_key` por conta (**2026-07-17**) | `docs/omnichannel/PLANO_ATENDIMENTO.md` §2 — D-A/D-B/D-C **já escritas**: verificar que constam, **não re-decidir**. **D-D a D-G entram no §2** — ver **E1.1** |
| **E2** | Publicar o canônico e **rebaixar o port a anexo técnico do front** | Cabeçalho de `PLANO_PORT_OMNICHANNEL.md` + `SPECS_PORT_OMNICHANNEL.md` (ver E2.1) |
| **E3** | Os **itens de legado** do §14 do canônico, com alvo de remoção | `docs/LEGADO.md` — arquivo **existe** (itens 1–6 + "Infra do princípio"); F0 **acrescenta**, não reescreve |
| **E4** | Espelho do roadmap: F0–F14, grupo e módulo na descrição da fusão | `web/app/components/roadmap/data/phases-part7.ts` · `groups.ts` (`id: "omnichannel-port"`, :70) · `modules.ts` (`id: "omnichannel"`, :71) |
| **E5** | Registrar **"um número = um cérebro"** como **norma operacional** | `docs/omnichannel/PLANO_ATENDIMENTO.md` §4 — **já escrito**. F0 **define**; a validação **interna** (mesmo número em duas instâncias do módulo) é **F4** |
| **E6** | Diretório de specs por fase existe e é referenciado pelo canônico | `docs/omnichannel/specs/OMNI-F*.md` |

### E1.1 — As 4 decisões de **2026-07-17** (do dono; registrar no canônico §2)

Vinculantes desde **2026-07-17**. Ninguém re-decide: quem discordar reabre **com o dono**.

| Decisão | O que ficou decidido | Consequência que as specs já têm de refletir |
|---|---|---|
| **D-D · Congelamento liberado** | A branch `refactor/multi-tenant-complete` fechou e a implementação está **liberada desde 2026-07-17** | O **aviso ativo** "IMPLEMENTAÇÃO CONGELADA" **sai de todos os documentos** — se ficar, o doc mente. O **registro histórico** de que houve congelamento permanece onde explica uma decisão passada |
| **D-E · `PENDING` = 7º `state`** | **Opção A** do Contrato 3.1 da F8: `pending` vira o **7º `state`** da máquina, com o **12º evento `human.pending`** (`PATCH /conversations/{id}/status` → `PENDING`), projetando `pending → PENDING`. Sai de `pending` por `msg.outbound.human`/`human.assign` (→ `human_active`) ou `conv.close` (→ `closed`); `msg.inbound` em `pending` = **`self`**. É **rótulo manual do operador** — sem produtor nem limpeza automática. `queued → PENDING` está **descartado com evidência**: `queued` é produzido pelo motor, e mapeá-lo trocaria "filtro sempre vazio" por "filtro sempre cheio". `PENDING` é **ortogonal** ao roteamento | Canônico **§7.2/§7.3** ganham a linha `pending → PENDING` · o **CHECK de `conversations.state` nasce com 7 valores** na **F2** (`new`, `ai_active`, `routing`, `queued`, `human_active`, `pending`, `closed`) — a F8 **não** faz `ALTER` · a matriz do Contrato 2 da F8 vira **7 × 12 = 84** pares (era 6 × 11 = 66), e os contadores da F8 acompanham · o **409 `invalid_transition` interino** de `PATCH status → PENDING` **deixa de existir**: agora é transição válida |
| **D-F · D4 (código morto do port) fica de fora** | `OmnichannelAuditModule.vue` + `useOmnichannelAudit.ts` **não são portados**: nunca renderizam nem no legado (as páginas que os chamariam redirecionam para fora). Não é remover funcionalidade — é **não importar código inalcançável** | **D4 sai de "aberta"/"pendente" e vira DECIDIDA: fora** · a **F1** deixa de ter esse blocker e os arquivos copiados byte a byte **caem 2** · bônus: era a única dependência de `~/components/docs/ProjectDocsModule.vue` |
| **D-G · `idempotency_key` por conta** | `unique (account_id, idempotency_key)`, **não** UNIQUE global. A chave vem do cliente: UNIQUE global deixa a conta A colidir com a chave da conta B e **suprimir o envio dela** — fere o princípio 2 (isolamento) | O **canônico §7.1 muda**: deixa de dizer "`idempotency_key UNIQUE`" global · a divergência que a **F3** já registrava **virou a norma** · onde alguma spec exigir **prefixar a chave com o `account_id`** como mitigação do UNIQUE global, isso vira **desnecessário** — remover |

### E2.1 — O que "rebaixar a anexo" significa (concreto)

Não apagar nada do port: ele continua sendo a fonte dos contratos verbatim do front. Marcar:

| Trecho do port | Marcação |
|---|---|
| Cabeçalho de `PLANO_PORT_OMNICHANNEL.md` e `SPECS_PORT_OMNICHANNEL.md` | "**Anexo técnico do front.** Canônico = `PLANO_ATENDIMENTO.md`" |
| `PLANO_PORT_OMNICHANNEL.md` §6 **D1** (Evolution single-provider) | **SUPERADA** por D-A (§2 do canônico) |
| `PLANO_PORT_OMNICHANNEL.md` §9 **F8** / `SPECS_PORT_OMNICHANNEL.md` F8 (IA no n8n) | **SUPERADA** por D-C — a IA vai para o Go |
| `PLANO_PORT_OMNICHANNEL.md` §6 **D3** (divisão de schemas) | **NÃO HERDADA** — o módulo mora em `messaging.*`, e ponto |
| `PLANO_PORT_OMNICHANNEL.md` §6 **D4** (código morto do audit) — hoje marcada "segue aberta" | **DECIDIDA: fora** por **D-F (2026-07-17)**. Não é mais decisão pendente |
| Numeração de fases do port (OMNI-F0..F9) | Ler pelo **mapa de renumeração** do canônico §9.1 |

D2 (mídia em disco + stream `Range`) **segue vigente** — o canônico a absorve, não a revoga.
**D4 não está mais aberta:** por **D-F (2026-07-17)**, `OmnichannelAuditModule.vue` e
`useOmnichannelAudit.ts` **não são portados** (código inalcançável, ver **E1.1**).

---

## Contratos

**F0 não tem contrato de código** — nenhuma rota, nenhuma migration, nenhuma interface Go.
O contrato abaixo é **normativo**: definição que as fases seguintes implementam.

### C1 — "Um número = um cérebro" (norma operacional; validação interna é F4)

| Item | Definição |
|---|---|
| **Regra** | Um mesmo número de WhatsApp é atendido por **um sistema só**. Não apontar o mesmo número para dois sistemas é responsabilidade de **quem opera** |
| **Por quê** | Duas respostas para o mesmo cliente é incidente **visível para o cliente final** |
| **O que o módulo valida** | Apenas o **interno**: o mesmo número não pode ser cadastrado em **duas instâncias do próprio `omnichannel`** — `UNIQUE` por conta, **no cadastro (F4)**. Falhar no cadastro é barato; falhar no runtime é o cliente vendo duas respostas |
| **Colisão com sistema externo** | **Não é gate de código** — o módulo não consulta outro módulo. É **risco operacional registrado** (canônico §12) |
| **Papel da F0** | **Só registrar a norma.** F0 não escreve validação — não escreve código |

---

## Armadilhas / o que NÃO fazer

| Não fazer | Por quê |
|---|---|
| **Re-decidir D-A a D-G** | D-A/D-B/D-C são vinculantes desde **2026-07-16**; D-D/D-E/D-F/D-G desde **2026-07-17**. Quem discordar reabre **com o dono**, não no código nem na spec |
| **Duplicar contrato do port** | Paginação `limit`+`beforeId`, os 3 shapes de `message.updated`, proteções do webhook, os 78 arquivos: vivem no anexo (`PLANO_PORT_OMNICHANNEL.md` §7/§8, `SPECS_PORT_OMNICHANNEL.md` F2/F4). **Remeter, não recopiar** — duplicar contrato é criar duas verdades (princípio 1) |
| **Escrever código de produto** | F0 é fundação. O único `.ts` permitido é **dado** do roadmap |
| **Apagar/reescrever o port** | Ele é a fonte dos contratos verbatim do front. Rebaixar ≠ deletar |
| **Tocar em outro módulo** | O `omnichannel` é **independente**: não lê, não escreve e não depende de outro módulo. Integração entre módulos está **fora deste plano** |
| **Presumir a numeração da próxima migration** | **Verificado no disco:** existem **dois `0197`** (`0197_operation_validation_reason.sql`, `0197_tools_module.sql`); a última é `0199_calendar_drop_day_media.sql`; **`0200` está livre**. Ninguém valida numeração — **conferir o disco** (vale da F2 em diante) |
| **Usar `-- +goose Down`** (F2+) | O migrator roda o arquivo **inteiro**; o bloco Down se auto-destrói. **Falha real:** `0143_automation_contacts.sql` criou e dropou a tabela no mesmo boot — corrigida por `0147_automation_contacts_fix.sql`. SQL **plano e idempotente** |
| **Esquecer que `platform_admin` tem `has()` = false no front** | Todo gating de menu/seção precisa de `isPlatformAdmin \|\| has(...)`, senão o módulo some justamente para quem administra (vale da F1 em diante) |

---

## Segurança

F0 **não abre superfície de ataque**: não há endpoint, migration nem código novo. O que ela
faz é **fixar como norma** o que as fases seguintes têm de obedecer — registrar aqui é o que
impede a F2/F4 de "esquecer":

| Invariante | Regra |
|---|---|
| **Escopo** | `account_id` **sempre** do Principal, **nunca** do body |
| **Defesa em profundidade** | O **repositório** filtra por conta **também** — não só o service, não só o front |
| **Fora de escopo** | **404, NUNCA 403** (403 vaza que o recurso existe — enumeration) |
| **Permissão × dado** | Permissão gateia **FEATURE**; fila gateia **DADO**. `conversations.view` não é "vê tudo": o atendente vê só as filas onde é `queue_member` + as atribuídas a ele (canônico §5.2) |
| **Segredos** | Só via `platform/secretbox` (F3). Nunca em coluna crua, nunca em log, nunca de volta pro front (só `{set,last4}`) |

---

## Verificável

Um humano prova que F0 fechou **sem rodar nada de produto**:

1. **Roadmap no browser** — abrir `/roadmap`, grupo "Omnichannel — Módulo de Atendimento
   WhatsApp": mostra **F0 a F14** (não OMNI-F0..F9 do port), todas `pending`; o card do módulo
   `omnichannel` descreve a fusão e cita `docs/omnichannel/PLANO_ATENDIMENTO.md`.
2. **`docs/LEGADO.md`** — lista os itens do §14 do canônico, cada um com alvo declarado
   (quase todos F14).
3. **Port marcado** — abrir `PLANO_PORT_OMNICHANNEL.md`: cabeçalho diz "anexo técnico",
   **D1 marcada SUPERADA** por D-A, **F8 SUPERADA** por D-C e **D4 marcada DECIDIDA: fora** por
   D-F (não pode continuar dizendo "segue aberta").
4. **Decisões citáveis** — `PLANO_ATENDIMENTO.md` §2 tem D-A/D-B/D-C **com data (2026-07-16)** e
   D-D/D-E/D-F/D-G **com data (2026-07-17)**, cada uma com racional.
5. **Zero código de produto** — o diff da F0 toca **só** `docs/**` e
   `web/app/components/roadmap/data/*.ts`. Nada em `back/`, nada em `automation/`, nada em
   `web/app/components/omnichannel/`. (O usuário confere o diff — **o agente não roda git**.)
6. **Sem aviso ativo de congelamento** — abrir os cabeçalhos de `docs/omnichannel/**` (canônico,
   port e `specs/OMNI-F*.md`): nenhum abre com o bloco de bloqueio; todos dizem **LIBERADO PARA
   IMPLEMENTAÇÃO (2026-07-17)**. O bloqueio caiu (D-D) e doc que ainda o anuncie como vigente está
   mentindo. **O que pode ficar** é a menção **histórica** (como a linha D-D da E1.1) — o que sai é
   o **aviso ativo**, não o registro do passado.

---

## Notas de Deploy

**Nenhuma.** F0 não tem migration, env var, container novo, nem dependência. Não exige
`docker compose up -d --build api` (não muda `back/`).

Para conferir o roadmap no browser em dev, o web já roda em `compose watch` — o dado do
roadmap é `.ts` estático e entra pelo HMR. **Não** rodar build de produção.

Registrado para as fases seguintes (não se aplica à F0):

| Item | Fase | Detalhe |
|---|---|---|
| Migrations `messaging.*` | F2+ | A partir de **0200** (livre, verificado). Idempotentes, **sem `-- +goose Down`** |
| `OMNI_SECRETS_KEY` | F3 | **Obrigatória — sem ela o módulo não sobe.** Fail-fast no boot |
| Rebuild da API | F2+ | Mudou `back/` → `docker compose up -d --build api` |
| **Migration nova** | F2+ | `docker compose build --no-cache api` — as migrations são **`embed.FS`** e o cache do `go build` pode **não re-embutir** o `.sql` novo. Sintoma: `migrate status` para na anterior, **sem erro** |

---

## Referência cruzada

- Canônico → [`../PLANO_ATENDIMENTO.md`](../PLANO_ATENDIMENTO.md) (§2 decisões · §4 fronteira · §9.2 fases · §14 legado)
- Anexo técnico do front → [`../PLANO_PORT_OMNICHANNEL.md`](../PLANO_PORT_OMNICHANNEL.md) · [`../SPECS_PORT_OMNICHANNEL.md`](../SPECS_PORT_OMNICHANNEL.md)
- Branch que **bloqueava** (fechada; liberação em 2026-07-17 — D-D) → [`../../MULTITENANT_COMPLETION_PLAN.md`](../../MULTITENANT_COMPLETION_PLAN.md)
- Princípios → [`../../ENGINEERING_PRINCIPLES.md`](../../ENGINEERING_PRINCIPLES.md) · skill `principios-engenharia`

# AC-15b — Testes de front, onda 2 (13 alvos + happy-dom pontual)

> Spec de implementação · Prioridade **P2** · Esforço **L** · Impacto **médio**
> Origem: AC-15 onda 2 (declarada na onda 1) · roadmap `ac-fixes-2026-07` → task `ac-15b-testes-front-onda-2`
> **Ordem recomendada:** DEPOIS da onda 2c do AC-07b (os stores-alvo viram cascas+slices lá;
> testes escritos contra a API pública sobrevivem, mas rodar depois evita conflito de arquivos).

## 1. Contexto

A onda 1 (AC-15) estabeleceu o harness: `web/vitest.config.ts` (environment `node`,
`fileParallelism: false`, `setupFiles: ./test/setup.ts` com mock global de `$fetch`/`useCookie`/
`useRuntimeConfig`/storage), padrão `*.test.ts` ao lado do arquivo, 22 arquivos de teste, 144
testes verdes, gate no CI. O próprio `vitest.config.ts:5-7` declara a fronteira: componentes/DOM
ficaram para "uma próxima rodada". Esta onda expande a cobertura UNIT (stores/composables/domain)
e introduz DOM leve (happy-dom) SÓ onde o alvo toca `document`/`window` — SEM `@nuxt/test-utils`
(mount de componente = onda 3).

## 2. Objetivo e não-objetivos

**Objetivo:** 13 alvos novos testados (lista §4.2), `happy-dom` instalado via container, tudo verde
no `vitest run`, contagem registrada no `web/AGENT.md`.

**Não-objetivos (FORA):**
- Componentes `.vue`/mount (onda 3, com `@nuxt/test-utils` — NÃO instalar agora).
- **Calendário: ZERO testes** (`app/stores/calendar.ts`, `app/utils/calendar*.ts`,
  `useCalendar*.ts`) — área em mutação ativa; testar agora congela contrato errado.
- `app/stores/erp.ts` (espera o split da onda 2c/2f).
- Alterar `vitest.config.ts` (environment default continua `node`).

## 3. Regras de execução (obrigatórias)

- NENHUM comando git. **`npm install` SÓ via container** (lockfile cross-platform — memória do
  projeto; instalar no host Windows QUEBRA o `npm ci` do container).
- Testes miram a API PÚBLICA (o que o store/composable retorna), nunca internals de slice — assim
  sobrevivem aos refactors do AC-07b.
- Padrão da onda 1: `*.test.ts` ao lado do alvo; `createPinia()+setActivePinia` no beforeEach para
  stores; mock de `$fetch` do `test/setup.ts`.
- Teste que exigir mudar código de produto = PARAR e reportar (nesta onda não se muda produto).
- Atualizar `web/AGENT.md` (contagem + regra calendário).

## 4. Mudanças (passo a passo)

### 4.1 Instalar happy-dom (via container, passo isolado)

```bash
docker compose run --rm web npm install -D happy-dom
# conferir: git diff web/package.json web/package-lock.json (só happy-dom entrou)
```

Uso POR ARQUIVO (só onde o alvo tocar `document`/`window`; vitest 2 suporta):

```ts
// @vitest-environment happy-dom
```

### 4.2 Os 13 alvos (1 arquivo de teste por alvo; casos mínimos por alvo:
estado inicial → ação feliz (fetch mockado) → erro de fetch (estado de erro, sem throw não tratado)
→ guarda multi-tenant/conta onde houver `accountId`/`X-Account-Id` no caminho)

| # | Alvo | Teste novo | Nota |
|---|---|---|---|
| 1 | `app/stores/operation-goals.ts` | `operation-goals.test.ts` | metas por loja: load/save/draft |
| 2 | `app/stores/consultants.ts` | `consultants.test.ts` | inclui criação com conta vinculada |
| 3 | `app/stores/reports.ts` | `reports.store.test.ts` | nome distinto do domain reports.test.ts |
| 4 | `app/stores/crm.ts` | `crm.test.ts` | |
| 5 | `app/stores/campaigns.ts` | `campaigns.store.test.ts` | domain campaigns.test.ts já existe |
| 6 | `app/stores/alerts.ts` | `alerts.test.ts` | ações resolve/dismiss mockadas |
| 7 | `app/stores/users.ts` | `users.test.ts` | convites/onboarding: shapes de payload |
| 8 | `app/stores/dashboard/runtime/state.ts` | `state.test.ts` | criação de estado/derivados puros |
| 9 | `app/stores/dashboard/runtime/status.ts` | `status.test.ts` | transições de status |
| 10 | `app/stores/dashboard/runtime/actions/settings-actions.ts` | `settings-actions.test.ts` | factory com deps fake |
| 11 | `app/composables/useCardapioProductForm.ts` | `useCardapioProductForm.test.ts` | validação/normalização do form (body COMPLETO no PATCH — memória) |
| 12 | `app/composables/useCardapioProductColumns.ts` | `useCardapioProductColumns.test.ts` | colunas visíveis/ordenação |
| 13 | `app/domain/utils/admin-metrics.ts` | `admin-metrics.test.ts` | funções puras de agregação (se a onda 2e já virou barril, testar via o barril) |

Para cada alvo: LER o arquivo primeiro; se um caso mínimo não se aplicar (ex.: alvo sem fetch),
substituir por 2 casos do comportamento central real. `// @vitest-environment happy-dom` apenas nos
que tocarem DOM (esperado: nenhum ou pouquíssimos desta lista).

### 4.3 Rodar e registrar

```bash
docker compose run --rm web npx vitest run
```

Atualizar `web/AGENT.md`: nova contagem (22 → 35 arquivos), regra "calendário sem testes até
estabilizar", nota do happy-dom por annotation.

## 5. Critérios de aceite

1. 13 arquivos de teste novos; `vitest run` 100% verde no container.
2. `web/package.json`/`package-lock.json` só ganharam `happy-dom` (dev); lockfile gerado NO container.
3. Zero mudança em código de produto e zero edição nos testes da onda 1.
4. Nenhum teste em área de calendário; `vitest.config.ts` intocado.
5. Cada teste roda isolado (`npx vitest run <arquivo>`) — sem dependência de ordem.

## 6. Validação

`docker compose run --rm web npx vitest run` (o comando é o critério). CI (`build-images.yml`)
continuará usando o mesmo gate.

## 7. Notas de Deploy

Nenhuma migration/env. `happy-dom` é devDependency — imagem de prod não muda (build multi-stage usa
`npm ci` mas o bundle final não embarca devDeps). Rebuild de imagem dev do web na próxima subida
(`npm run dev:watch` já faz o ensure-node-modules com o lock novo).

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| 13 × `*.test.ts` (tabela §4.2) | criar |
| `web/package.json` + `web/package-lock.json` | editar (happy-dom, via container) |
| `web/AGENT.md` | editar |

**Conflitos potenciais:** AC-07b-2c/2f-3 (mesmos stores) — rodar APÓS 2c; se 2f-3 ainda não rodou,
os testes de runtime/state valem contra o arquivo atual e continuam valendo após o split (API pública).

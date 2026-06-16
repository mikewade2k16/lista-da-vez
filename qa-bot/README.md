# QA Bot

Runner generico para testes manuais assistidos e smoke tests automaticos usando `Python + Playwright`.

## Objetivo

Deixar um robo reaproveitavel para varios apps, com cenarios externos em YAML e um runner unico.

## Estrutura

- `main.py`: CLI do runner
- `qa_bot/`: motor do bot
- `scenarios/`: cenarios declarativos
- `artifacts/`: screenshots e saidas geradas em runtime

## Setup

```bash
cd qa-bot
python -m venv .venv
.venv\Scripts\activate
python -m pip install -r requirements.txt
python -m playwright install chromium
```

## Rodando o primeiro cenario

Com o Nuxt no ar em `http://localhost:3000`:

```bash
cd qa-bot
python main.py scenarios/operation_smoke.yaml --base-url http://localhost:3000 --headed --slow-mo 250 --pause-before-close
```

Se quiser abrir em outra porta:

```bash
cd ..
npm run dev:3001
cd qa-bot
python main.py scenarios/operation_smoke.yaml --base-url http://localhost:3001 --headed --slow-mo 250 --pause-before-close
```

## O que o runner suporta hoje

- navegacao por rota
- clique
- preenchimento de input
- selecao de `select`
- `check` e `uncheck`
- esperas por visibilidade e invisibilidade
- assercao de texto
- assercao de URL
- limpeza de `localStorage` e `sessionStorage`
- screenshots

## Formato do cenario

Exemplo enxuto:

```yaml
id: exemplo-smoke
name: Smoke de exemplo
defaults:
  timeout_ms: 7000
  pause_after_step_ms: 150
steps:
  - action: goto
    path: /alguma-rota
  - action: expect_visible
    testid: algum-componente
  - action: click
    testid: algum-botao
  - action: fill
    target: input[name="email"]
    value: qa@example.com
```

Cada passo pode usar:

- `testid`: usa `data-testid`
- `target`: seletor CSS direto
- `path`: rota relativa ao `--base-url`
- `value`: valor da acao ou texto esperado
- `timeout_ms`: timeout especifico do passo

## Cenario inicial

O primeiro cenario pronto esta em [operation_smoke.yaml](c:/Users/Mike/Documents/Projects/fila-atendimento/qa-bot/scenarios/operation_smoke.yaml).

Ele cobre:

- limpar storage local
- abrir `/operacao`
- colocar consultores na fila
- iniciar atendimento fora da vez
- abrir o modal de fechamento
- preencher campos obrigatorios
- encerrar o atendimento
- verificar retorno do consultor para a fila

## Auditoria de performance (`perf_audit.py`)

Script dedicado (fora do runner de cenarios) que mede 3 marcos por rota — **T1** clique→troca de rota, **T2** clique→primeira pintura, **T3** clique→carregamento final — em 3 rodadas sem cache, nos modos **in-app** (navegacao SPA) e **cold** (1a visita), como `platform_admin`. Plano e resultados: [docs/PERFORMANCE_AUDIT_PLAN.md](c:/Users/Mike/Documents/Projects/fila-atendimento/docs/PERFORMANCE_AUDIT_PLAN.md).

Importante: medir contra **build de producao** (o dev compila rota sob demanda no Vite — 1a visita pode levar minutos, falseando a metrica). Subir o prod numa porta livre:

```bash
docker build -t omni-web-prod ./web
docker run --rm -d --name omni-web-prod -p 3055:3003 \
  -e NUXT_PUBLIC_API_BASE=http://localhost:9091 \
  -e NUXT_API_INTERNAL_BASE=http://host.docker.internal:9091 omni-web-prod
```

Rodar (credenciais por env, nunca versionadas):

```bash
OMNI_QA_EMAIL=... OMNI_QA_PASSWORD=... \
  .venv/Scripts/python.exe perf_audit.py --base-url http://localhost:3055 --runs 3
# flags: --only "/rota1,/rota2"  --modes inapp,cold  --warmup  --headed
```

Saida: `artifacts/perf-<timestamp>.{md,csv}` (tabela por rota×modo×rodada + media + ranking). No Windows, passar `--only` com rotas que comecam por `/` via PowerShell (o git-bash converte o path).

### Warm-up do dev (`warmup_dev.py`)

No modo dev o Vite compila cada rota sob demanda na 1a visita da sessao (pode levar segundos/minutos — medido: 203s na 1a, 0,07s na 2a). `warmup_dev.py` visita TODAS as rotas 1x **logo apos `docker compose up`**, deixando tudo compilado para a 1a navegacao do dia ser instantanea:

```bash
OMNI_QA_EMAIL=... OMNI_QA_PASSWORD=... \
  .venv/Scripts/python.exe warmup_dev.py --base-url http://localhost:3003
```

Imprime o tempo de cada rota (a 1a passada = custo de compile) e o ranking das mais lentas. Rode de novo e veja cair para ~0,1s — prova de que era compile, nao o app. E o jeito de matar a dor de "clico e demora" no dev.

## Observacoes

- hoje os testes ainda dependem do estado mock e de `localStorage`
- quando o backend Go entrar, o runner continua valido, mas os cenarios vao passar a validar tambem API e sincronizacao
- ja existem `data-testid` na `operacao` e na UI global para o bot ficar mais estavel

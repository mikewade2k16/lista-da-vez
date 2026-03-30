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

## Observacoes

- hoje os testes ainda dependem do estado mock e de `localStorage`
- quando o backend Go entrar, o runner continua valido, mas os cenarios vao passar a validar tambem API e sincronizacao
- ja existem `data-testid` na `operacao` e na UI global para o bot ficar mais estavel

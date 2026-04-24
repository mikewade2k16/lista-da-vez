# Campanhas e Corridinhas

## Objetivo

Separar dois tipos de iniciativa comercial:

- `Campanhas comerciais`: acoes de marketing do grupo ou da marca, como Dia das Maes, Prata Instagram, Troca do Ouro.
- `Corridinhas`: campanhas internas de incentivo para consultores, com metas, ranking e premiacao.

## O que ja funciona hoje

- Cadastro e edicao de campanhas em `/campanhas`.
- Regras automaticas por periodo, desfecho, origem, motivo, fora da vez e cliente recorrente.
- Aplicacao dessas regras no historico de atendimento.
- Filtro por campanha em relatorios.

## O que entra agora

- Campanhas comerciais podem vincular `produtos por codigo`.
- Quando um atendimento fecha com `productsClosed[].code` batendo em uma campanha comercial ativa, o historico recebe o match automaticamente.
- A Operacao ganha um painel com campanhas comerciais ativas, produtos e codigos para consulta do time.

## O que nao entra no modal agora

Para `campanhas comerciais`, o modal de fechamento nao precisa de novo campo manual.

Motivo:

- o dado nasce automaticamente pelo codigo do produto fechado;
- isso reduz atrito no fechamento;
- evita erro humano de marcar campanha errada.

## Impacto no operacional

### Fila e modal

- Nenhuma mudanca obrigatoria na fila.
- Nenhum campo extra obrigatorio no modal para campanha comercial.
- O modal continua sendo a fonte do produto fechado; a campanha e derivada do codigo do produto.

### Tela de Operacao

- Deve exibir campanhas comerciais ativas para consulta rapida.
- Deve mostrar ao menos:
  - nome da campanha;
  - periodo;
  - descricao ou regra resumida;
  - produtos vinculados com codigo.

### Relatorios

- Continuam usando `campaignMatches`.
- Agora o match pode vir tambem de `productCodes`, nao so de origem/motivo/regras gerais.

## Modelo recomendado

### 1. Campanhas comerciais

Campos minimos:

- `name`
- `description`
- `campaignType = comercial`
- `startsAt`
- `endsAt`
- `productCodes[]`
- `sourceIds[]` opcional
- `reasonIds[]` opcional
- `isActive`

Campos recomendados para proxima fase:

- `objectiveText`
- `rulesText`
- `scope = group | stores`
- `storeIds[]`
- `sitePublishedOnly`
- `assets / links`

Regra operacional:

- pode existir mais de uma ativa ao mesmo tempo;
- match automatico depende do produto vendido;
- origem e motivo continuam como filtros complementares, nao obrigatorios.

### 2. Corridinhas

Campos esperados para proxima fase:

- `name`
- `description`
- `campaignType = interna`
- `scope = group | stores`
- `storeIds[]`
- `startsAt`
- `endsAt`
- `goalType`
- `goalTarget`
- `rankingMode`
- `rewardRules[]`
- `isActive`

Exemplos de `goalType`:

- `sold-value`
- `average-ticket`
- `pa`
- `conversion-rate`
- `closed-services`

Regra operacional:

- pode existir uma corridinha geral e varias corridinhas por loja;
- pode haver mais de uma ativa ao mesmo tempo;
- precisa de leaderboard e premiacao customizavel;
- nao deve depender de preenchimento manual no modal para funcionar.

## Regras de escopo

- `Campanhas comerciais`: por padrao pensar em escopo de grupo, com possibilidade de restricao por loja.
- `Corridinhas`: podem ser de grupo ou especificas por loja.

## Recomendacao de produto

### Fase 1

- Campanhas comerciais por codigo de produto.
- Painel de campanhas ativas na Operacao.
- Sem mudar o modal.

### Fase 2

- Separar `/campanhas` em duas trilhas visuais:
  - `Campanhas`
  - `Corridinhas`
- Adicionar escopo por loja/grupo.
- Adicionar objetivo, regras e premiacao estruturada para corridinhas.

### Fase 3

- Criar area de avisos/notificacoes operacionais unificada.
- Misturar nessa area:
  - campanhas ativas;
  - metas;
  - alertas de performance;
  - corridinhas vigentes.

## Decisao atual

- `Campanhas comerciais`: entram no operacional como painel de consulta e match automatico por codigo.
- `Corridinhas`: ficam documentadas como proxima fase, sem obrigar mudanca agora no modal de fechamento.

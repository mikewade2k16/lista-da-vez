# Template de pacote atômico para agente executor

Copiar este conteúdo no prompt do executor e preencher todos os campos. Campo vazio mantém o
pacote em `DRAFT`.

```md
# Pacote <EFASE-AREA-NÚMERO> — <resultado único>

## Resultado obrigatório
<uma frase verificável; não usar “melhorar”, “ajustar” ou “avançar”>

## Contexto que você deve ler
1. AGENT.md
2. skills principios-engenharia e omnichannel-hibrido
3. docs/omnichannel/evolucao/CONTRATO_EXECUCAO_AGENTES.md
4. docs/omnichannel/evolucao/MATRIZ_DEPENDENCIAS.md
5. docs/omnichannel/evolucao/CATALOGO_DESPACHO_AGENTES.md — somente seção da fase
6. docs/omnichannel/evolucao/<SPEC_DA_FASE>.md — somente seção <PACOTE>
7. <AGENTs locais>

## Decisões já tomadas — não reabrir
- <decisão e razão>

## Pode ler
- <paths/globs exatos>

## Pode alterar/criar
- <paths/globs exatos>

## Não pode alterar
- qualquer arquivo fora da allowlist de escrita
- workflow-whatsapp.json, WAHA, Calendar, Operação e workflows de outro owner
- migrations já aplicadas
- <proibições específicas>

## Contratos de entrada
- <schema/API/tabela/fixture>

## Contratos de saída
- <schema/API/evento/UI>

## Passos obrigatórios
1. registre preflight e baseline;
2. implemente apenas o resultado obrigatório;
3. formate e rode validações do escopo;
4. prove todos os critérios de aceite;
5. entregue o handoff do contrato comum.

## Critérios de aceite
- [ ] <prova positiva>
- [ ] <duplicata/retry>
- [ ] <erro/fallback>
- [ ] <cross-tenant/permissão>
- [ ] <nenhum recurso fora do ownership alterado>

## Comandos mínimos
```text
<comandos exatos>
```

## Stop conditions
- pare sem editar se <condição objetiva>;
- pare sem ampliar escopo se <dependência ausente>;
- não improvise <decisão de produto/arquitetura>.

## Handoff
Use exatamente a seção 12 de CONTRATO_EXECUCAO_AGENTES.md.
```

## Como quebrar trabalho grande

- uma tarefa = um resultado primário;
- migration e backfill pesado são pacotes distintos;
- adapter de provider e wiring são pacotes distintos;
- contrato HTTP e UI podem ser pacotes distintos depois da fixture;
- workflow n8n e policy Go nunca ficam no mesmo executor;
- teste de integração/revisão é pacote separado do autor da implementação;
- se o diff previsto exceder ~8–12 arquivos ou misturar mais de duas camadas, dividir.

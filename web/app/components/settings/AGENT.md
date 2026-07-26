# AGENT - Settings Components

## Escopo

Componentes administrativos de configuracoes em `web/app/components/settings/`.

## Padrao atual

- `SettingsWorkspace.vue` hospeda somente as abas visiveis **Operacao**,
  **Modal**, **Motivos e cadastros** e **Gamificacao**.
- Workspace, cabecalho, aviso tenant-wide e tabs usam densidade compacta:
  paddings e gaps curtos, sem perder hover, foco ou responsividade.
- Secoes novas ficam em `web/app/components/settings/sections/`.
- `useSettingsWorkspace.js` concentra store actions, permissao, validacoes e
  helpers. Componentes de secao nao chamam stores diretamente.
- `settings-workspace-data.js` concentra tabs, configuracoes estaticas e listas
  de campos.
- Produtos e Consultores nao possuem aba visual na Config. O catalogo manual
  de produtos e o backend de consultores continuam preservados porque sao
  usados pela Operacao.
- `SettingsCrmGoalsSection.vue` e renderizado no bloco Metas de
  `MultiStoreWorkspace.vue`. A politica persiste por tenant e nao pertence mais
  a uma aba separada da Config.

## Layout das quatro abas

- `SettingsCatalogsSection.vue` reune motivos da visita, cancelamentos, pausas,
  perdas, fora da vez, origens e profissoes em um sidebar no desktop e
  navegacao horizontal em telas menores. Apenas o tipo ativo ocupa o painel.
- `SettingsReasonInputSection.vue` e `SettingsOptionTabSection.vue` exibem o
  comportamento do campo sempre aberto e inline. Cancelamento ocupa uma unica
  linha com cinco controles no desktop.
- `SettingsOptionManager.vue` exibe a lista cadastrada sem dropdown. O botao de
  adicionar aparece acima e abaixo do ultimo item, sempre com tooltip, e abre
  o cadastro inline no ponto acionado. CRUD e ordenacao continuam reais.
- `SettingsOperationSection.vue` mantem **Capacidade e fila**,
  **Tempos e alertas** e **Comportamento do atendimento** sempre abertos, com
  titulo/resumo na coluna esquerda e campos na mesma linha a direita no
  desktop. O Score 360 nao aparece nessa aba: sua superficie visual
  autoritativa e Gamificacao.
- `SettingsOperationTemplateManager.vue` e o unico bloco colapsavel da aba
  Operacao. Os cards ficam lado a lado. Aplicar template altera somente
  configuracoes operacionais e do modal e preserva os pesos do Score.
- `SettingsModalSection.vue` usa sidebar para **Fluxo**, **Campos e
  validacoes**, **Interesses** e **Textos**, exibindo somente um topico por vez.
  Dentro de Campos e Textos, os subgrupos sao accordions compactos fechados por
  padrao. Fluxo e Interesses mantem seus controles em linha no desktop.
- `SettingsScoreWeightsCard.vue` exibe as cinco metricas reais do Score 360 em
  uma linha no desktop: Conversao, Valor vendido, Qualidade, P.A. e Disciplina
  de fila. Remover persiste peso zero; adicionar reativa uma metrica suportada
  pelo calculo. Nao oferecer criterio arbitrario sem fonte, normalizacao e
  avaliador reais. O total alerta quando for diferente de 100%.
- `SettingsGamificationSection.vue` mantem pesos e badges abertos. Os cinco
  badges possuem IDs de avaliador fixos e aparecem em uma linha no desktop,
  com ativacao, titulo e Top N editaveis.
- Regra de densidade: varios topicos independentes na mesma pagina usam sidebar
  quando houver largura ou accordions quando a leitura for sequencial. Poucos
  controles relacionados ficam abertos e em linha. Evitar card dentro de card,
  padding duplicado e descricoes ocupando uma linha sozinha sem necessidade.
- A antiga aba Alertas nao e exibida. Seus thresholds legados permanecem no
  contrato por compatibilidade; a central autoritativa fica em `/alertas`.

## Regras locais

- Nao chamar stores diretamente dentro de componentes de secao.
- Manter cada secao abaixo de 500 linhas e preferir props simples via `ctx`.
- Preservar contratos de `settingsStore` e `consultantsStore`; mudancas de
  persistencia ficam no store.
- `SettingsCrmGoalsSection.vue` so habilita edicao quando
  `ctx.canEditCrmCommercialPolicy` for verdadeiro.
- Novos componentes de Score ou badges exigem fonte, normalizacao e avaliador
  reais antes de serem oferecidos na interface.
- Validar alteracoes em `/configuracoes` com mutacao reversivel; respeitar
  solicitacao explicita de nao usar Playwright.

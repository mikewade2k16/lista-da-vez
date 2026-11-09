# Auditoria do Finance — 2026-11-01

> Registro do estado anterior às correções. O andamento e as evidências atuais
> estão em [CORRECOES_FINANCE.md](CORRECOES_FINANCE.md).

Branch: `fix/finance`. Base: `5d6ef3b`. Skill: `principios-engenharia`.

## Resultado

O módulo não está 100% funcional. Há falhas de segurança, interface e persistência.
Esta auditoria não altera código funcional, contas, senhas nem planilhas existentes.
Nenhum commit, push ou deploy foi executado durante a auditoria.

## Método e limites

- Leitura dos contratos, handlers, serviços, stores, migration e composables.
- Inspeção do Finance e do painel de configuração no Chrome em localhost:3003,
  com sessão já autenticada. Nenhum campo financeiro foi editado.
- Conferência dos registros de componentes gerados pelo Nuxt no host e container.
- Quatro testes temporários sobre cópias sem alterações dos seis arquivos Go de
  domínio/persistência, usando a migration 0187 em PostgreSQL 16 descartável,
  separado do banco da aplicação, com duas contas sintéticas.
- Harness em `.tmp/finance-audit/`, ignorado pelo Git. O ambiente de testes não
  usa tokens, credenciais de usuários nem dados reais.
- Não foi executado ataque entre contas reais. O achado de autorização decorre
  do caminho completo de middleware e resolução de escopo no código.
- Não houve teste completo de cada perfil, dispositivo, concorrência ou fluxo de
  negócio. Não se deve extrapolar o CRUD básico para todo o módulo.

## Achados prioritários

### FIN-01 — P0: escopo da conta controlado pelo header sem membership

`back/internal/modules/finance/http.go:14` usa apenas `RequireAuth` e a função
`accountIDFromContext` em `:32` retorna diretamente `X-Account-Id`.
`RequireAuth` autentica, mas não valida membership da conta escolhida. O guard
em `back/internal/platform/httpapi/account_guard.go:118` verifica se o módulo
está habilitado na conta informada, não se o usuário pertence a ela.

Consequência: um usuário autenticado pode selecionar pelo header outra conta com
Finance habilitado. Os filtros SQL passam a filtrar pela conta forjada. CRUD e
configuração financeira ficam expostos a acesso entre contas por esse caminho.
A defesa correta já existe em `RequireAuthWithAccount`, mas não é usada aqui.
Preservar o comportamento autorizado de platform_admin na correção.

### FIN-02 — P1: permissões de leitor/gestor não são aplicadas por endpoint

`back/internal/modules/finance/module.go` registra o perfil `finance.viewer`,
mas `http.go:14` aplica o mesmo wrapper às rotas GET/POST/PUT/PATCH/DELETE.
Não há verificação de `finance.sheets.manage` ou `finance.config.manage` nas
mutações. Gating do menu não substitui autorização da API.

### FIN-03 — P1: componentes essenciais não resolvidos no frontend

`web/layers/finance/components/finance/FinanceLineCard.vue:139` e outros pontos
usam `OmniSwitchInput`, `OmniMoneyInput`, `OmniSelectMenuInput` e
`OmniMinimalPopover`, sem imports explícitos. O Nuxt registra esses arquivos como
`OmniInputsOmniSwitchInput`, `OmniInputsOmniMoneyInput`,
`InputsOmniSelectMenuInput` e `OmniOverlayOmniMinimalPopover`.

Confirmado no navegador: campos de valor/categoria e switch não aparecem como
controles utilizáveis, e o conteúdo de ajustes aparece diretamente nas linhas.
O painel de configuração também está sem seletores/valores desses componentes.
É causa direta do problema visual apresentado na imagem do usuário.

### FIN-04 — P1: datas preenchidas quebram a leitura após persistir

`back/internal/modules/finance/store_sheets.go:152` declara `effDate *string` e
passa `&effDate` ao pgx para uma coluna PostgreSQL `date`. Há o mesmo padrão em
ajustes e no PATCH. Teste com data de efetivação preenchida falhou com:

```
can't scan into dest[6] (col: effective_date):
cannot scan date (OID 1082) in binary format into **string
```

O Create/Update faz commit antes de chamar Get: pode salvar o dado e devolver
erro ao reler. Uma planilha com a data preenchida também pode prejudicar a
listagem, que carrega linhas para calcular os resumos. Usar tipo de data
compatível e formatar explicitamente no contrato JSON.

### FIN-05 — P1: recorrência mensal colide na chave primária de linhas

`web/layers/finance/utils/finance-ids.ts:60` gera ID determinístico somente pelo
cliente; a variante por loja também não inclui planilha ou mês. Entretanto,
`finance.lines.id` é PK global na migration e o store insere o ID recebido.

Teste criando a mesma recorrência em novembro e dezembro falhou com:

```
ERROR: duplicate key value violates unique constraint "lines_pkey"
(SQLSTATE 23505)
```

Separar identidade da recorrência da identidade da linha persistida por planilha.
O mesmo conflito pode ocorrer entre contas que usem a mesma origem de recorrência.

### FIN-06 — P1: composição de contas fixas não retorna corretamente

`back/internal/modules/finance/store_config.go:113` faz append no slice e guarda
ponteiros para seus elementos. Os próximos appends podem realocar o slice; o
preenchimento dos membros em `:138` passa a modificar elementos da alocação antiga.

Teste com duas contas fixas, uma composição em cada, confirmou que a primeira
retorna zero membros, apesar de os membros terem sido gravados. O front usa a
resposta para reidratar o draft e um salvamento posterior pode consolidar a perda.

### FIN-07 — P1: autosave pode perder edições ao navegar/trocar planilha

`useFinanceSheetEditor.ts:465` agenda o salvamento em 1200 ms. O callback chama
`persist()`, que lê a seleção e o draft naquele momento, sem capturar o contexto
original. O watcher da seleção em `:861` reidrata o draft sem salvar o anterior.
Em `:880`, sair da página apenas cancela o timer, sem flush ou aviso.

Por inspeção: editar A e selecionar B antes do debounce pode perder a edição de
A; sair antes do debounce descarta a alteração pendente. A configuração usa
padrão semelhante com debounce de 900 ms. Falta proteção de dirty state/contexto,
tratamento de navegação e testes com latência/troca rápida de conta e planilha.

### FIN-08 — P1: configurações atuais podem reescrever histórico financeiro

`useFinanceSheetEditor.ts:288` sincroniza linhas antigas com a configuração
atual: remove contas fixas ausentes e substitui nome/valor (`:313`). Recorrências
também recebem o valor atual. A página observa a configuração e chama
`syncAllFixedRows(true)`, que agenda gravação.

Não há bloqueio por período nem pelo status fechado nesse caminho. Por inspeção,
alterar uma configuração pode modificar planilhas históricas/fechadas quando
sincronizadas. É preciso definir a regra de snapshot histórico e fechamento.

### FIN-09 — P2: indicador de salvo não comprova persistência

`pages/finance.vue:498` mostra "Salvo automaticamente" sempre que não há indicador
de requisição em andamento, inclusive durante o debounce. O painel de configuração
usa a mesma simplificação. O estado precisa distinguir alteração pendente,
salvando, salvo e erro; hoje o texto pode transmitir uma garantia falsa.

## Testes executados

| Cenário | Resultado |
| --- | --- |
| CRUD básico sem datas, saldo com ajuste, atualização e exclusão | PASSOU |
| Repositório retorna 404 para consulta com outra conta sintética | PASSOU dentro do cenário de CRUD; não valida o middleware |
| Persistir e reler linha com data de efetivação | FALHOU: conversão de date para **string |
| Persistir duas contas fixas com composição | FALHOU: membros da primeira não retornam |
| Mesma recorrência em duas planilhas mensais | FALHOU: lines_pkey |

O primeiro cenário usou entrada 100 + ajuste 10, saída 30 e confirmou saldo
esperado 80. Não há testes próprios do Finance no repositório atual.
O build global bem-sucedido da etapa anterior não detecta essas falhas funcionais.

## Ordem proposta para correção

1. Validar membership e permissões por operação, com regressão entre contas e perfis.
2. Corrigir resolução dos componentes e validar a interface no navegador.
3. Corrigir tipos de datas, composição de contas e identidade das recorrências.
4. Proteger autosave, contexto de conta/planilha e histórico/fechamento.
5. Acrescentar testes permanentes e smoke completo de configuração, entradas,
   saídas, ajustes, efetivação, recorrências e recarga da página.

Correções funcionais e novos commits aguardam o próximo direcionamento do usuário.

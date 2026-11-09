# Correções do Finance

Base: auditoria de 2026-11-01. Branch: `fix/finance`.
Skill aplicada: `principios-engenharia`. Sem commit/push/deploy nesta etapa.

## Checklist de execução

- [x] FIN-01: validar membership e usar somente conta autenticada no contexto.
- [x] FIN-02: aplicar permissões por operação; leitores não alteram dados.
- [x] FIN-03: resolver componentes e verificar campos/popovers no navegador.
- [x] FIN-04: corrigir leitura/gravação de datas e testar round-trip.
- [x] FIN-05: separar identidade da recorrência e da linha por planilha.
- [x] FIN-06: corrigir retorno da composição de múltiplas contas fixas.
- [x] FIN-07: proteger autosave ao trocar planilha/conta, navegar e sob latência.
- [x] FIN-08: preservar snapshots históricos e fechamento das planilhas.
- [x] FIN-09: distinguir pendente, salvando, salvo e erro na interface.
- [x] Validação final: regressão Go/SQL, lint, tipos/build e smoke no navegador.

## Regras de implementação

Corrigir e validar cada grupo antes de avançar. Nenhuma senha, sessão ou registro
real será usado como fixture. Testes de persistência usam banco descartável.
Preservar acesso administrativo autorizado e funcionalidades existentes.
Configurações definem valores iniciais de novas planilhas; mudanças futuras não
reescrevem automaticamente snapshots já persistidos. Fechamento impede edição
até reabertura explícita por usuário com permissão de gestão.

## Evidências de validação

- Regressão HTTP: autenticação, membership, leitor/gestor, proprietário e admin.
- PostgreSQL descartável: datas com ajustes, CRUD, isolamento, composição,
  bloqueio de planilha fechada, reabertura e rejeição de versão desatualizada.
- Frontend: fila sob latência/falha, coalescência por contexto, recorrências por
  planilha, compatibilidade de IDs legados e preservação do snapshot histórico.
- Smoke local em planilha temporária: entrada 100 + ajuste 10, saída 30;
  saldo esperado/efetivado 80, datas e valores conferidos diretamente no banco;
  troca de planilha com pendência e fechamento verificados no navegador.

O lint Go do módulo concluiu com `0 issues`. A API local foi reconstruída com
as correções finais. Nenhum commit, push ou deploy foi realizado.

- `go test ./...` passou no backend completo com o repositório montado (incluindo
  os contratos de workflow usados pelos testes de Calendar).
- Suite direcionada PostgreSQL passou após as correções finais: membership/RBAC,
  datas e ajustes, composição e fechamento/conflito de versão.
- 7 testes Vitest passaram no Finance; ESLint da layer sem erros ou avisos.
- Typecheck global continua com erros em outros módulos; nenhum diagnóstico
  pertence a `layers/finance`. Não houve alteração funcional fora do Finance.
- Planilha temporária removida; as 2 planilhas e 6 linhas originais foram conferidas.

- GolangCI-Lint: `0 issues` no módulo financeiro.
- Smoke final após reinício limpo: Finance abre com os controles reais e somente
  as duas planilhas originais. A API final está saudável. Nenhum dado financeiro
  original, credencial, sessão, seed ou configuração de produção foi alterado.
- Build de produção com `NODE_ENV=production` e heap Node de 8192 MB; o primeiro
  ensaio sem esse limite excedeu o heap padrão, conforme registrado nos logs locais.
- Não foram executados commit, push ou deploy. Mudanças permanecem em `fix/finance`.

- Build final de produção concluído. Foi também adicionada regressão para a
  hidratação de uma nova planilha não remover contas fixas recém-inicializadas.
- O resultado não equivale a cobertura exaustiva de todos os cenários do produto;
  valida os nove achados desta auditoria e os fluxos locais descritos acima.

## Continuação do QA — 2026-11-01

- [x] Salvar funcionalidades e passos reproduzíveis em `ROTEIRO_TESTES.md`.
- [x] Corrigir exclusão do último ajuste sem recriar legado; 4 regressões permanentes, teste UI e conferência PostgreSQL (base100/ajuste0).
- [x] Expor ação explícita Limpar categoria; teste UI e banco confirmam categoria vazia.
- [x] Concluir viewport estreito (detectado cabeçalho cortado; correção validada em393×852 CSS).
- [x] Completar edição do tipo de conta fixa na UI.
- [x] Validar mensalidades e operações por loja com fixtures persistidas no ambiente local.
- [x] Validar criação do mês seguinte com configuração persistida.
- [x] Validar navegação e falha de rede com autosave na UI.
- [x] Validar perfis reais QA, preservando usuários existentes.
- [x] Limpar fixtures e consolidar resultados finais.

Esta lista acompanha a nova execução; não altera retroativamente o relatório histórico do QA.

Evidências e alcance dos checks desta continuação: [VALIDACAO_COMPLEMENTAR.md](VALIDACAO_COMPLEMENTAR.md). Perfis QA reais passaram via HTTP; UI não foi ensaiada com esses logins.

---
name: principios-engenharia
description: Princípios de engenharia e regras canônicas que o agente deve ler ANTES de escrever ou alterar qualquer código. Use sempre que for codar, refatorar, corrigir bug, criar página/componente/módulo/migration, fazer deploy, ou revisar código — mesmo que o usuário não peça explicitamente "siga as regras". Cobre fonte única de verdade no banco, isolamento multi-tenant, coexistência de features, design system, padrões de lint Go/SQL/TypeScript, e um registro de falhas reais para não repetir erros. Consulte este skill no INÍCIO de qualquer tarefa de implementação, não no fim.
---

# Princípios de Engenharia

Regras canônicas para escrever e alterar código com qualidade, segurança e consistência. Leia este arquivo no **começo** de qualquer tarefa de implementação. Os princípios abaixo são inegociáveis e valem para qualquer arquivo novo ou editado, em qualquer branch.

Quando a tarefa tocar uma área específica, **leia também o arquivo de referência correspondente** (índice no fim). Não carregue todas as referências de uma vez — só a(s) relevante(s) à tarefa.

---

## Princípios inegociáveis (valem sempre)

### 1. Fonte única de verdade no banco — nada hardcoded
**Toda** informação exibida vem do banco, através do back; **toda** alteração vai ao back e é gravada no banco. O fluxo é sempre: leitura `banco → back → front`; escrita `front → back → banco → front re-lê do banco` (ou usa o objeto que a API retornou). Nunca confiar só no estado local "achando que salvou".

- Sem valor cravado no código nem default que minta. Um default é só o estado inicial *antes* do dado real chegar; assim que a fonte autoritativa responde, o valor do banco vence.
- O front nunca renderiza um campo real a partir de fonte parcial/secundária (contexto, cache, fallback, storage local) que não tem aquele campo. Se a fonte autoritativa ainda não chegou, mostrar loading/vazio — não um default que mascare o dado.
- Rascunho de edição re-hidrata da resposta do back assim que ela chega; só se preserva enquanto há edição pendente do usuário (flag `touched`/dirty).
- **Por quê:** já se perdeu tempo depurando "não salvou / reverteu" quando o dado já estava certo no banco — o bug era o front exibir de uma fonte que não era o banco. Ao depurar isso, checar PRIMEIRO se o dado está no banco e se a API autoritativa o devolve, antes de mexer no back.

### 2. Isolamento multi-tenant — um usuário NUNCA vê dado de outro tenant
Nenhum payload/parâmetro forjado pode contornar o isolamento.

- Todo escopo (`tenantId`/`storeId`/`accountId`) vindo de query/body é validado contra o Principal/sessão e usado como **filtro dentro do permitido**, nunca confiado do client. Fora do escopo → erro.
- Defesa em profundidade: a query do repositório TAMBÉM filtra por tenant, mesmo que o service já tenha validado.
- Recurso fora do escopo retorna **404, não 403** (403 vs 404 vaza que o recurso existe — enumeration).
- `account_id`/tenant nunca vêm do body; sempre do Principal resolvido pelo middleware.
- **Por quê:** sem validação no service, o client passa o id de outro tenant e vaza dado. Handler novo que aceita um id de escopo sem validar é bug de segurança. Detalhe em `references/seguranca-multitenant.md`.

### 3. Nunca remover funcionalidade existente para resolver um problema
Ao corrigir bug ou adicionar comportamento, **não desligar/remover** uma funcionalidade já construída como atalho. Duas funcionalidades diferentes (ex.: abrir por hover E fechar no clique-fora) **coexistem** — somam, não se substituem. Só pode remover/trocar quando: (a) uma não pode existir sem quebrar a outra, ou (b) é mudança deliberada de regra de negócio. Nesses casos: **avisar e perguntar antes**, nunca remover por conta própria.

### 4. Avisar, documentar e marcar legado/mock — nunca esconder como "pronto"
Objetivo final é uma única fonte de verdade no banco real. Se uma feature só fica "pronta" removendo o legado, ela NÃO está pronta enquanto o legado existir. Ao encontrar ou criar legado/mock/dado-só-local/qualquer-coisa-não-persistida:
1. **Avisar o usuário na hora**: "isso ainda é legado/mock, não está no banco real, não está pronto".
2. **Documentar** num registro central de legado: o que é, por que é legado, qual o alvo, status de remoção.
3. **Marcar visível no front (só para admin)**: badge/aviso ("MOCK", "LEGADO", "não persiste") na própria tela, para ninguém desenvolver achando que está pronto.

Manter dois sistemas em sync é band-aid temporário e precisa estar marcado para remoção, nunca tratado como solução final.

### 5. Config/dado faltante = aviso acionável inline, nunca default silencioso
Em produção, dados que o cálculo usa vão faltar. A tela nunca pode mascarar isso nem obrigar o usuário a caçar a página de config. Onde a ausência muda o resultado: mostrar aviso honesto ali mesmo ("Sem meta cadastrada — penalidade desligada"). Se o usuário pode editar aquilo, o aviso abre um editor inline na própria tela, gravando pela MESMA API canônica do recurso. A fonte continua única; muda só o ponto de entrada da edição.

### 6. Validar local antes de propor commit ou deploy
Sempre rodar e testar local antes. Mudança de UI precisa de teste no browser, não só type-check — type-check e testes validam corretude de código, não de feature.

### 7. Higiene de processo
- Documentar antes de implementar: para tarefa não-trivial (3+ passos ou múltiplas camadas), apresentar o plano ao usuário antes de codar.
- Atualizar a documentação do módulo (AGENT.md ou equivalente) ao alterar o módulo: novos endpoints, tabelas, contratos.
- Nunca commitar/push/deploy por conta própria — devolver os comandos para o usuário rodar.
- Sem `Co-Authored-By` do agente em commits. Atribuição fica com o desenvolvedor humano.
- Sem emojis em UI, código, comentários ou commits, salvo pedido explícito.
- Ao planejar trabalho com 2+ partes independentes, oferecer a opção de rodar em paralelo com múltiplos agentes (não assumir sequencial por padrão). Não compensa para keystones de design onde o prompt custa quase tanto quanto fazer, nem quando uma parte depende do resultado da outra.

---

## Código limpo (base para tudo)

- **Limite de linhas por arquivo** (~450–500). Ao ultrapassar, dividir por responsabilidade. Preferir menos arquivos coesos a muitos arquivos rasos — desde que cada um tenha responsabilidade clara.
- Sem repetição: o que aparece 2× vira função/componente/helper compartilhado.
- Sem código morto: remover o que não se usa; não comentar "pra depois".
- Nomes descritivos; código que se explica sem comentário.
- Separação de camadas estrita: `handler → service → repository` no back, `page → composable → store/fetch` no front. Sem pular camada. Handler não acessa banco; repository não conhece HTTP; page orquestra, não tem lógica.
- Componentização: UI que aparece em 2+ lugares vira componente; query/lógica usada em 2+ handlers vira método no repository/service.

---

## Quando ler cada referência

Leia o arquivo de referência **antes** de mexer na área correspondente:

- **`references/frontend.md`** — criar/alterar componente, página, workspace, estilo. Design system e tokens de cor, classes semânticas BEM, layout compacto, blocos colapsáveis, overflow de página, dropdown/popover (fechar no clique-fora/Esc), cabeçalho de página admin, esconder página não-pronta, checklist de criar página nova (falhas silenciosas de rota/gating/workspace).
- **`references/backend.md`** — criar/alterar módulo, handler, service, repository. Padrão de módulo, IDs como string, scan de NULL, RBAC dinâmico (permissões no banco), injeção de dependência, tratamento de erro.
- **`references/database.md`** — migration, schema, tabela, view, mexer em dados de usuário/senha. Migration idempotente, schema-per-módulo, `account_id` em tabela tenant-scoped, view de compat ao mover tabela, e a regra crítica de nunca sobrescrever senha/seed sem permissão explícita.
- **`references/seguranca-multitenant.md`** — qualquer endpoint, fetch, ou checagem de permissão. Inventário de camadas de defesa, gaps conhecidos (RLS, security headers), e "pedir só o necessário" (sem N+1, paginação, projeção lean, gatear fetch por permissão).
- **`references/lint.md`** — escrever código Go, SQL ou TypeScript/Vue. Padrões obrigatórios que o linter cobra (perms de arquivo, switch vs if-else, schema qualificado em SQL, sem `any`, sem `console.log`, limite de linhas).
- **`references/deploy.md`** — qualquer deploy ou mudança em script/workflow de deploy, ou adicionar variável de ambiente.
- **`references/registro-de-falhas.md`** — **leia ao depurar um bug que parece familiar.** Casos reais já resolvidos (componente que some, draft que não re-hidrata, rota que conflita no boot, workspace que não aparece no menu, select que explode, sync externo bloqueado por WAF). Cada um vira uma regra. Consultar antes de gastar tempo re-descobrindo uma causa-raiz já conhecida.

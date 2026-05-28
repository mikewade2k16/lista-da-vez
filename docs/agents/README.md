# Briefings por Agente

Esta pasta tem 1 briefing por agente envolvido no ciclo de Levar Omni para Produção.

**Você é um dos 3 agentes**:

| Agente | Briefing | Zona |
|---|---|---|
| Claude Code | [CLAUDE.md](CLAUDE.md) | `back/`, infra, docs, scripts |
| Codex CLI | [CODEX.md](CODEX.md) | `web/app/components/{users,feedback,settings,erp}/` + `theme/` |
| GitHub Copilot (Codex) | [COPILOT.md](COPILOT.md) | `web/app/components/operation/` + `docs/adr/` |

**Para o usuário Mike**: os prompts de kick-off (que você cola na 1ª mensagem de cada agente) estão em [PROMPTS.md](PROMPTS.md).

Antes de começar, leia **na ordem**:

1. [../PARALELIZACAO.md](../PARALELIZACAO.md) — visão mestre, ondas, regras comuns
2. Seu próprio briefing (link acima) — tarefas concretas com arquivos e critérios de aceitação
3. [../PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md) — contexto das fases (apenas se precisar de detalhes que não estão no briefing)

---

## Regras inegociáveis (resumo)

1. **Branch única**: `refactor/multi-tenant-core`. Você só edita arquivos, **nunca roda comandos git** — o usuário centraliza commit/pull/push/rebase.
2. **Zona disjunta**: nunca edite arquivo fora da sua zona. Tocou fora = pare e avise.
3. **Nenhum comando git** (`git status` é OK para diagnóstico se necessário, mas nada que escreva no repo: sem `git add`, `git commit`, `git pull`, `git rebase`, `git push`, `git stash`, `git checkout` em arquivo).
4. **Pre-commit hook é amigo**: se um lint falhar enquanto você está iterando, leia o erro e corrija — não tente `--no-verify` (você nem deve estar commitando).
5. **Conflito inesperado** (arquivo já modificado por outro agente): pare, avise no chat com o usuário, não resolva sozinho.
6. **Atualizar status** em [../PARALELIZACAO.md](../PARALELIZACAO.md) (só a sua linha na tabela de Status) ao completar uma tarefa. O usuário commita quando achar pertinente.
7. **AGENT.md do módulo tocado** — sempre atualizar quando mexer no comportamento. (Sem commit.)
8. **Validações continuam**: `npm --prefix web run build`, `npm --prefix web run lint`, `go test ./...`, smoke manual — tudo como antes. Só o git que sai da sua mão.

## Como reportar status

Quando concluir uma tarefa do briefing:

1. Marca a tarefa como `[x]` no SEU briefing.
2. Edita APENAS sua linha em [../PARALELIZACAO.md](../PARALELIZACAO.md) na tabela "Status" da Onda correspondente:
   - troca `⚪ pending` por `🟢 done`
   - troca `— ` por a data ISO (`2026-05-21`)
3. Avisa no chat o que concluiu e sugere uma mensagem curta de commit (Conventional Commits) — o **usuário** decide quando juntar tarefas em um commit.

Se estiver no meio de uma tarefa, marca `🟡 in progress`. Se travou, marca `🔴 blocked` e abre conversa no chat com o usuário descrevendo o bloqueio.

## Convenções de mensagem de commit (apenas para sugerir ao usuário)

```
<tipo>(<escopo opcional>): <descrição imperativa curta>
```

Tipos permitidos: `feat`, `fix`, `refactor`, `test`, `chore`, `docs`, `style`, `perf`, `ci`.

Exemplos de sugestão:

```
refactor(users): extrair drafts de UsersAccessManager para useUserAccessDrafts
refactor(erp): separar helpers CRM de repository_postgres em repository_crm
docs(adr): adicionar ADR-0001 sobre rename para Omni
```

## Se você precisa tocar fora da sua zona

NÃO toque direto. Em vez disso:

1. Pare a tarefa atual.
2. Documente no chat com o usuário:
   - Qual arquivo fora da zona você precisa tocar
   - Por quê (a tarefa do briefing não dá pra fazer sem esse arquivo)
   - O que sugere
3. Aguarde o usuário decidir: ou ele faz, ou redireciona pra outro agente, ou aprova você fazer com supervisão.

## Quem faz git

Só o usuário (Mike). Em uma sessão multi-agente, qualquer comando git rodado por um agente vira corrida com os outros 2, gera rebases em rajada e risco de quem chegar por último sobrescrever quem chegou primeiro. Centralizar git no humano elimina isso. Repete: você edita arquivos no working tree, valida com build/lint/test/smoke, atualiza status no `PARALELIZACAO.md`, e avisa no chat. Mike commita.

## Estado das fases já concluídas (não re-fazer)

Não toque nestes itens, já estão prontos:

- **Fase 0** Decisões fechadas (nome = Omni, DB rename = sim, layer queue = placeholder)
- **Fase 1** Limpeza da raiz (35 → 10 arquivos)
- **Fase 2** Reorganização de docs (`docs_depoy/` consolidado, `operations.md` movido)
- **Fase 6** Qualidade & padronização (ESLint+Prettier, golangci-lint, Husky, tsconfig, AGENT.md padronizado, migration numbering, domain-first)

Detalhes em [../PLANO_REFATORACAO.md](../PLANO_REFATORACAO.md).

# AC-03 — tmp/ com dump real de banco (245MB) e tokens/secret em disco — RUNBOOK de limpeza

> **Natureza deste documento**: RUNBOOK MANUAL + 1 mudança de documentação.
> **REGRA DURA**: NENHUM comando das seções 3.1–3.3 é executado por agente/subagente.
> São dados reais (dump de banco de produção local pré-rename). Só o DONO do projeto
> (Mike) roda os comandos, manualmente, após as confirmações indicadas.
> A ÚNICA ação que o subagente implementador executa é a da seção 3.4 (nota de
> prevenção no `AGENT.md` raiz) — edição de markdown, sem tocar em `tmp/`.

---

## 1. Contexto

**Achado canônico AC-03** (P0 advisory, esforço S, impacto médio — `fatos.json > achados_canonicos.AC-03`):

`tmp/` na raiz do repo soma **254,18MB** em disco de dev. Está gitignored
(`.gitignore:42` → linha `tmp/`, confirmado com `git check-ignore -v`), ou seja,
**nada disso vai para o git** — o risco é exclusivamente de higiene de disco local
e de segredo/dado real esquecido em texto plano.

Inventário real medido em 2026-07-02 (`ls -laR tmp/`):

### 1.1 Dump real de banco pré-rename (o item de 245MB)

| Arquivo | Bytes | Data | O que é |
|---|---|---|---|
| `tmp/omni-rename-20260521_200120/lista-da-vez-before-omni.dump` | 257.265.404 (245,35MB) | 2026-05-21 20:02 | **pg_dump REAL** do banco local imediatamente ANTES do restore do rename lista-da-vez → omni |
| `tmp/omni-rename-20260521_200120/omni-before-restore.dump` | 1.130.264 (1,08MB) | 2026-05-21 20:02 | dump do banco `omni` (então quase vazio) antes do mesmo restore |
| `tmp/omni-rename-20260521_200120/README.txt` | 79 | 2026-05-21 20:01 | texto: "Backups before local lista-da-vez -> omni DB restore, created 20260521_200120" |

O rename lista-da-vez → omni foi **concluído em 2026-05** (ADR 0001) e o banco atual
funciona há mais de um mês com dezenas de migrations posteriores (última: 0186).
O dump é um seguro contra um restore que deu certo há 6 semanas.

### 1.2 Segredos/tokens de teste em texto plano

Todos verificados nesta sessão (tipo e tamanho; conteúdo NÃO deve ser copiado
para nenhum doc):

| Arquivo | Bytes | Data | O que era |
|---|---|---|---|
| `tmp/secret.key` | 20 | 2026-04-28 | **confirmado igual ao dev secret default público** (`dev-secret-change-me`, `back/internal/config/config.go:12`) — nenhum segredo novo, mas não deve existir como arquivo |
| `tmp/token.txt` | 233 | 2026-04-28 | JWT de teste (TTL 12h — expirado há meses) |
| `tmp/full_token.txt` | 0 | 2026-04-28 | vazio (sobra de redirect de script) |
| `tmp/runtime_token.txt` | 232 | 2026-05-20 | JWT de teste de sessão runtime |
| `tmp/runtime_token_0184.txt` | 313 | 2026-05-20 | JWT de teste (validação da migration 0184) |
| `tmp/runtime_token_mike.txt` | 325 | 2026-05-20 | JWT de teste do usuário mike |
| `tmp/runtime_token_mike_url.txt` | 257 | 2026-05-20 | mesmo token em formato URL |
| `tmp/payload.b64` | 183 | 2026-04-28 | payload de JWT em base64 — claims reais (`sub` do platform_admin, e-mail `mikewade2k16@gmail.com`), `exp` já vencido |
| `tmp/me_context.json` | 2.696 | 2026-05-20 | resposta de `/v1/me/context` (dados reais de contexto do admin) |
| `tmp/latest_cross_user_comment.txt` | 29 | 2026-05-20 | fragmento de teste de comentário cross-user |

Risco efetivo **baixo** (tokens expirados; secret é o default já público no repo),
mas o padrão "colar token em `tmp/*.txt`" é o que um dia vaza um token válido.

### 1.3 Artefatos de trabalho antigos (restante dos ~7MB)

| Arquivo | Bytes | Data | O que é |
|---|---|---|---|
| `tmp/task-video-smoke.mp4` | 5.752.049 (5,49MB) | 2026-05-20 | vídeo de smoke test de upload de tasks |
| `tmp/tasks_dump.sql` | 1.039.472 (0,99MB) | 2026-05-23 | dump da saga de import de tasks |
| `tmp/tasks_dump_filtered.sql` | 460.311 (0,44MB) | 2026-05-23 | idem, filtrado |
| `tmp/tasks-import/import-tasks-oficial.sql` | 247.457 | 2026-05-22 | SQL de import oficial de tasks |
| `tmp/tasks-import/pre-import-tasks-backup.sql` | 32.482 | 2026-05-22 | backup pré-import |
| `tmp/tasks-import/tasks-extraidas.txt` | 90.682 | 2026-05-22 | extração intermediária |
| `tmp/tasks-import/tasks-normalizadas.json` | 454.095 | 2026-05-22 | normalização intermediária |
| `tmp/pre_restore_empty_tasks_20260525_001330.sql` | 4.299 | 2026-05-25 | backup pré-restore de tasks vazias |
| `tmp/tasks_crow_board.json` | 3.265 | 2026-05-20 | snapshot de board de tasks |
| `tmp/tasks_crow_snapshot.json` | 1.125 | 2026-05-20 | idem |
| `tmp/tasks_snapshot.json` | 979 | 2026-05-20 | idem |
| `tmp/nuxt-crm-3001.log` | 46.016 | 2026-06-08 | log de dev server Nuxt |

Os arquivos `tasks*` são backups da saga de import/restore de tasks (concluída em
2026-05; os dados de tasks estão no banco atual e funcionando). Pela regra do
projeto de **nunca sobrescrever/perder dado de usuário sem confirmação**, eles só
saem depois da confirmação do passo 3.3.

---

## 2. Objetivo e não-objetivos

**Objetivo**
1. Entregar um runbook exato, arquivo por arquivo, com comandos prontos para o
   DONO decidir e executar a limpeza de `tmp/` (recupera ~254MB de disco e elimina
   segredos de teste em texto plano).
2. Prevenir recorrência: regra escrita no `AGENT.md` raiz proibindo segredo/dump
   real em `tmp/` (única mudança que o implementador executa).

**Não-objetivos (explicitamente FORA de escopo)**
- NÃO executar nenhuma exclusão/movimentação em `tmp/` por agente (nem no
  implementador, nem em validação). Dados reais = só o dono roda.
- NÃO alterar `.gitignore` (a linha 42 `tmp/` já cobre; nada está tracked).
- NÃO tocar em `web-reference/`, `back;C`, `erp-source-local/`, PNGs/pix da raiz —
  são outros achados (AC-17, AC-18, AC-20), com specs próprias.
- NÃO mexer em backup de produção/VPS (isso é AC-05).
- NÃO editar `docs/LEGADO.md`: o registro ali é de legado/mock **em código de
  produto**; `tmp/` é higiene de disco local, não entra. (Decisão tomada — não
  reabrir.)
- NÃO criar script automatizado de limpeza de `tmp/` (contraria a regra dura).

---

## 3. Mudanças

### 3.1 [DONO — manual] Decisão sobre o dump pré-rename (245MB)

**Pré-verificação (read-only, o dono roda se quiser dupla checagem):**

```powershell
# banco atual responde e tem as migrations aplicadas até 0186?
docker compose exec postgres psql -U omni -d omni -c "SELECT max(version) FROM schema_migrations;"
# painel funciona? (login em http://localhost:3003) — se sim, o dump pré-rename não protege mais nada
```

**Opção A — armazenamento frio fora do repo (conservadora, recomendada se houver
qualquer dúvida).** Move a pasta inteira para fora do working tree; o repo
enxuga 246,43MB e o dump continua recuperável:

```powershell
New-Item -ItemType Directory -Force "C:\Users\Mike\Backups\omni-cold" | Out-Null
Move-Item "C:\Users\Mike\Documents\Projects\fila-atendimento\tmp\omni-rename-20260521_200120" `
          "C:\Users\Mike\Backups\omni-cold\omni-rename-20260521_200120"
```

**Opção B — exclusão definitiva (só após confirmar a pré-verificação acima):**

```powershell
Remove-Item -Recurse -Force -Confirm:$false `
  "C:\Users\Mike\Documents\Projects\fila-atendimento\tmp\omni-rename-20260521_200120"
```

Decisão já tomada para o runbook: **A ou B é escolha exclusiva do dono**; o
implementador não escolhe nem executa. Se em 30 dias o armazenamento frio não for
consultado, o dono pode apagar a cópia fria.

### 3.2 [DONO — manual] Excluir os arquivos de token/secret (baixo risco)

Todos os 10 arquivos da seção 1.2 são de teste, expirados ou redundantes
(o `secret.key` é literalmente o default público de `config.go:12`). Comando único:

```powershell
Set-Location "C:\Users\Mike\Documents\Projects\fila-atendimento"
Remove-Item -Force -Confirm:$false `
  tmp\secret.key, tmp\token.txt, tmp\full_token.txt, `
  tmp\runtime_token.txt, tmp\runtime_token_0184.txt, `
  tmp\runtime_token_mike.txt, tmp\runtime_token_mike_url.txt, `
  tmp\payload.b64, tmp\me_context.json, tmp\latest_cross_user_comment.txt
```

Nenhuma pré-condição além de estar na raiz do repo — nada no código referencia
esses arquivos (eram entrada/saída de curls manuais de teste).

### 3.3 [DONO — manual] Limpar artefatos antigos (tasks, vídeo, logs)

**Pré-condição**: confirmar que a página de Tasks do painel está íntegra com os
dados atuais (os `tasks*` de 1.3 são backups da saga de import de 2026-05; se as
tasks no banco estão corretas hoje, os backups intermediários não protegem nada).

```powershell
Set-Location "C:\Users\Mike\Documents\Projects\fila-atendimento"
Remove-Item -Force -Confirm:$false `
  tmp\task-video-smoke.mp4, tmp\tasks_dump.sql, tmp\tasks_dump_filtered.sql, `
  tmp\pre_restore_empty_tasks_20260525_001330.sql, `
  tmp\tasks_crow_board.json, tmp\tasks_crow_snapshot.json, tmp\tasks_snapshot.json, `
  tmp\nuxt-crm-3001.log
Remove-Item -Recurse -Force -Confirm:$false tmp\tasks-import
```

Se preferir o caminho conservador, mover os `tasks*` junto para
`C:\Users\Mike\Backups\omni-cold\` (mesmo padrão da Opção A) em vez de excluir.

Ao final de 3.1+3.2+3.3, `tmp/` fica vazio (a pasta em si pode ficar — está
gitignored e continua útil como área de rascunho).

### 3.4 [IMPLEMENTADOR — única ação executável] Nota de prevenção no AGENT.md raiz

Editar `c:\Users\Mike\Documents\Projects\fila-atendimento\AGENT.md`, seção
`## Regras gerais` (linhas 73–88). Acrescentar ao FINAL da lista de bullets da
seção (após a linha 88, `  - seguranca por loja/dispositivo entra como proxima camada de hardening`,
e antes de `## Documentos principais`) exatamente este bloco:

```markdown
- Higiene de `tmp/` (raiz): e area de rascunho gitignored, NAO e lugar de dado real.
  - Proibido deixar token/JWT, secret/chave ou dump de banco em `tmp/` apos o uso — apagar no fim da tarefa.
  - Dump/backup real vai para fora do repo (ex.: `C:\Users\Mike\Backups\omni-cold\`), nunca fica em `tmp/`.
  - Agentes nao excluem conteudo de `tmp/` por conta propria: dado real exige confirmacao do dono (runbook em `docs/specs/ac-fixes-2026-07/AC-03-tmp-dump-segredos.md`).
```

Observações de execução:
- Manter o estilo sem acento do arquivo (o `AGENT.md` raiz é escrito sem
  acentuação — o bloco acima já segue isso).
- Nenhuma outra linha do `AGENT.md` muda.
- O implementador NÃO roda nenhum comando das seções 3.1–3.3, nem "para testar".

---

## Regras de execução (obrigatórias para o implementador)

- NENHUM comando git (sessão multi-agente — só o usuário roda git).
- NENHUM comando das seções 3.1–3.3 é executado por agente. São do DONO, manual,
  após confirmação. Isso inclui `Remove-Item`, `Move-Item` e qualquer variação.
- NÃO rodar npm/build/generate. Esta spec não toca `back/` (não há
  `docker compose up -d --build api`) nem `web/` — é só markdown.
- Máx 450 linhas por arquivo (a edição do `AGENT.md` o mantém em ~132 linhas).
- Não remover funcionalidade existente; zero mock/legado novo.
- Não editar `docs/LEGADO.md` (decisão da seção 2).
- Portas fixas intocadas (api 9091, web 3003, postgres 5432) — não se aplica aqui,
  mas nenhuma configuração de porta pode ser alterada.
- NUNCA tocar em password_hash/dados de usuário — não se aplica (nenhum acesso a banco).
- Atualizar AGENT.md do módulo tocado: o "módulo" aqui É o `AGENT.md` raiz (3.4);
  nenhum módulo de `back/` ou `web/` é tocado, então nenhum outro AGENT.md muda.

---

## 4. Critérios de aceite

Da parte do implementador (verificável imediatamente):
1. `AGENT.md` raiz contém o bloco "Higiene de `tmp/`" com os 3 sub-bullets, dentro
   de `## Regras gerais`, antes de `## Documentos principais`.
2. Nenhum arquivo dentro de `tmp/` foi criado, alterado, movido ou excluído pelo
   agente (mtimes de `tmp/` idênticos aos da seção 1).
3. Nenhum outro arquivo do repo foi modificado além de `AGENT.md` (e a própria
   spec, já escrita).
4. Nenhum comando git foi executado.

Da parte do dono (quando decidir rodar o runbook — não bloqueia o implementador):
5. Passo 3.1 decidido (A ou B) e executado; `tmp/omni-rename-20260521_200120/`
   não existe mais no working tree.
6. Os 10 arquivos da seção 1.2 não existem mais.
7. Artefatos da seção 1.3 removidos (ou movidos para frio).
8. `tmp/` reduzido de 254,18MB para ~0.

---

## 5. Validação

Implementador (somente leitura):

```powershell
# 1) nota de prevencao presente no AGENT.md raiz
Select-String -Path "AGENT.md" -Pattern "Higiene de ``tmp/``" -SimpleMatch:$false

# 2) prova de que tmp/ nao foi tocado pelo agente (comparar com secao 1: nada novo, mesmas datas)
Get-ChildItem -Recurse tmp | Select-Object FullName, Length, LastWriteTime
```

Dono (após rodar o runbook):

```powershell
# tamanho restante de tmp/ (esperado ~0)
"{0:N2} MB" -f ((Get-ChildItem -Recurse -File tmp | Measure-Object -Sum Length).Sum / 1MB)

# nada de token/secret sobrou
Get-ChildItem tmp -Recurse -Include *token*, *.key, payload.b64

# sanidade do banco (o dump nao era mais necessario)
docker compose exec postgres psql -U omni -d omni -c "SELECT max(version) FROM schema_migrations;"
```

---

## 6. Notas de Deploy

**Nenhuma.** Sem migration, sem env var, sem rebuild de api/web, sem mudança de
imagem. Tudo é disco local de dev + 1 edição de markdown.

---

## 7. Arquivos tocados

Pelo implementador:
- `docs/specs/ac-fixes-2026-07/AC-03-tmp-dump-segredos.md` (esta spec — já criada)
- `AGENT.md` (raiz — bloco novo em `## Regras gerais`, seção 3.4)

Pelo dono (manual, fora do escopo do implementador; todos gitignored):
- `tmp/omni-rename-20260521_200120/` (mover ou excluir — 246,43MB)
- `tmp/secret.key`, `tmp/token.txt`, `tmp/full_token.txt`, `tmp/runtime_token.txt`,
  `tmp/runtime_token_0184.txt`, `tmp/runtime_token_mike.txt`,
  `tmp/runtime_token_mike_url.txt`, `tmp/payload.b64`, `tmp/me_context.json`,
  `tmp/latest_cross_user_comment.txt` (excluir)
- `tmp/task-video-smoke.mp4`, `tmp/tasks_dump.sql`, `tmp/tasks_dump_filtered.sql`,
  `tmp/pre_restore_empty_tasks_20260525_001330.sql`, `tmp/tasks_crow_board.json`,
  `tmp/tasks_crow_snapshot.json`, `tmp/tasks_snapshot.json`,
  `tmp/nuxt-crm-3001.log`, `tmp/tasks-import/` (excluir ou mover para frio)

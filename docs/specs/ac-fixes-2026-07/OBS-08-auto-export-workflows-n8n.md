# OBS-08 — Auto-export dos workflows n8n ao mudar (repo sempre pronto pro deploy)

> Spec de implementação · Prioridade **P1** · Esforço **M** · Impacto **médio-alto**
> Origem: ideia do dono 2026-07-13 (grupo observabilidade-n8n) · roadmap `observabilidade-n8n` → task `obs-08-auto-export-workflows-n8n`
> **É o par de `npm run n8n:import`** (`scripts/dev/n8n-import.ps1`): hoje só temos import-on-change; esta spec cria export-on-change. Ver [[feedback_n8n_import_on_change]].

## 1. Contexto

**O achado (dor real, 2026-07-13):** ao ir fazer deploy, os `automation/export/workflow-*.json` estavam **desatualizados** em relação ao que rodava no n8n — foi preciso exportar na mão os 5 workflows antes de subir (diff de +587/−226 linhas). O n8n guarda os workflows no **próprio banco** (SQLite, não lê do arquivo), então o repositório só reflete o n8n se alguém **lembrar de exportar**. Ninguém lembra. Resultado: o deploy leva uma versão velha, ou trava até exportar manualmente.

O import já é resolvido (`scripts/dev/n8n-import.ps1` faz o ciclo copia→import→reativa→restart num comando). **Falta o inverso**: quando o workflow muda no n8n, materializar o `.json` versionado automaticamente, para o `deploy:fast:prod` sempre levar o n8n idêntico ao que está rodando, sem passo manual.

**Objetivo maior:** junto com AC-04b (deploy self-healing de infra) e OBS-07 (sonda do n8n), fecha a visão do dono: **infra self-healing + n8n sempre versionado = deploy sempre pronto**.

**Evidências / o que já sabemos (levantado em 2026-07-13):**
- **Método de export canônico** (o mesmo que o `deploy-pull.ps1:229` usa para ler estado): `n8n export:workflow --id=<id> --output=/tmp/x.json` dentro do container, depois `docker cp`/`cat` para fora. Comprovado funcionando ao exportar os 5.
- **Container:** dev = `omni-n8n-1` (`scripts/dev/n8n-import.ps1:29`); imagem `n8nio/n8n:2.23.2`; profile `automation`. Prod = serviço `n8n` (loopback `127.0.0.1:${AUTOMATION_N8N_PORT}`).
- **Mapa id → arquivo** (naming estável, NÃO derivar do nome do workflow — derivar polui o git diff se o nome mudar):
  | arquivo | id | nome |
  |---|---|---|
  | `workflow-calendar-chat.json` | `calendarchat0001` | Calendar Chat |
  | `workflow-calendar-omni.json` | `calendaromni0001` | Calendar Omni |
  | `workflow-calendar-transcribe.json` | `calendartrans001` | Calendar Transcribe |
  | `workflow-omni-chat.json` | `omnichatmvp00001` | Omni Chat |
  | `workflow-whatsapp.json` | `lzhb5JjN5kdcVuRR` | Whatsapp |
- **Shape versionado exigido** (senão vira ruído de diff): **array com 1 objeto**, chaves `id,name,nodes,connections,active,settings,staticData,meta,pinData,versionId`, e **`active: false`** (o import desativa e o deploy reativa; manter `false` evita flip-flop no diff a cada export). O `n8n export:workflow --id` entrega objeto único → normalizar para `[obj]` + `active:false`.
- **Segurança comprovada:** `n8n export:workflow` **NÃO** exporta credenciais decriptadas — os blocos `credentials` dos nós trazem só `{id,name}` (ex.: `redis`→`Redis account`). Confirmado nos 5 workflows. A spec deve manter uma **verificação anti-vazamento** no pipeline (falhar se algum nó trouxer campo de credencial além de `id`/`name`).
- **Watch do dev:** o n8n dev NÃO tem bind mount do código (roda do volume); o dev do web usa compose watch. Não há hook nativo "on save" trivial no n8n OSS.

## 2. Objetivo e não-objetivos

**Objetivo (escopo fechado — MVP determinístico primeiro):**
1. Criar `scripts/dev/n8n-export.ps1` (espelho do `n8n-import.ps1`): exporta os workflows do container para `automation/export/workflow-*.json`, no shape versionado (array + `active:false`), pelo **mapa id→arquivo fixo** (nomes estáveis). Flags: `-Only <slug>`, `-Container`, e dois modos de gatilho:
   - `-Check` (não escreve; sai ≠0 se há divergência — para o **guard do git/pre-commit**: AVISA);
   - `-Sync` (verifica e, se houver divergência, **exporta sozinho** e segue com exit 0 — para o **gatilho do deploy**: AUTO-EXPORTA).
2. Verificação anti-vazamento embutida: aborta o arquivo se algum nó tiver credencial com campo além de `id`/`name`.
3. Registrar como o **par** do import: `npm run n8n:export` / `:chat` / `:check` / `:sync` no `package.json`.
4. **DOIS gatilhos automáticos** (o dono deploya às vezes ANTES de commitar — os dois pontos precisam do gatilho):
   - **git (pre-commit):** modo `-Check` — na hora de versionar, AVISA se o export está atrás do n8n (barato, sem daemon). §4.3-A.
   - **deploy (`deploy:fast:prod`):** modo `-Sync` — antes de empacotar/enviar a automação, **auto-exporta** se o n8n dev estiver à frente, para o deploy SEMPRE levar o n8n atual sem passo manual. §4.3-D. **Este é o gatilho que o dono mais quer** ("rodo o deploy, ele já verifica e exporta se precisar").
   As opções B/C (trigger dentro do n8n, watcher contínuo) ficam documentadas como evolução.

**Não-objetivos (FORA):**
- NÃO exportar credenciais (só estrutura; a verificação garante).
- NÃO derivar nomes de arquivo do nome do workflow (mapa fixo id→arquivo; nome muda sem renomear arquivo).
- NÃO importar/reativar nada (isso é o `n8n-import.ps1`; export só materializa arquivo).
- NÃO instalar daemon/watcher pesado no MVP (a opção "trigger dentro do n8n" e "watcher contínuo" ficam como evolução, §4.3-B/C).
- NÃO auto-commitar o export ([[feedback_local_only]] / [[feedback_no_git_in_multiagent]]): o script só escreve os arquivos; **o dono commita**.
- NÃO rodar contra a VPS por padrão (o export é do dev; sincronizar prod→repo é um follow-up com cuidado extra, §4.4).

## 3. Regras de execução (obrigatórias)

- **NENHUM comando git** no script (só escreve arquivos; o dono commita). [[feedback_local_only]]
- PowerShell 5.1-safe: `.ps1` **ASCII sem BOM** (o `n8n-import.ps1` documenta isso no header — acento/travessão quebram o parser). Sem `&&`/ternário/`??`.
- Pegadinha MSYS/Git Bash: passar `/tmp/x` direto vira caminho Windows no container → usar `docker exec <c> sh -lc "... /tmp/..."` (mesma nota do `n8n-import.ps1:16-18`).
- **Não usar `docker cp database.sqlite`** para ler estado (WAL não é levado) — sempre `n8n export:workflow` (nota `n8n-import.ps1:18-19`).
- Mapa id→arquivo em UM ponto do script, com comentário de que é o mesmo conjunto de `n8n-import.ps1` e `deploy-pull.ps1:246` (contrato).
- Atualizar `automation/AGENT.md` (registrar o par export/import) e a doc de fluxo n8n.

## 4. Mudanças (passo a passo)

### 4.1 CRIAR `scripts/dev/n8n-export.ps1`

Espelho do `n8n-import.ps1`. Núcleo (pseudo-real, o implementador escreve ASCII-safe):

```
param(
  [string]$Container = "omni-n8n-1",
  [string]$Only = "",
  [switch]$Check,     # nao escreve; sai !=0 se algum arquivo divergir (guard do pre-commit: AVISA)
  [switch]$Sync       # verifica e, se divergir, ESCREVE e segue exit 0 (gatilho do deploy: AUTO-EXPORTA)
)
# Sem -Check e sem -Sync: exporta sempre (uso manual `npm run n8n:export`).
# -Check e -Sync sao mutuamente exclusivos (validar no topo; erro se ambos).
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$dir  = Join-Path $root "automation/export"

# MAPA FIXO id -> arquivo (mesmo conjunto de n8n-import.ps1 e deploy-pull.ps1:246).
$map = @(
  @{ id="calendarchat0001"; file="workflow-calendar-chat.json" },
  @{ id="calendaromni0001"; file="workflow-calendar-omni.json" },
  @{ id="calendartrans001"; file="workflow-calendar-transcribe.json" },
  @{ id="omnichatmvp00001"; file="workflow-omni-chat.json" },
  @{ id="lzhb5JjN5kdcVuRR"; file="workflow-whatsapp.json" }
)
if ($Only) { $map = $map | Where-Object { $_.file -like "*$Only*" } }

$drift = $false
foreach ($m in $map) {
  # 1) exporta o workflow do container para /tmp e traz o conteudo (cat via sh -lc)
  docker exec $Container sh -lc "n8n export:workflow --id=$($m.id) --output=/tmp/exp_$($m.id).json >/dev/null 2>&1" | Out-Null
  $raw = docker exec $Container sh -lc "cat /tmp/exp_$($m.id).json"
  # 2) normaliza (array + active:false) e VERIFICA credenciais via node (fail se vazar)
  #    -> chama um node -e que: parseia, checa que cada node.credentials[*] so tem id/name,
  #       seta active=false, imprime JSON pretty (2 espacos) OU sai 3 se detectar vazamento.
  # 3) -Check: compara com o arquivo; se difere marca $drift e NAO escreve.
  #    -Sync : compara; se difere, ESCREVE (auto-export) e marca $exported.
  #    normal: escreve sempre com \n final.
}
if ($Check -and $drift) {
  Write-Host "n8n-export: os arquivos versionados estao ATRAS do n8n rodando. Rode: npm run n8n:export"
  exit 1   # guard do git: AVISA (o dono decide se o pre-commit bloqueia ou so alerta, ver 4.3-A)
}
if ($Sync -and $exported) {
  Write-Host "n8n-export: n8n estava a frente; workflows exportados automaticamente ($exported arquivo(s)). Deploy seguira com a versao atual."
  # exit 0 de proposito: no deploy queremos SEGUIR com o export fresco, nunca travar.
}
```

Detalhes que a implementação DEVE respeitar (todos comprovados ao exportar em 2026-07-13):
- Normalização e checagem num `node -e` (o container tem Node): abortar com exit≠0 se algum `node.credentials[k]` tiver chave fora de `["id","name"]`.
- Saída pretty com 2 espaços + `\n` final (bate com os arquivos atuais; minimiza diff).
- `active: false` sempre.
- Se um id do mapa **não existir** no n8n → avisar (`Write-Warning`) e seguir (não derrubar tudo).

### 4.2 EDITAR `package.json` — scripts (par do import)

```json
"n8n:export": "powershell.exe -ExecutionPolicy Bypass -File scripts\\dev\\n8n-export.ps1",
"n8n:export:chat": "powershell.exe -ExecutionPolicy Bypass -File scripts\\dev\\n8n-export.ps1 -Only calendar-chat",
"n8n:export:check": "powershell.exe -ExecutionPolicy Bypass -File scripts\\dev\\n8n-export.ps1 -Check",
"n8n:export:sync": "powershell.exe -ExecutionPolicy Bypass -File scripts\\dev\\n8n-export.ps1 -Sync"
```

### 4.3 OS GATILHOS "on change" — implementar A **e** D (os dois pontos onde o repo pode estar velho)

> O dono deploya às vezes **antes** de commitar. Logo, um gatilho só no git NÃO basta — o deploy é o outro (e principal) ponto de risco. Implementar **A (git)** e **D (deploy)**. B e C são evolução.

**A) [git] Guard no pre-commit (`-Check`) — AVISA** — barato, sem daemon, alinhado ao pre-commit atual (husky/lint-staged):
- Passo que, **se o container n8n estiver up**, roda `n8n-export.ps1 -Check`. Arquivos atrás do n8n → **avisa** ("rode npm run n8n:export"). **Decisão do dono:** bloquear o commit ou só alertar (recomendo **alertar**, não bloquear — commit que não toca n8n não pode travar por isso; e o deploy tem o gatilho D que garante o export de qualquer jeito).
- Container n8n down → no-op.

**D) [deploy — O PRINCIPAL] Auto-export dentro do `deploy:fast:prod` (`-Sync`) — EXPORTA SOZINHO:**
- **Onde:** no `scripts/deploy/deploy-fast.ps1`, **condicionado a `-DeployAutomation`** (o `deploy:fast:prod` já passa essa flag), rodando **ANTES** do build/envio da automação — ou seja, logo no início, antes de `Build-And-Push` e antes de delegar ao `deploy-pull.ps1` (que é quem faz o `scp` dos `workflow-*.json`, `deploy-pull.ps1:196-202`). Assim o que for enviado já está fresco.
- **Comportamento:** roda `n8n-export.ps1 -Sync`. Se o n8n dev estiver à frente, **exporta automaticamente**, imprime o aviso ("n8n estava à frente; exportei N arquivos") e **segue o deploy** (exit 0 — no deploy nunca travamos por isso; o objetivo é levar o atual).
- **Guarda-corpo (importante):** o `-Sync` só roda se o **container n8n dev estiver up** (é a fonte da verdade local). Se estiver down, **não** exporta e **avisa** que não conseguiu verificar (`AVISO: n8n dev fora; nao verifiquei se automation/export esta fresco — deploy seguira com os arquivos versionados atuais`). Nunca apagar/zerar arquivo por o container estar fora.
- **Efeito colateral esperado e desejado:** o `-Sync` pode deixar arquivos `automation/export/*.json` modificados na working tree (não commitados). O deploy os USA (envia via scp) mesmo sem commit — que é exatamente o pedido ("deployar antes de commitar"). O script **NÃO commita** ([[feedback_local_only]]); ao final o dono vê os arquivos alterados e commita quando quiser. Registrar isso claramente no output do deploy.
- **Flag de escape:** aceitar `-SkipWorkflowExport` no `deploy-fast.ps1` para pular o gatilho (casos raros: dono quer deployar uma versão versionada específica, não o dev atual).

**B) [Evolução] Trigger dentro do próprio n8n** — workflow n8n que escuta "workflow salvo" (ou timer curto) e roda `n8n export:workflow` num volume mapeado para `automation/export/`. Mais "tempo real", mas exige volume + permissão de arquivo no host + o n8n OSS não expõe hook de save trivial (provável polling). Documentar, não implementar no MVP.

**C) [Evolução] Watcher no host** — `npm run n8n:watch` que faz `-Check` em intervalo e exporta ao detectar diff. Mais um daemon para lembrar de rodar. Opcional.

A spec ENTREGA **A + D** + o script da 4.1. B e C ficam registradas como follow-up no AGENT.md.

### 4.4 Follow-up documentado (NÃO no MVP): export da VPS→repo

Exportar o que roda em **produção** de volta para o repo (não só o dev) tem valor mas exige cuidado: rodar via o túnel/ssh do deploy, e resolver a política de "qual é a fonte da verdade, dev ou prod?". Registrar como decisão pendente — hoje o fluxo é dev→repo→deploy→prod (uma direção só).

### 4.5 EDITAR docs

- `automation/AGENT.md`: registrar o par **import↔export** (n8n roda do banco; `n8n:import` leva arquivo→n8n, `n8n:export` traz n8n→arquivo), o mapa id↔arquivo como contrato, a garantia anti-credencial, e as opções B/C como evolução.
- `docs/DEPLOY_VPS.md` ou a doc de automação: mencionar `npm run n8n:export` no checklist pré-deploy ("garanta que os workflows estão exportados") até o gatilho A estar consolidado.

## 5. Critérios de aceite

1. `npm run n8n:export` com o dev up: reexporta os 5 arquivos no shape versionado (array + `active:false`, 2 espaços, `\n` final); rodar 2x seguidas **sem mudar nada no n8n** produz **zero diff** na 2ª (idempotente/estável — o teste-chave contra ruído de diff).
2. `npm run n8n:export:chat`: mexe só em `workflow-calendar-chat.json`.
3. Alterar um workflow no n8n (ex.: renomear um nó) → `npm run n8n:export` reflete a mudança no arquivo certo, e **só** naquele arquivo.
4. `npm run n8n:export:check` sai **0** quando repo == n8n e **1** quando o n8n está à frente (com mensagem "rode npm run n8n:export").
5. **Gatilho do deploy (D):** com o n8n dev à frente, rodar o `deploy-fast.ps1 -DeployAutomation` (dry-run/interrompendo antes do push, ou em staging) **exporta sozinho** os arquivos divergentes e SEGUE (não trava); o output mostra "n8n estava à frente; exportei N". Com o n8n dev **down**, o deploy imprime o AVISO e segue com os arquivos versionados atuais (não zera nada). Com `-SkipWorkflowExport`, o gatilho é pulado.
6. **Anti-vazamento:** injetar (em teste) um workflow cujo nó tenha `credentials.x.password` → o export ABORTA aquele arquivo com erro claro, não grava segredo.
7. Import de volta funciona: `npm run n8n:export` seguido de `npm run n8n:import` não quebra (ids preservados, sem duplicação).
8. `.ps1` é ASCII sem BOM (`file scripts/dev/n8n-export.ps1` / abrir e conferir); `powershell -File ...` parseia sem erro.

## 6. Validação

```bash
# dev up:
npm run n8n:export
git diff --stat automation/export/        # 1a vez: pode mudar; roda de novo:
npm run n8n:export
git diff --stat automation/export/        # 2a vez SEM mexer no n8n: deve ser VAZIO
npm run n8n:export:check                   # exit 0 quando alinhado
# conferir credenciais so id/name nos 5 (mesma checagem que a spec embute):
node -e 'for(const f of require("fs").readdirSync("automation/export").filter(x=>x.startsWith("workflow-"))){const w=JSON.parse(require("fs").readFileSync("automation/export/"+f))[0];for(const n of (w.nodes||[]))for(const k in (n.credentials||{})){const ok=Object.keys(n.credentials[k]).every(x=>["id","name"].includes(x));if(!ok)console.log("VAZOU",f,n.name,k)}}console.log("check credenciais ok")'
```

Gatilho A: fazer um commit de teste com o n8n à frente e confirmar o aviso (e que NÃO bloqueia, se essa for a decisão).

## 7. Notas de Deploy

- **Migrations:** nenhuma. **Rebuild:** nenhum (é tooling de dev). **Env vars:** nenhuma.
- Não muda o runtime de prod: só garante que os `automation/export/*.json` que o `deploy:fast:prod -DeployAutomation` já envia estejam sempre frescos.
- Se a opção A bloquear o commit (decisão do dono), documentar no AGENT.md que o guard pode ser pulado com `--no-verify` quando o n8n dev estiver intencionalmente à frente.

## 8. Arquivos tocados

| Arquivo | Ação |
|---|---|
| `scripts/dev/n8n-export.ps1` | criar |
| `package.json` | editar (scripts n8n:export / :chat / :check / :sync) |
| `scripts/deploy/deploy-fast.ps1` | **editar (gatilho D: `-Sync` sob `-DeployAutomation`, antes do build; flag `-SkipWorkflowExport`)** |
| `.husky/pre-commit` ou config lint-staged | editar (guard `-Check`, opção A) — confirmar o ponto exato antes |
| `automation/AGENT.md` | editar (par import↔export, contrato, follow-ups B/C/VPS) |
| `docs/DEPLOY_VPS.md` (ou doc de automação) | editar (checklist pré-deploy + os dois gatilhos) |

**Conflitos potenciais:** contrato de mapa id↔arquivo com `n8n-import.ps1` e `deploy-pull.ps1:246` (os três devem listar o mesmo conjunto). Nenhum conflito com AC-04b/OBS-07. **O gatilho D edita o `deploy-fast.ps1` — o mesmo arquivo NÃO é tocado por AC-04b nem OBS-07, então pode ser feito em paralelo sem colisão.** **Decisões pendentes do dono:** (a) o guard do **pre-commit** BLOQUEIA ou só AVISA (o gatilho D do deploy sempre auto-exporta, independente disso); (b) incluir Whatsapp no export sempre (hoje sim, está no mapa) ou tratá-lo à parte por não ser core de prod.

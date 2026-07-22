# E0 — ownership e fronteira dos workflows

**Status:** `DONE` em 2026-07-20

**Resultado:** qualquer operação n8n do omnichannel é limitada aos dois IDs próprios e falha
fechado se tentar tocar Automação, WAHA, Calendário ou Operação.

## 1. Decisões fechadas

- WAHA e `workflow-whatsapp.json` pertencem ao módulo Automação; não são legado omnichannel.
- O brain WhatsApp e o first-contact Instagram são workflows distintos porque têm entradas,
  políticas e ritmos diferentes.
- Inventário/check global pode ler somente metadados, IDs e hashes dos workflows não selecionados;
  não pode exportar, importar, sincronizar nem materializar o conteúdo deles. Sync/import/write
  exige `-Owner` e `-Only` explícitos.
- Drift de workflow de outro módulo é apenas reportado; nunca corrigido por esta fase.
- Nenhum workflow guarda credencial ou estado definitivo.

## 2. Inventário canônico

| Arquivo | ID | Owner | Omnichannel pode alterar? | Estado esperado nesta fase |
|---|---|---|---:|---|
| `workflow-omnichannel-brain.json` | `omnibrain0000001` | Omnichannel | sim | exportável; ativação não faz parte do código |
| `workflow-instagram-first-contact.json` | `instafirst000001` | Omnichannel | sim | exportável; pode permanecer inativo |
| `workflow-whatsapp.json` | `lzhb5JjN5kdcVuRR` | Automação | não | conteúdo e estado preservados byte a byte |
| `workflow-calendar-chat.json` | `calendarchat0001` | Calendário | não | preservado |
| `workflow-calendar-omni.json` | `calendaromni0001` | Calendário | não | preservado |
| `workflow-calendar-transcribe.json` | `calendartrans001` | Calendário | não | preservado |
| `workflow-omni-chat.json` | `omnichatmvp00001` | Operação | não | preservado |

## 3. Implementação dos guardas

O registro de ownership deve ser uma estrutura explícita usada por import/export/check, não uma
lista duplicada em cada script. Cada entrada contém `key`, `module`, `workflowId`, `exportPath` e
`writable`. O seletor `-Only` resolve exatamente uma entrada; nome/ID desconhecido encerra com erro.
Toda escrita recebe `-Owner` explícito e exige `selectedEntry.module == requestedOwner`. Em uma
tarefa omnichannel, `-Owner omnichannel` aceita somente `omnibrain0000001` e
`instafirst000001`; `writable` nunca substitui a validação do owner solicitado.

Antes de escrever/importar:

1. ler o JSON local e confirmar `id` canônico;
2. consultar runtime e confirmar que o ID resolvido é o mesmo;
3. confirmar `module` igual ao `-Owner` solicitado e `writable=true`;
4. gravar somente o arquivo daquela entrada;
5. comparar hashes dos arquivos não selecionados antes/depois;
6. falhar se algum hash mudar.

O modo `-Check` pode produzir relatório global, mas o exit code precisa distinguir:

- drift no alvo selecionado;
- drift fora do owner, somente informativo;
- ID/arquivo inconsistente, erro de integridade;
- runtime indisponível, erro operacional.

## 4. Pacotes atômicos

### E0-OWN-01 — registrar ownership documental

- **Escrita:** somente `AGENT.md`, `automation/AGENT.md`,
  `back/internal/modules/omnichannel/AGENT.md`, `docs/omnichannel/PLANO_TECNICO_EVOLUCAO.md`,
  `docs/omnichannel/ARQUITETURA_HIBRIDA_N8N.md`, `docs/omnichannel/ESTADO.md`,
  `docs/omnichannel/evolucao/E0_OWNERSHIP_WORKFLOWS.md`,
  `docs/omnichannel/evolucao/CONTRATO_EXECUCAO_AGENTES.md`,
  `docs/omnichannel/evolucao/CATALOGO_DESPACHO_AGENTES.md` e
  `docs/omnichannel/evolucao/MATRIZ_DEPENDENCIAS.md`.
- **Não pode:** código, JSON de workflow, compose, VPS.
- **Aceite:** os dois IDs próprios e todos os owners proibidos aparecem sem ambiguidade; WAHA é
  explicitamente preservado.

### E0-GUARD-02 — fechar scripts de import/export

- **Escrita:** somente scripts n8n compartilhados listados no pacote e testes deles.
- **Leitura:** somente os `workflow-*.json` canônicos registrados, e apenas para
  metadados/ID/hash quando o arquivo não for o alvo selecionado.
- **Não pode:** modificar qualquer JSON exportado.
- **Aceite:** tentativa de `-Only whatsapp` pelo contexto omnichannel falha antes de escrever;
  check direcionado aos dois workflows próprios funciona; hashes não próprios permanecem iguais.

### E0-QA-03 — auditoria independente

- **Escrita:** nenhuma, salvo relatório de evidência solicitado.
- **Aceite:** parse dos `workflow-*.json` canônicos registrados; unicidade de IDs; owner conhecido;
  diff/hash antes/depois; `git diff -- automation/export/workflow-whatsapp.json` vazio.

O QA deve excluir `credentials*.json`, `.mcp.json`, arquivos `.bak-*` e qualquer artefato não
canônico. Backups de `workflow-whatsapp.json` pertencem à Automação e não podem ser lidos,
alterados, renomeados ou removidos por tarefas omnichannel.

### E0-INT-04 — remover callers globais e fechar a cobertura

- **Escrita:** `scripts/dev/n8n-*.ps1`, `scripts/dev/n8n-*.js`, testes desses scripts,
  `package.json`, `.husky/pre-commit`, `scripts/deploy/deploy-fast.ps1` e estes documentos E0.
- **Não pode:** `scripts/deploy/deploy-pull.ps1`, compose, VPS, runtime n8n, credenciais ou qualquer
  `automation/export/*.json`.
- **Implementação:** import, export, sync e check de runtime exigem `-Owner` + `-Only`; o único
  check sem owner valida apenas registry/IDs/hashes locais e não chama Docker. Aliases npm passam
  owner/key exatos. O pre-commit não exporta conteúdo global. O deploy rápido não executa sync
  global: recebe owner/key explícitos ou falha antes de Docker.
- **Compatibilidade:** remover o modo global implícito. Operação de plataforma sobre vários owners
  não é simulada por uma tarefa omnichannel e exige pacote/autorização próprios.
- **Cobertura:** testar o normalizador sem runtime para credential fora de `{id,name}`, ID
  divergente, limpeza de `pinData/staticData` e nodes/URLs diretos de WAHA, Evolution,
  WhatsApp/Meta/Instagram. Mensagens de erro do CLI são sanitizadas, sem eco bruto.
- **Aceite:** não existe caller versionado de import/export/sync sem owner/key; `-Check` global não
  cria temporário nem chama Docker; os cinco hashes não próprios continuam iguais à baseline.

### E0-DOC-05 — alinhar instruções operacionais

- **Escrita:** somente `automation/AGENT.md`, `docs/DEPLOY_VPS.md` e os documentos E0.
- **Não pode:** scripts, workflows, compose, credenciais, runtime ou VPS.
- **Aceite:** nenhuma instrução canônica recomenda import/export/sync global; o check global é
  descrito como inventário local; `deploy:fast:prod` é API/web por padrão; o caminho
  `-DeployAutomation`/`deploy-pull` é identificado como operação global de plataforma e proibido
  para tarefas isoladas do Omnichannel até existir deploy owner-scoped de ponta a ponta.

## 5. Testes obrigatórios

- unidade: resolver key válida, inválida, ID divergente, arquivo ausente e owner proibido;
- integração local: check de `omnichannel-brain` e `instagram-first-contact` isoladamente;
- regressão: snapshot/hash dos workflows não próprios;
- erro: runtime indisponível não aciona sync nem trunca arquivo;
- segurança: saída de script não imprime credenciais do n8n.

## 6. Rollback e stop conditions

Rollback é reverter apenas os guardas/documentos desta fase. Nunca restaurar um arquivo n8n de
outro módulo por cópia automática. Pare se runtime e export local tiverem IDs diferentes, se um
workflow aparecer com dois owners ou se o script não conseguir limitar escrita a um único alvo.

## 7. Evidência de fechamento

- `E0-OWN-01`, `E0-GUARD-02`, `E0-QA-03`, `E0-INT-04` e `E0-DOC-05` revisados;
- `89/89` testes offline de ownership, normalização, callers e falhas precoces passaram;
- parser PowerShell 5.1, `node --check`, JSON do `package.json`, `bash -n` e
  `git diff --check` passaram;
- check global provado local-only, sem Node, Docker, runtime ou temporário;
- import/export/sync/check de runtime exigem owner + key exatos;
- hashes de Calendar, Operação e WhatsApp permaneceram iguais à baseline; WAHA e seus backups
  não foram lidos nem alterados;
- nenhum import, export, sync, restart, ativação, deploy ou alteração de VPS foi executado.

Limite conhecido e intencional: `deploy-pull.ps1 -DeployAutomation` continua sendo operação global
de plataforma. Ele não faz parte do deploy owner-scoped e permanece proibido para tarefas isoladas
do Omnichannel até pacote futuro implementar escopo de ponta a ponta.

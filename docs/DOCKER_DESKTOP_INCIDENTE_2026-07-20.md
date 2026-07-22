# Incidente local do Docker Desktop — 2026-07-20

**Estado:** núcleo local reconstruído; canal real e aceite externo E1 ainda pendentes  
**Escopo:** ambiente local de desenvolvimento; produção continua ativa  
**Regra:** qualquer snapshot, cópia, parada ou restore exige autorização explícita

## 1. Resumo factual

Durante o rebuild local da API, o BuildKit falhou com `EOF` e depois travou. Os logs do Docker
Desktop registraram `ENOSPC` às 21:36, seguido de falha/pânico e encerramento do backend às 21:45.
Um helper interno foi encerrado durante a tentativa de recuperação; essa ação foi insegura e não
deve se repetir. No start das 21:51, o Docker informou que `docker_data.vhdx` não existia e criou
um disco novo vazio.

Não houve comando `docker system prune`, `docker compose down -v` ou remoção explícita de volumes
pela CLI. Ainda assim, o resultado observado foi zero containers, imagens e volumes no datastore
local novo.

## 2. Stop conditions

- não encerrar `com.docker.*`, `Docker Desktop`, `dockerd`, `vmmem*` ou helpers internos;
- não rodar Compose contra o datastore vazio antes do plano de restore;
- não criar volumes com os nomes antigos antes de preservar as fontes de recuperação;
- não acessar, alterar ou restaurar produção sem autorização específica;
- não importar/exportar workflows protegidos individualmente; restaurar o volume n8n completo;
- não fazer deploy, commit, push ou iniciar E2 durante a recuperação.

## 3. Fontes locais preservadas

- worktree completo, migrations `0213` e `0214` e implementação E1;
- `.env` local e arquivos de configuração no host, fora do VHDX;
- sete exports versionados de workflows em `automation/export/`;
- export local de credenciais n8n ignorado pelo Git; conteúdo não foi exibido nem versionado;
- cópia de 13 arquivos de mídia do Omnichannel ainda presente na Lixeira;
- código-fonte e documentação dos demais módulos intactos.

O VHDX antigo não foi localizado no perfil, Lixeira ou em outra unidade. Não há shadow copy
acessível ao usuário atual; a leitura do journal NTFS requer terminal administrativo.

## 4. Inventário read-only da VPS em 2026-07-21

### Produção principal

- `api`, `web`, PostgreSQL, n8n, WAHA, Redis e Whisper ativos/saudáveis;
- imagem API/Web: `local-20260720-155921`;
- banco na migration `0212_messaging_contact_crm_backfill.sql`;
- banco possui 23 tabelas em `messaging`, mas zero contatos, zero conversas e zero mensagens do
  Omnichannel; há duas linhas de outbox;
- colunas E1 e `ai_generation` ainda não existem em produção;
- compose remoto difere do worktree local, que já contém a migration `0214`.

### Volumes úteis

| Volume | Tamanho observado | Estado |
|---|---:|---|
| `listaatendimento_postgres_data` | 1,957 GB | ativo |
| `listaatendimento_automation_n8n_data` | 332,9 MB | ativo; SQLite + WAL |
| `listaatendimento_automation_waha_sessions` | 116,5 MB | ativo; quatro arquivos |
| `listaatendimento_automation_waha_media` | 0 B | ativo/vazio |
| `listaatendimento_api_uploads` | 384,3 MB | ativo |
| `listaatendimento_automation_redis` | 88 B | descartável/recriável |
| `listaatendimento_automation_whisper_cache` | 2,243 GB | cache recriável |
| `omnichannel-mvp_evolution_instances` | 0 B | stack antiga/parada |
| `omnichannel-mvp_postgres_data` | 293,5 MB | stack antiga/parada; não inspecionado |

O n8n da VPS possui cinco workflows e todos estão ativos: WhatsApp, Omni Chat, Calendar Chat,
Calendar Omni e Calendar Transcribe. Os dois workflows novos do Omnichannel ainda não foram
implantados. Não foi feito export, import, ativação ou alteração de workflow durante o inventário.

### Backups PostgreSQL verificados

Ambos passaram em `gzip -t`:

| Arquivo | Tamanho | SHA-256 |
|---|---:|---|
| `backups/backup_20260720_190555.sql.gz` | 186.152.096 B | `ac1997ae8381b83477d91259b230086a84a959568aa0cde6b97452b0d72cf0de` |
| `backups/daily/backup_20260720_064001.sql.gz` | 186.119.281 B | `560489b69670b8f8e2ff2be4408a9ef42bb62fa2e5d3ca54b4dcf4e93a8196dc` |

Não foram encontrados arquivos de backup separados para os volumes n8n, WAHA ou Evolution.

## 5. Recuperação — R1 e R2 concluídas; R3 executada até o gate seguro

### Ponto de restauração criado em 2026-07-21

Com autorização explícita, foram executados somente o snapshot consistente na VPS e a cópia
para o host local. Não houve restore local, alteração de workflow, deploy, commit ou push.

- diretório remoto preservado:
  `/home/deploy/lista-atendimento/backups/volume-recovery-20260721T024500Z`;
- diretório local privado, fora do Git:
  `C:\Users\Mike\OmniRecovery\20260721T024500Z`;
- n8n pausado de `2026-07-21T02:55:12Z` a `02:55:25Z` (13 segundos);
- WAHA pausado de `2026-07-21T02:55:56Z` a `02:56:04Z` (8 segundos);
- os arquivos permanecerão também na VPS até o aceite completo da restauração local.

As duas primeiras tentativas de snapshot foram abortadas por permissões insuficientes no helper.
As proteções armadas retomaram os containers, e nenhum arquivo parcial permaneceu. A execução
final usou os volumes somente para leitura, helper sem rede e gravação atômica no host.

| Artefato local verificado | Tamanho | SHA-256 |
|---|---:|---|
| `backup_20260720_190555.sql.gz` | 186.152.096 B | `ac1997ae8381b83477d91259b230086a84a959568aa0cde6b97452b0d72cf0de` |
| `n8n-data.tar.gz` | 116.470.996 B | `2f0079e53ef6ff9adbba80f1b452259960dc627065d9f061c349b70d71356c85` |
| `waha-sessions.tar.gz` | 17.393.124 B | `9d873fbeef4295b3728cc2fc8eb3cf50fb49f8e9fbc73e89ac8eccc9096ce4b4` |
| `waha-media.tar.gz` | 86 B | `4046eef2cb561aeb5d59845f81eef0b44caefaa86a987581f994d2d49f10628b` |

Os quatro hashes locais coincidem com as fontes remotas. `gzip -t` validou o SQL e `tar -tzf`
validou os três snapshots (`n8n`: 2.755 entradas; sessões WAHA: 7; mídia WAHA: 1), sem expor
credenciais nem conteúdo dos arquivos.

Após as cópias, API e Web responderam HTTP `200`; PostgreSQL, API, Web, n8n, WAHA e Redis estavam
saudáveis. Os mesmos cinco workflows continuavam ativos pelos IDs:
`lzhb5JjN5kdcVuRR`, `omnichatmvp00001`, `calendarchat0001`, `calendaromni0001` e
`calendartrans001`.

### R1 — snapshot consistente na VPS — concluída

1. criar diretório datado de recuperação fora do repo;
2. pausar brevemente n8n e WAHA, porque SQLite/WAL e sessão podem mudar durante a cópia;
3. arquivar os volumes completos como leitura, sem exportar workflows individualmente;
4. incluir `api_uploads` se o objetivo for reproduzir produção localmente;
5. retomar os containers imediatamente;
6. gerar SHA-256 e conferir health público;
7. nunca tocar Calendar, Automation, Operação ou seus workflows isoladamente.

### R2 — transferência para o host local — concluída

1. copiar o SQL de 19:07 e os arquivos de volume via `scp`;
2. validar tamanho e SHA-256 após a transferência;
3. manter os arquivos fora do Git e com acesso restrito;
4. não apagar os snapshots da VPS até o aceite local completo.

### R3 — reconstrução local — núcleo concluído

1. confirmar novamente que o Docker local está no datastore vazio esperado;
2. recriar apenas a infraestrutura necessária, sem `down -v` ou prune;
3. restaurar o SQL em banco novo e validar tabelas/contagens antes de apontar a API;
4. deixar as migrations locais aplicar `0213` e `0214` sobre a base restaurada;
5. restaurar os volumes n8n e WAHA completos, preservando IDs, estados e credenciais;
6. restaurar as 13 mídias preservadas no volume privado do Omnichannel;
7. reconstruir imagens locais a partir do worktree atual;
8. re-parear a Evolution local: a VPS não possui snapshot útil da sessão Evolution do piloto;
9. validar PostgreSQL, n8n, WAHA, API/Web e somente então retomar o aceite E1.

### Evidência da execução R3 em 2026-07-21

- o datastore novo foi confirmado vazio antes da primeira criação, com 113,2 GB livres;
- os volumes `omni_automation_n8n_data`, `omni_automation_waha_sessions` e
  `omni_automation_waha_media` foram preenchidos antes de iniciar consumidores;
- o n8n `2.23.2` subiu saudável e manteve exatamente os cinco workflows da VPS, com os mesmos
  IDs e todos ativos; nenhum workflow foi importado, exportado ou editado;
- a WAHA permanece desligada localmente: o volume de sessões está restaurado e íntegro, mas
  iniciar uma segunda sessão enquanto a VPS está ativa poderia duplicar consumo/respostas;
- o dump foi restaurado primeiro em `omni_restore_local`, validado e promovido a `omni`; o banco
  vazio inicial foi preservado como `omni_empty_incident_20260721`, sem aceitar conexões;
- o dump das 19:05 é válido, porém vai somente até a migration `0199`, anterior ao deploy do
  Omnichannel na VPS. As três seeds de desenvolvimento que produção ignora foram conciliadas
  somente no ledger local para não sobrescrever usuários, senhas ou dados restaurados;
- a imagem local da API aplicou `0200` a `0214`; o schema `messaging` ficou com 23 tabelas,
  `ai_generation` presente e zero contatos/conversas/mensagens antes do cenário demo;
- API e Web ficaram saudáveis; `/healthz`, `/` e `/omnichannel` responderam HTTP `200`;
- os 13 JPEGs preservados (1.534.407 bytes) foram copiados para
  `omni_api_omnichannel_media`; a fonte continua na Lixeira. Como o dump antecede `messaging`,
  esses arquivos estão preservados, mas não possuem linhas de mensagem recuperáveis no banco;
- foi criado somente um cenário local `mock`, sem rede externa: uma instância conectada, três
  contatos, três conversas e uma resposta citada. A resposta percorreu outbox/provider mock,
  terminou `SENT` e manteve `reply_to_message_id` no PostgreSQL;
- a Evolution `v2.3.7` e seu PostgreSQL dedicado foram recriados em volumes novos e estão
  saudáveis; nenhuma instância/número foi pareada, pois esse gate exige QR controlado e a
  confirmação de que não haverá uma segunda sessão do mesmo número;
- a `WAHA` real não foi iniciada, nenhum contato real recebeu mensagem e E2 não foi iniciada.

R3 não reproduz integralmente os volumes compartilhados `api_uploads`/ERP da VPS e não recupera a
sessão Evolution perdida no VHDX antigo. Esses itens não impedem a tela e o smoke interno do E1,
mas continuam explicitamente fora do aceite de recuperação integral dos demais arquivos locais.

## 6. Critérios de recuperação

- os cinco workflows existentes continuam com os mesmos IDs e estado ativo;
- n8n e WAHA sobem usando cópias integrais de seus volumes;
- banco restaurado preserva dados dos outros módulos e chega à migration `0214` localmente;
- nenhum workflow fora do ownership Omnichannel recebe diff/import/export;
- uploads e mídias recuperados têm contagem/tamanho/hash conferidos;
- Evolution é tratada como novo pareamento controlado, nunca como sessão recuperada sem evidência;
- só depois destes gates o E1 volta a ser executado.

# Handoff — gravação e transcrições de atendimentos

Data do handoff: 2026-07-24
Estado: blocos de gravação e Whisper local implementados; validação manual com áudio real pendente.

## Objetivo do produto

Quando um atendimento da fila começa, o navegador inicia uma gravação de áudio.
O áudio é produzido em partes para reduzir a perda em interrupções, enviado ao
servidor durante a captura e relacionado ao atendimento, loja, conta e consultor.

Depois do encerramento, o áudio fica disponível na aba **Transcrições**. O próximo
bloco deve enviá-lo ao Whisper, persistir o texto e mostrá-lo nessa mesma aba.
Mais adiante, a intenção é transcrever partes durante a gravação, aproximando a
experiência de tempo real.

### Limite atual importante

A captura atual usa:

```ts
navigator.mediaDevices.getUserMedia({ audio: true });
```

Portanto, ela grava **o microfone selecionado pelo navegador**. Captura de áudio
interno do computador, chamada, aba ou desktop ainda não foi implementada.

## Feature flag experimental

- Chave: `attendanceAudioRecording`.
- Fonte autoritativa: JSON `experimental_features` em
  `core.platform_settings`.
- Valor inicial/fallback: `false` (fail-closed).
- Somente `platform_admin` pode alterar a flag.
- Painel: `/manage/experimental-features`.
- O toggle controla a criação/listagem da experiência. Uma gravação já iniciada
  pode terminar e consolidar suas partes mesmo se o toggle for desligado durante
  o atendimento.

Arquivos principais:

- `back/internal/modules/core/platform_features_model.go`
- `back/internal/modules/core/platform_features_service.go`
- `back/internal/modules/core/platform_features_http.go`
- `back/internal/modules/core/platform_features_test.go`
- `web/app/stores/platformFeatures.ts`
- `web/app/stores/platformFeatures.test.ts`
- `web/app/components/admin/ExperimentalFeaturesWorkspace.vue`
- `web/app/pages/manage/experimental-features.vue`

## Fluxo implementado

### 1. Início

Depois que o backend confirma o início do atendimento,
`OperationQueueColumns.vue` chama `startForService`.

O navegador:

1. solicita acesso ao microfone;
2. escolhe um MIME suportado pelo `MediaRecorder`;
3. cria uma sessão local;
4. cria/recupera a gravação autoritativa na API;
5. inicia o `MediaRecorder` com checkpoint de 5 segundos.

Existe apenas uma gravação ativa por navegador.

### 2. Cada parte

Para cada evento `dataavailable`:

1. a parte é salva no IndexedDB;
2. a mesma parte é enviada à API com número sequencial;
3. o backend valida conta, loja, atendimento, MIME e tamanho;
4. o arquivo é persistido em storage privado;
5. metadados e SHA-256 são persistidos no PostgreSQL.

O upload é idempotente por:

```text
account_id + recording_id + sequence
```

Repetir a mesma sequência com o mesmo SHA-256 é aceito. Bytes diferentes na
mesma sequência retornam conflito.

### 3. Parada

Ao parar:

1. o `MediaRecorder` entrega a última parte;
2. a fila local de escritas/uploads é aguardada;
3. a sessão local é encerrada;
4. o player local é montado separadamente;
5. as partes preservadas no IndexedDB são reenviadas idempotentemente;
6. a API consolida as partes recebidas e marca o áudio como `ready`.

Se o player local falhar, isso não invalida o áudio enviado ao servidor.

### 4. Consulta

A página `/transcricoes`:

- lista registros paginados e escopados por conta/loja;
- mostra atendimento, consultor, loja, horários, partes, tamanho e desfecho;
- diferencia gravação pronta de transcrição concluída;
- busca o áudio por endpoint autenticado e cria uma URL Blob temporária;
- permite solicitar a transcrição e acompanha `pending/processing/completed/failed`.

## Correção do erro de consolidação local

Mensagem observada:

> A gravacao parou, mas nao foi possivel consolidar as partes salvas no navegador.

O fluxo anterior tratava como uma única operação:

- atualizar a sessão local;
- reler todas as partes do IndexedDB;
- construir o Blob;
- montar o player.

Qualquer falha nessa sequência produzia a mesma mensagem genérica. Ela não
provava que as partes tinham sido perdidas.

Correção aplicada:

- persistência local, upload remoto, consolidação remota e player local possuem
  estados/erros separados;
- cada parte é enviada durante a gravação;
- o encerramento tenta reenviar o que existe no IndexedDB;
- o servidor consolida as partes independentemente do player;
- a UI oferece **Tentar enviar** quando o servidor ainda não confirmou;
- uma falha ao montar o player orienta o usuário a consultar **Transcrições**.

## Persistência e schema

Migration:

```text
back/internal/platform/database/migrations/0256_queue_attendance_transcriptions.sql
```

### `queue.attendance_recordings`

Guarda:

- `account_id`, `store_id`, `service_id`;
- consultor e nome capturado no início;
- `client_session_id` idempotente;
- estados da gravação e transcrição;
- MIME, início, fim, número de partes e tamanho;
- chave privada e SHA-256 do áudio consolidado;
- texto/erro futuro da transcrição;
- criador e timestamps;
- `retention_expires_at`, reservado para política futura.

Estados de gravação:

```text
recording | ready | interrupted | failed
```

Estados de transcrição:

```text
pending | processing | completed | failed
```

### `queue.attendance_recording_chunks`

Manifesto das partes com:

- conta e gravação;
- sequência;
- storage key;
- MIME;
- tamanho;
- SHA-256.

Não existe FK de `queue.*` para `core.*`. O isolamento é repetido por
`account_id`, conforme o contrato do módulo.

### Estado local confirmado em 2026-07-24

- banco alvo confirmado antes da migration: database `omni`, usuário `omni`;
- migrations anteriores chegavam até `0255`;
- `0256_queue_attendance_transcriptions.sql` foi aplicada;
- as duas tabelas existem;
- o role `omni_app` recebeu privilégios de leitura/escrita;
- nenhuma gravação real havia sido criada na hora da verificação.

## Storage privado

Variável:

```text
ATTENDANCE_AUDIO_DIR
```

Defaults:

```text
backend local: data/media/attendance-audio
container: /app/data/media/attendance-audio
```

Volume Compose:

```text
api_attendance_audio
```

Layout aproximado:

```text
<account>/<recording>/chunks/<sequence>-<sha256>.part
<account>/<recording>/audio.<ext>
```

Os nomes das partes incluem o SHA-256. Isso evita que uploads concorrentes e
conflitantes da mesma sequência sobrescrevam ou removam o arquivo válido.

Diretórios usam permissão `0750`; arquivos, `0600`. A chave física nunca é
devolvida ao frontend. O áudio é servido somente por endpoint autenticado.

Limites atuais:

- parte: 2 MiB;
- gravação: 1 GiB;
- sequência máxima: 100.000;
- MIME permitido: WebM, MP4/M4A, Ogg, MPEG/MP3 e WAV.

## API implementada

Todas as rotas exigem autenticação com conta ativa:

```text
POST /v1/operations/transcriptions
PUT  /v1/operations/transcriptions/{recordingId}/chunks/{sequence}
POST /v1/operations/transcriptions/{recordingId}/complete
GET  /v1/operations/transcriptions
GET  /v1/operations/transcriptions/{recordingId}/audio
```

Regras relevantes:

- `account_id` vem exclusivamente de `Principal.AccountID`;
- a loja é validada no service e repetida nas queries;
- o atendimento precisa existir nos ativos ou no histórico da mesma conta/loja;
- dados de consultor e início são resolvidos no PostgreSQL, não confiados ao
  payload do navegador;
- acesso fora do escopo retorna 404;
- o binário não fica no PostgreSQL nem em `/uploads`;
- a listagem é paginada;
- o endpoint de áudio usa resposta privada e `nosniff`.

Pacote:

```text
back/internal/modules/queue/transcriptions/
```

Arquivos:

- `AGENT.md`
- `errors.go`
- `http.go`
- `model.go`
- `service.go`
- `store_postgres.go`
- `storage_disk.go`
- `service_test.go`
- `storage_disk_test.go`

Wiring:

```text
back/internal/platform/app/app.go
```

## Frontend implementado

Captura e proteção local:

- `web/app/domain/operation/attendance-audio.ts`
- `web/app/domain/operation/attendance-audio.test.ts`
- `web/app/utils/attendance-audio-storage.ts`
- `web/app/stores/attendanceAudioRecording.ts`
- `web/app/components/operation/OperationAudioRecordingStatus.vue`
- `web/app/components/operation/OperationQueueColumns.vue`

Listagem e reprodução:

- `web/app/domain/operation/attendance-transcriptions.ts`
- `web/app/domain/operation/attendance-transcriptions.test.ts`
- `web/app/stores/attendanceTranscriptions.ts`
- `web/app/components/operation/AttendanceTranscriptionsWorkspace.vue`
- `web/app/pages/transcricoes.vue`

Navegação e gate:

- `web/layers/queue/nav.config.ts`
- `web/app/middleware/module-enabled.global.ts`
- `web/app/middleware/module-enabled.test.ts`

IndexedDB:

```text
omni-attendance-audio-v1
```

Uma sessão que permanecer como `recording` após recarregar a página é recuperada
como `interrupted`, preservando as partes já gravadas para retry.

## Configuração alterada

A variável e o volume foram adicionados a:

- `.env.docker.example`
- `.env.production.example`
- `.env.staging.example`
- `back/.env.example`
- `docker-compose.yml`
- `docker-compose.prod.yml`

Não houve commit, push ou deploy.

## Validações executadas

Backend:

```text
go test ./internal/modules/queue/transcriptions ./internal/platform/app
golangci-lint run ./internal/modules/queue/transcriptions/...
```

Resultado:

- testes passaram;
- lint do pacote passou com zero issues;
- existem testes de escopo/feature flag, idempotência e storage/consolidação.

Frontend:

```text
vitest:
  attendance-audio.test.ts
  attendance-transcriptions.test.ts
  module-enabled.test.ts
```

Resultado:

- 3 arquivos passaram;
- 11 testes passaram;
- ESLint dos arquivos tocados: zero erros;
- warnings conhecidos: arquivos de teste ignorados pelo ESLint e
  `nav.config.ts` com 504 linhas para limite recomendado de 500.

O `vue-tsc --noEmit` global continua falhando por dívida anterior e mudanças
paralelas do repositório. A saída possui muitos erros fora desta feature; nenhum
erro dos novos arquivos de gravação/transcrição foi identificado nessa rodada.

Docker/HTTP:

- migration e grants confirmados;
- API reconstruída e saudável;
- web reconstruído e iniciado;
- `GET /healthz` retornou 200;
- `/transcricoes` sem sessão retornou redirect para autenticação;
- rota da API sem contexto autenticado foi encontrada e rejeitada pelo gate.

Durante o rebuild ocorreu um `EOF`/erro interno do Docker Desktop. O runtime foi
recuperado e a stack voltou saudável. Se isso se repetir, seguir estritamente
`AGENT.md`: interromper somente o cliente iniciado, preservar evidências e
volumes, e não encerrar processos internos do Docker Desktop.

## Teste manual pendente

Este é o primeiro passo do próximo chat:

1. entrar como admin e confirmar que a feature está ativa;
2. abrir a operação de uma loja;
3. iniciar um atendimento;
4. permitir o microfone;
5. falar por pelo menos 10–15 segundos;
6. confirmar que o indicador mostra partes locais/enviadas;
7. encerrar ou parar a gravação;
8. confirmar o estado **SALVO**;
9. abrir `/transcricoes`;
10. localizar o atendimento e clicar em **Ouvir áudio**;
11. recarregar a página e repetir a reprodução;
12. opcionalmente interromper rede/recarregar durante outra gravação e testar
    **Tentar enviar** depois.

Durante o teste, verificar no banco:

```sql
select
    id,
    account_id,
    store_id,
    service_id,
    recording_status,
    transcription_status,
    chunk_count,
    size_bytes,
    audio_storage_key
from queue.attendance_recordings
order by created_at desc
limit 10;
```

E no volume:

```powershell
docker compose exec -T api sh -lc 'find "$ATTENDANCE_AUDIO_DIR" -type f -maxdepth 4 -ls'
```

Não imprimir nem expor conteúdo de áudio em logs.

## Bloco 2 implementado — Whisper

Já existe no Compose um Whisper self-hosted:

```text
imagem: fedirz/faster-whisper-server:latest-cpu
endpoint interno: http://whisper:8000/v1/audio/transcriptions
endpoint host local: http://localhost:8010/v1/audio/transcriptions
```

Ele é OpenAI-compatible, aceita WebM/Opus e não exige chave local.

### Implementado

Adicionar uma ação manual **Transcrever** na aba, com processamento durável:

1. `POST /v1/operations/transcriptions/{id}/transcribe`;
2. validar feature, conta, loja, permissão e áudio `ready`;
3. criar/claimar um job persistido no PostgreSQL;
4. marcar `transcription_status=processing`;
5. worker ler o áudio pelo storage privado;
6. enviar multipart ao Whisper configurável;
7. persistir `transcript_text` e `completed`, ou erro sanitizado e `failed`;
8. permitir retry idempotente;
9. a UI atualizar por polling curto ou invalidação realtime;
10. mostrar texto, erro e botão de retry sem inventar resultado.

Não usar goroutine solta como única fila: reinício da API não pode perder o job.
Adicionar uma migration `0257`, sem renumerar a `0256`, para lease/tentativas ou
uma tabela dedicada de jobs.

Configuração sugerida, sem hardcode:

```text
ATTENDANCE_TRANSCRIPTION_BASE_URL=http://whisper:8000
ATTENDANCE_TRANSCRIPTION_MODEL=<modelo configurado>
ATTENDANCE_TRANSCRIPTION_LANGUAGE=pt
ATTENDANCE_TRANSCRIPTION_TIMEOUT=<duração>
```

Adicionar exemplos, Compose, preflight aplicável e testes sem imprimir áudio,
texto sensível ou credenciais.

### Depois: transcrição durante a gravação

Não mandar cada Blob de 5 segundos cegamente e concatenar textos. O bloco de
tempo real deve ter contrato próprio:

- segmentos numerados e idempotentes;
- janela mínima útil e pequena sobreposição para não cortar palavras;
- tabela/estado de transcrição parcial por sequência;
- ordenação e deduplicação de overlap;
- texto parcial claramente identificado na UI;
- reconciliação final usando o áudio consolidado;
- a transcrição final autoritativa substitui o rascunho parcial;
- retry e retomada após queda do navegador/API/Whisper;
- WebSocket apenas invalida/avisa; PostgreSQL continua sendo a fonte de verdade.

## Critérios de pronto do próximo bloco

- áudio real reproduzido depois de reload;
- job Whisper sobrevive a restart;
- isolamento entre contas e lojas testado;
- botão de retry idempotente;
- texto e erro persistidos no PostgreSQL;
- áudio continua privado;
- feature desligada falha fechada para novas ações;
- testes Go e frontend passam no escopo;
- migration, ERD e `AGENT.md` atualizados;
- validação manual executada sem Playwright, conforme solicitado pelo usuário.

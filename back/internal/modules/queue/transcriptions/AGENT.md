# AGENT — Transcricoes de atendimentos

## Escopo

Este pacote pertence ao modulo `queue` e cuida somente da captura persistente,
consulta e entrega privada do audio ligado a um atendimento.

## Contrato do bloco 1

- `POST /v1/operations/transcriptions` cria ou recupera uma gravacao pelo
  `clientSessionId`; o atendimento precisa existir nos ativos ou no historico
  da mesma account/loja.
- `PUT /v1/operations/transcriptions/{id}/chunks/{sequence}` recebe um bloco cru,
  limitado a 2 MiB. O par gravacao+sequencia e idempotente; bytes diferentes na
  mesma sequencia retornam conflito.
- `POST /v1/operations/transcriptions/{id}/complete` consolida no storage privado,
  marca o audio como `ready` e solicita automaticamente a transcricao.
- `POST /v1/operations/transcriptions/{id}/transcribe` cria ou recupera a
  solicitacao duravel para o Whisper local. Retry de falha reinicia as tentativas.
- `GET /v1/operations/transcriptions` lista metadados paginados.
- `GET /v1/operations/transcriptions/{id}/audio` serve o arquivo somente depois
  de validar autenticacao, membership, permissao e `account_id`.

## Fonte de verdade e storage

- PostgreSQL: `queue.attendance_recordings` e
  `queue.attendance_recording_chunks`.
- Binario: `ATTENDANCE_AUDIO_DIR`, fora de `/uploads`, com diretorios `0750` e
  arquivos `0600`.
- O caminho nunca entra no JSON; o frontend recebe apenas `hasAudio`.
- O escopo vem exclusivamente do Principal: `AccountID` validado a partir de
  `X-Account-Id` e preferido, com fallback legado para `TenantID`. O frontend
  resolve a account dona da loja visivel antes de enviar o header. O repository
  repete esse identificador em todas as queries.
- Nao ha FK `queue -> core`, conforme o contrato do modulo; `account_id` continua
  obrigatorio e e validado pela membership antes do service.

## Job do Whisper

- A migration `0257` adiciona solicitacao, lease, proxima tentativa e contador
  diretamente na gravacao; reinicio da API nao perde o trabalho.
- O worker reclama jobs com `FOR UPDATE SKIP LOCKED`, faz no maximo tres
  tentativas e recupera leases expirados.
- O audio e enviado por multipart para
  `ATTENDANCE_TRANSCRIPTION_BASE_URL/v1/audio/transcriptions`, com modelo,
  idioma e timeout configuraveis por env.
- `transcript_text` e `transcript_error` sao autoritativos no PostgreSQL. Audio,
  texto e dados pessoais nunca entram nos logs do worker.
- A coluna `retention_expires_at` prepara a politica futura, mas nenhuma poda
  automatica esta ativa enquanto a retencao nao for configurada pelo produto.

## Transcricao quase ao vivo

- Os blocos de transporte continuam com 5 segundos e permanecem a fonte
  autoritativa para a consolidacao final.
- A migration `0260` cria
  `queue.attendance_live_transcription_segments`. A cada cinco blocos contiguos,
  o repository agenda uma janela duravel de 25 segundos.
- Depois da primeira janela, o segmento reaproveita o ultimo bloco e descarta
  seus primeiros 2,5 segundos. Isso deixa 2,5 segundos de sobreposicao util sem
  fazer o custo crescer com a duracao total do atendimento.
- Fragmentos WebM posteriores nao carregam cabecalho proprio. O storage monta
  somente `primeiro bloco + janela limitada`, e o ffmpeg da imagem da API gera
  um WAV mono de 16 kHz valido antes de chamar o Whisper.
- Um worker separado processa as janelas para que uma passagem final longa nao
  bloqueie o quase ao vivo. O merge remove a repeticao exata da sobreposicao.
- `live_transcript_text` e uma previa mutavel. Ao encerrar, a passagem completa
  pelo audio consolidado grava `transcript_text`, que sempre vence na API e e a
  unica entrada da analise/resumo via n8n.

## Analise e resumo via n8n

- O audio nunca passa pelo n8n: o worker Go envia o arquivo privado diretamente
  ao Whisper local e persiste a transcricao autoritativa.
- Um segundo job duravel envia somente texto e contexto ao workflow Queue-owned
  `queue-attendance-analysis` (`queueattendan001`).
- Configuracao por conta vive em `queue.attendance_analysis_configs` e guarda
  somente `credential_id`. A chave vem do cofre global account-scoped
  `messaging.ai_credentials` por uma fachada server-side injetada no composition
  root; Queue nao consulta o schema `messaging` diretamente.
- No workspace agregado, o frontend aplica a mesma configuracao a cada account
  operacional visivel. A fachada permite que uma account cliente use a
  credencial da account agencia somente quando ambas estao ativas e pertencem a
  mesma `organization_id`; qualquer outro cruzamento falha fechado.
- `queue.attendance_analysis_secrets` e legado de rollback da primeira versao do
  MVP e nao pode ser lida pelo runtime.
- Sem configuracao persistida, OpenAI e a provedora principal do resumo. O drawer
  seleciona primeiro a credencial OpenAI global da conta; Gemini permanece como
  alternativa explicita.
- Estado, lease e retry da analise sao independentes. Falha no resumo nao perde
  o audio nem solicita nova transcricao.
- Salvar uma configuracao valida rearma automaticamente analises pendentes ou
  falhas sem resumo, inclusive as que esgotaram tentativas por chave ausente.
- O n8n e stateless e devolve `{summary, report}`; o Go valida e persiste ambos.
- O workflow monta `requestBody` sem `temperature` para modelos OpenAI `gpt-5*`,
  pois esses modelos rejeitam valores diferentes do default. Os demais modelos
  continuam recebendo a temperatura configurada.
